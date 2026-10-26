// Package application は連合コンテキストのユースケースを定義する。
// 自インスタンスのイベントの発行、連合先のログの取り込み（pull）と検証、インスタンス間の対戦の受け口を扱う。
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/domain"
	sharedapp "github.com/tacky0612/smash-hub/backend/internal/shared/application"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// IngestCutoff は取り込みの締め切り。ホストがログに追記した日時と played_at の差がこれを超える記録はレート計算に含めない。
const IngestCutoff = 7 * 24 * time.Hour

// PeerState は連合先と、その連合先のログをどこまで取り込んだか。
type PeerState struct {
	URL       string
	PublicKey string // base64。初回に /federation/instance から取得して固定する
	LastSeq   int64
	LastHash  string
}

// RemoteAccount は他インスタンスのアカウントの公開情報。
type RemoteAccount struct {
	AccountID    shared.AccountID
	DisplayName  string
	HomeInstance string
}

// StoredRecord は保存済みの対戦記録。
type StoredRecord struct {
	shared.MatchRecord
	LoggedAt time.Time
}

// RemoteRequest は他インスタンスから配信された対戦募集。
type RemoteRequest struct {
	ID        string           `json:"id"`
	Origin    string           `json:"-"`
	AccountID shared.AccountID `json:"account_id"`
	Fighter   shared.FighterID `json:"fighter"`
	RateRange int              `json:"rate_range"`
	Rating    int              `json:"rating"`
	CreatedAt time.Time        `json:"created_at"`
	ExpiresAt time.Time        `json:"-"`
}

// Store は連合コンテキストの永続化。
type Store interface {
	// AppendOwn は自インスタンスのログに1件追記する。build には次の seq と直前のハッシュが渡される（直列化して呼ばれる）。
	AppendOwn(ctx context.Context, origin string, build func(seq int64, prev string) (domain.Event, error)) (domain.Event, error)
	Events(ctx context.Context, origin string, since int64, limit int) ([]json.RawMessage, error)
	Peer(ctx context.Context, url string) (PeerState, error)
	SavePeer(ctx context.Context, p PeerState, syncErr string, now time.Time) error
	StoreEvent(ctx context.Context, e domain.Event) error
	InsertRecord(ctx context.Context, r shared.MatchRecord, loggedAt time.Time) error
	Records(ctx context.Context, from, to time.Time) ([]StoredRecord, error)
	RecordsFor(ctx context.Context, account shared.AccountID, limit int) ([]StoredRecord, error)
	UpsertRemoteAccount(ctx context.Context, a RemoteAccount, now time.Time) error
	RemoteAccount(ctx context.Context, id shared.AccountID) (RemoteAccount, error)
	UpsertRemoteRequest(ctx context.Context, r RemoteRequest) error
	DeleteRemoteRequest(ctx context.Context, origin, id string) error
	RemoteRequests(ctx context.Context, now time.Time) ([]RemoteRequest, error)
}

// PeerClient は連合先との HTTP 通信（adapter が実装する）。
type PeerClient interface {
	InstanceInfo(ctx context.Context, peer string) (InstanceInfo, error)
	Log(ctx context.Context, peer string, since int64, limit int) ([]domain.Event, error)
}

// InstanceInfo はインスタンスの公開情報。
type InstanceInfo struct {
	InstanceID string `json:"instance_id"`
	Name       string `json:"name"`
	PublicKey  string `json:"public_key"`
}

// Service は連合のユースケース。
type Service struct {
	Tx        sharedapp.TxManager
	Store     Store
	Client    PeerClient
	Clock     shared.Clock
	Self      shared.InstanceID
	Name      string
	PublicKey string // base64
	Sign      domain.Signer
	VerifyKey func(pubB64 string, msg []byte, sig string) bool
	VerifyDID domain.VerifyDID
	Peers     []string
	// RecordsChanged は対戦記録が増えたときに呼ばれる（rating の再計算。同じトランザクション内）
	RecordsChanged func(ctx context.Context) error
}

// Info は自インスタンスの公開情報。
func (s *Service) Info() InstanceInfo {
	return InstanceInfo{InstanceID: string(s.Self), Name: s.Name, PublicKey: s.PublicKey}
}

// IsPeer は url が連合先（許可リスト）に含まれるかを返す。
func (s *Service) IsPeer(url string) bool {
	return slices.Contains(s.Peers, url)
}

func (s *Service) append(ctx context.Context, typ string, payload any) (domain.Event, error) {
	createdAt := shared.FormatTime(s.Clock.Now())
	return s.Store.AppendOwn(ctx, string(s.Self), func(seq int64, prev string) (domain.Event, error) {
		return domain.NewEvent(typ, s.Self, seq, prev, createdAt, payload, s.Sign)
	})
}

// PublishAccount はアカウントの公開情報をログに追記する（a はアカウント自身の鍵で署名済み）。
func (s *Service) PublishAccount(ctx context.Context, a domain.AccountPublished) error {
	a.HomeInstance = string(s.Self)
	_, err := s.append(ctx, domain.TypeAccountPublished, a)
	return err
}

// PublishMatch は確定した対戦記録をログに追記し、レート計算の入力に加える。
func (s *Service) PublishMatch(ctx context.Context, r shared.MatchRecord) error {
	e, err := s.append(ctx, domain.TypeMatchRecorded, r)
	if err != nil {
		return err
	}
	loggedAt, _ := time.Parse(time.RFC3339, e.CreatedAt)
	return s.Store.InsertRecord(ctx, r, loggedAt)
}

// OwnLog は自インスタンスのログを since の次から返す。
func (s *Service) OwnLog(ctx context.Context, since int64, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.Store.Events(ctx, string(s.Self), since, min(limit, 500))
}

// PeerPublicKey は連合先の公開鍵を返す。初回は連合先から取得して保存し、以後は保存した値を使う。
func (s *Service) PeerPublicKey(ctx context.Context, peer string) (string, error) {
	if !s.IsPeer(peer) {
		return "", shared.ErrForbidden
	}
	st, err := s.Store.Peer(ctx, peer)
	if err != nil {
		return "", err
	}
	if st.PublicKey != "" {
		return st.PublicKey, nil
	}
	info, err := s.Client.InstanceInfo(ctx, peer)
	if err != nil {
		return "", err
	}
	if info.InstanceID != peer || info.PublicKey == "" {
		return "", fmt.Errorf("peer %s returned instance_id %q", peer, info.InstanceID)
	}
	st.PublicKey = info.PublicKey
	return st.PublicKey, s.Store.SavePeer(ctx, st, "", s.Clock.Now())
}

// SyncAll はすべての連合先のログを取り込む。1つの連合先の失敗は他に影響させない。
func (s *Service) SyncAll(ctx context.Context) {
	for _, p := range s.Peers {
		if n, err := s.SyncPeer(ctx, p); err != nil {
			slog.Warn("federation sync failed", "peer", p, "err", err)
		} else if n > 0 {
			slog.Info("federation synced", "peer", p, "events", n)
		}
	}
}

// SyncPeer は連合先のログの新しいイベントを取得し、検証して取り込む。取り込んだ件数を返す。
// 検証に失敗したイベント以降は取り込まず、理由を記録する（docs/architecture/federation.md 6.3）。
func (s *Service) SyncPeer(ctx context.Context, peer string) (int, error) {
	pub, err := s.PeerPublicKey(ctx, peer)
	if err != nil {
		return 0, err
	}
	st, err := s.Store.Peer(ctx, peer)
	if err != nil {
		return 0, err
	}
	events, err := s.Client.Log(ctx, peer, st.LastSeq, 200)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}
	imported := 0
	var syncErr error
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		records := false
		for _, e := range events {
			if err := s.importEvent(ctx, peer, pub, &st, e); err != nil {
				syncErr = fmt.Errorf("seq %d: %w", e.Seq, err)
				break
			}
			imported++
			records = records || e.Type == domain.TypeMatchRecorded
		}
		msg := ""
		if syncErr != nil {
			msg = syncErr.Error()
		}
		if err := s.Store.SavePeer(ctx, st, msg, s.Clock.Now()); err != nil {
			return err
		}
		if records && s.RecordsChanged != nil {
			return s.RecordsChanged(ctx)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return imported, syncErr
}

func (s *Service) importEvent(ctx context.Context, peer, pub string, st *PeerState, e domain.Event) error {
	verifyInstance := func(msg []byte, sig string) bool { return s.VerifyKey(pub, msg, sig) }
	if err := domain.VerifyChain(e, peer, st.LastSeq, st.LastHash, verifyInstance); err != nil {
		return err
	}
	switch e.Type {
	case domain.TypeAccountPublished:
		a, err := domain.DecodeAccountPublished(e, s.VerifyDID)
		if err != nil {
			return err
		}
		if err := s.Store.UpsertRemoteAccount(ctx, RemoteAccount{AccountID: a.AccountID, DisplayName: a.DisplayName, HomeInstance: a.HomeInstance}, s.Clock.Now()); err != nil {
			return err
		}
	case domain.TypeMatchRecorded:
		r, err := domain.DecodeMatchRecorded(e, s.VerifyDID)
		if err != nil {
			return err
		}
		loggedAt, err := time.Parse(time.RFC3339, e.CreatedAt)
		if err != nil {
			return err
		}
		if err := s.Store.InsertRecord(ctx, r, loggedAt); err != nil {
			return err
		}
	default:
		// 未対応の種別は、ログとしては保存して先へ進む（将来のバージョンとの互換のため）
	}
	if err := s.Store.StoreEvent(ctx, e); err != nil {
		return err
	}
	st.LastSeq, st.LastHash = e.Seq, e.Hash()
	return nil
}

// ConfirmedResults はレート計算の入力となる記録を返す。取り込みの締め切りを過ぎた記録は除く。
func (s *Service) ConfirmedResults(ctx context.Context, from, to time.Time) ([]StoredRecord, error) {
	all, err := s.Store.Records(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, r := range all {
		played, err := time.Parse(time.RFC3339, r.PlayedAt)
		if err != nil {
			continue
		}
		if r.LoggedAt.Sub(played) <= IngestCutoff {
			out = append(out, r)
		}
	}
	return out, nil
}

// --- インスタンスをまたぐ対戦募集 ---

// RemoteRequestTTL は配信された募集の有効期間。配信元は待機中、これより短い間隔で配信し直す。
const RemoteRequestTTL = 30 * time.Second

// ReceiveRequest は連合先から配信された募集を保存する。
func (s *Service) ReceiveRequest(ctx context.Context, from string, r RemoteRequest) error {
	r.Origin, r.ExpiresAt = from, s.Clock.Now().Add(RemoteRequestTTL)
	if r.ID == "" || r.AccountID == "" || !shared.IsValidFighter(r.Fighter, true) {
		return shared.Invalid("募集の内容が正しくありません")
	}
	return s.Store.UpsertRemoteRequest(ctx, r)
}

// WithdrawRequest は配信された募集を取り消す。
func (s *Service) WithdrawRequest(ctx context.Context, from, id string) error {
	return s.Store.DeleteRemoteRequest(ctx, from, id)
}

// ActiveRemoteRequests は有効な配信済みの募集を返す。
func (s *Service) ActiveRemoteRequests(ctx context.Context) ([]RemoteRequest, error) {
	return s.Store.RemoteRequests(ctx, s.Clock.Now())
}

// ErrNotPeer は許可リストにない相手からのリクエスト。
var ErrNotPeer = errors.New("not a federation peer")
