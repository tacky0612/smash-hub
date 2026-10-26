// Package application は対戦コンテキストのユースケースを定義する。
package application

import (
	"context"
	"errors"
	"sort"

	"github.com/tacky0612/smash-hub/backend/internal/modules/match/domain"
	sharedapp "github.com/tacky0612/smash-hub/backend/internal/shared/application"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// ResultsChanged は確定した対戦記録が変わったことを受け取る（rating コンテキストが実装し、bootstrap で接続する）。
// 対戦の確定と同じトランザクション内で呼ばれる。
type ResultsChanged interface {
	OnResultsChanged(ctx context.Context) error
}

// Signer はアカウント自身の鍵で署名する（account コンテキストが実装する）。
type Signer interface {
	SignAs(ctx context.Context, id shared.AccountID, msg []byte) (string, error)
}

// RecordPublisher は確定した対戦記録を連合のログに追記する（federation コンテキストが実装する）。
type RecordPublisher interface {
	PublishMatch(ctx context.Context, r shared.MatchRecord) error
}

// RecordReader は連合で共有された確定済みの対戦記録を返す（federation コンテキストが実装する）。
type RecordReader interface {
	RecordsFor(ctx context.Context, account shared.AccountID, limit int) ([]shared.MatchRecord, error)
}

// RemoteHosts は他インスタンスがホストする対戦の状態の取得と、結果報告の中継を行う（federation が実装する）。
type RemoteHosts interface {
	Live(ctx context.Context, host shared.InstanceID, id shared.MatchID) (domain.Match, error)
	Report(ctx context.Context, host shared.InstanceID, id shared.MatchID, by shared.AccountID, r domain.Report, sig string) (domain.Match, error)
}

// Service は対戦のユースケース。
type Service struct {
	Tx        sharedapp.TxManager
	Matches   domain.Repository
	Remote    domain.RemoteRefRepository
	Clock     shared.Clock
	Self      shared.InstanceID
	Changed   ResultsChanged
	Signer    Signer
	Publisher RecordPublisher
	Records   RecordReader
	Hosts     RemoteHosts
	// VerifyDID は did:key のアカウント ID で署名を検証する
	VerifyDID func(did string, msg []byte, sig string) bool
}

// Start は成立した対戦を作る（matchmaking コンテキストから呼ばれる）。自インスタンスがホストになる。
// p2Home はプレイヤー2が他インスタンスの利用者ならそのインスタンス（自インスタンスなら空）。
func (s *Service) Start(ctx context.Context, p1, p2 shared.AccountID, f1, f2 shared.FighterID, p2Home shared.InstanceID) (shared.MatchID, error) {
	m, err := domain.New(shared.MatchID(shared.NewID()), s.Self, p1, p2, f1, f2, p2Home, s.Clock.Now())
	if err != nil {
		return "", err
	}
	return m.ID, s.Matches.Create(ctx, m)
}

// JoinRemote は、自インスタンスの利用者が他インスタンスのホストする対戦に参加したことを記録する。
func (s *Service) JoinRemote(ctx context.Context, host shared.InstanceID, id shared.MatchID, account shared.AccountID) error {
	return s.Remote.Create(ctx, domain.RemoteRef{MatchID: id, Host: host, AccountID: account, Status: domain.StatusInProgress, CreatedAt: s.Clock.Now()})
}

// ActiveMatchID は id の進行中の対戦 ID を返す。なければ空文字。
func (s *Service) ActiveMatchID(ctx context.Context, id shared.AccountID) (shared.MatchID, error) {
	m, err := s.Active(ctx, id)
	if errors.Is(err, shared.ErrNotFound) {
		return "", nil
	}
	return m.ID, err
}

// Get は対戦を取得する。自インスタンスがホストでなければホストから取得する。期限を過ぎていれば期限切れにしてから返す。
func (s *Service) Get(ctx context.Context, id shared.MatchID) (domain.Match, error) {
	m, err := s.Matches.Get(ctx, id)
	if errors.Is(err, shared.ErrNotFound) {
		return s.getRemote(ctx, id)
	}
	if err != nil {
		return m, err
	}
	if m.Status == domain.StatusInProgress && !s.Clock.Now().Before(m.ExpiresAt) {
		if err := s.expire(ctx, id); err != nil {
			return m, err
		}
		return s.Matches.Get(ctx, id)
	}
	return m, nil
}

func (s *Service) getRemote(ctx context.Context, id shared.MatchID) (domain.Match, error) {
	if s.Remote == nil || s.Hosts == nil {
		return domain.Match{}, shared.ErrNotFound
	}
	ref, err := s.Remote.Get(ctx, id)
	if err != nil {
		return domain.Match{}, err
	}
	m, err := s.Hosts.Live(ctx, ref.Host, id)
	if err != nil {
		return m, err
	}
	if m.Status != ref.Status {
		if err := s.Remote.SetStatus(ctx, id, m.Status); err != nil {
			return m, err
		}
	}
	return m, nil
}

// Active は id の進行中の対戦を返す（自インスタンスがホストする対戦と、他インスタンスがホストする対戦の両方）。
func (s *Service) Active(ctx context.Context, id shared.AccountID) (domain.Match, error) {
	m, err := s.Matches.Active(ctx, id)
	if err == nil {
		if !s.Clock.Now().Before(m.ExpiresAt) {
			if err := s.expire(ctx, m.ID); err != nil {
				return m, err
			}
		} else {
			return m, nil
		}
	} else if !errors.Is(err, shared.ErrNotFound) {
		return m, err
	}
	if s.Remote == nil {
		return domain.Match{}, shared.ErrNotFound
	}
	refs, err := s.Remote.Active(ctx, id)
	if err != nil {
		return domain.Match{}, err
	}
	for _, ref := range refs {
		m, err := s.getRemote(ctx, ref.MatchID)
		if err != nil {
			return m, err
		}
		if m.Status == domain.StatusInProgress {
			return m, nil
		}
	}
	return domain.Match{}, shared.ErrNotFound
}

// SetRoomID は専用部屋の ID を設定する（部屋を立てるのはホスト側のプレイヤー1）。
func (s *Service) SetRoomID(ctx context.Context, by shared.AccountID, id shared.MatchID, roomID string) (domain.Match, error) {
	return s.update(ctx, id, func(m *domain.Match) (bool, error) {
		return false, m.SetRoomID(by, roomID, s.Clock.Now())
	})
}

// Report は自インスタンスの利用者の結果報告。勝敗の報告には、対戦結果の内容へのその利用者の署名を付ける。
// 他インスタンスがホストする対戦なら、ホストへ中継する。
func (s *Service) Report(ctx context.Context, by shared.AccountID, id shared.MatchID, r domain.Report) (domain.Match, error) {
	if _, err := s.Matches.Get(ctx, id); errors.Is(err, shared.ErrNotFound) {
		return s.reportRemote(ctx, by, id, r)
	}
	return s.update(ctx, id, func(m *domain.Match) (bool, error) {
		if by == m.Player2 && m.Player2Home != "" {
			return false, shared.ErrForbidden // 他インスタンスの利用者は、ホームインスタンス経由で報告する
		}
		sig, err := s.sign(ctx, *m, by, r)
		if err != nil {
			return false, err
		}
		return m.SubmitReport(by, r, sig, s.Clock.Now())
	})
}

func (s *Service) reportRemote(ctx context.Context, by shared.AccountID, id shared.MatchID, r domain.Report) (domain.Match, error) {
	m, err := s.getRemote(ctx, id)
	if err != nil {
		return m, err
	}
	if !m.IsParticipant(by) {
		return m, shared.ErrForbidden
	}
	sig, err := s.sign(ctx, m, by, r)
	if err != nil {
		return m, err
	}
	ref, err := s.Remote.Get(ctx, id)
	if err != nil {
		return m, err
	}
	m, err = s.Hosts.Report(ctx, ref.Host, id, by, r, sig)
	if err != nil {
		return m, err
	}
	return m, s.Remote.SetStatus(ctx, id, m.Status)
}

func (s *Service) sign(ctx context.Context, m domain.Match, by shared.AccountID, r domain.Report) (string, error) {
	winner := m.WinnerIndex(by, r)
	if winner < 0 || s.Signer == nil {
		return "", nil // 中止の報告には署名しない
	}
	return s.Signer.SignAs(ctx, by, m.Statement(winner).Bytes())
}

// RemoteReport はホストとして、他インスタンスから中継されたプレイヤー2の結果報告を受け付ける。
// 中継元がプレイヤー2のホームインスタンスであることと、プレイヤー2の署名を検証する。
func (s *Service) RemoteReport(ctx context.Context, from shared.InstanceID, id shared.MatchID, by shared.AccountID, r domain.Report, sig string) (domain.Match, error) {
	return s.update(ctx, id, func(m *domain.Match) (bool, error) {
		if by != m.Player2 || m.Player2Home == "" || m.Player2Home != from {
			return false, shared.ErrForbidden
		}
		if winner := m.WinnerIndex(by, r); winner >= 0 {
			if s.VerifyDID == nil || !s.VerifyDID(string(by), m.Statement(winner).Bytes(), sig) {
				return false, shared.Invalid("報告の署名が正しくありません")
			}
		}
		return m.SubmitReport(by, r, sig, s.Clock.Now())
	})
}

// LiveForPeer はホストとして、プレイヤー2のホームインスタンスに対戦の状態を返す。
func (s *Service) LiveForPeer(ctx context.Context, from shared.InstanceID, id shared.MatchID) (domain.Match, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return m, err
	}
	if m.Host != s.Self || m.Player2Home == "" || m.Player2Home != from {
		return domain.Match{}, shared.ErrForbidden
	}
	return m, nil
}

// History は id の最近の対戦を返す。自インスタンスがホストした対戦と、連合で共有された確定済みの記録を合わせる。
func (s *Service) History(ctx context.Context, id shared.AccountID, limit int) ([]domain.Match, error) {
	local, err := s.Matches.History(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	seen := map[shared.MatchID]bool{}
	for _, m := range local {
		seen[m.ID] = true
	}
	if s.Records != nil {
		recs, err := s.Records.RecordsFor(ctx, id, limit)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			if seen[r.MatchID] {
				continue
			}
			local = append(local, fromRecord(r))
		}
	}
	sort.Slice(local, func(i, j int) bool { return local[i].PlayedAt.After(local[j].PlayedAt) })
	if len(local) > limit {
		local = local[:limit]
	}
	return local, nil
}

func fromRecord(r shared.MatchRecord) domain.Match {
	m := domain.Match{
		ID: r.MatchID, Host: r.HostInstance, Status: domain.StatusConfirmed,
		Player1: r.Players[0].AccountID, Player2: r.Players[1].AccountID,
		Winner: r.Players[r.Winner].AccountID,
	}
	if len(r.Picks) > 0 && len(r.Picks[0].Characters) == 2 {
		m.Fighter1, m.Fighter2 = r.Picks[0].Characters[0], r.Picks[0].Characters[1]
	}
	m.PlayedAt, _ = parseTime(r.PlayedAt)
	return m
}

// ExpireDue は期限を過ぎた対戦をまとめて期限切れにする（定期実行）。
func (s *Service) ExpireDue(ctx context.Context) (int, error) {
	ids, err := s.Matches.DueForExpiry(ctx, s.Clock.Now(), 100)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.expire(ctx, id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

func (s *Service) expire(ctx context.Context, id shared.MatchID) error {
	_, err := s.update(ctx, id, func(m *domain.Match) (bool, error) {
		m.ExpireIfDue(s.Clock.Now())
		return false, nil
	})
	return err
}

// update は行ロックを取って対戦を変更・保存する。fn が true を返したら確定として扱い、
// 同じトランザクションで対戦記録を連合のログに追記し、レートを再計算する。
// 期限切れによる状態変更は、fn がエラーを返しても保存する。
func (s *Service) update(ctx context.Context, id shared.MatchID, fn func(m *domain.Match) (bool, error)) (domain.Match, error) {
	var result domain.Match
	var fnErr error
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		m, err := s.Matches.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		before := m.Status
		confirmed, err := fn(&m)
		if err != nil && m.Status == before {
			return err
		}
		fnErr = err
		if err := s.Matches.Save(ctx, m); err != nil {
			return err
		}
		if confirmed {
			if s.Publisher != nil {
				rec, err := m.Record()
				if err != nil {
					return err
				}
				if err := s.Publisher.PublishMatch(ctx, rec); err != nil {
					return err
				}
			}
			if s.Changed != nil {
				if err := s.Changed.OnResultsChanged(ctx); err != nil {
					return err
				}
			}
		}
		result = m
		return nil
	})
	if err != nil {
		return domain.Match{}, err
	}
	return result, fnErr
}
