// Package domain は連合コンテキストのドメインモデル（署名付き追記専用ログのイベント）を定義する。
// 仕様は docs/architecture/federation.md。
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// イベント種別。
const (
	TypeAccountPublished = "AccountPublished"
	TypeMatchRecorded    = "MatchRecorded"
)

// EventVersion はイベントのスキーマのバージョン。
const EventVersion = 1

// Event は追記専用ログの1件。
type Event struct {
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	InstanceID string          `json:"instance_id"`
	Seq        int64           `json:"seq"`
	Prev       string          `json:"prev"`
	CreatedAt  string          `json:"created_at"`
	Payload    json.RawMessage `json:"payload"`
	Sig        string          `json:"sig,omitempty"`
}

// Signer は署名を作る（鍵の扱いは platform/crypto が担う）。
type Signer func(msg []byte) string

// Verifier は公開鍵で署名を検証する。
type Verifier func(msg []byte, sig string) bool

// SigningBytes はインスタンス署名の対象（sig を除いたイベントの正規化 JSON）。
func (e Event) SigningBytes() []byte {
	e.Sig = ""
	b, err := shared.Canonical(e)
	if err != nil {
		panic(err)
	}
	return b
}

// Body は署名済みイベントの正規化 JSON。ログにはこのバイト列をそのまま保存・配信する。
func (e Event) Body() []byte {
	b, err := shared.Canonical(e)
	if err != nil {
		panic(err)
	}
	return b
}

// Hash はイベントのハッシュ。次のイベントの prev になる。
func (e Event) Hash() string {
	sum := sha256.Sum256(e.Body())
	return hex.EncodeToString(sum[:])
}

// NewEvent は payload を正規化して、署名済みのイベントを作る。
func NewEvent(typ string, instance shared.InstanceID, seq int64, prev, createdAt string, payload any, sign Signer) (Event, error) {
	p, err := shared.Canonical(payload)
	if err != nil {
		return Event{}, err
	}
	e := Event{Type: typ, Version: EventVersion, InstanceID: string(instance), Seq: seq, Prev: prev, CreatedAt: createdAt, Payload: p}
	e.Sig = sign(e.SigningBytes())
	return e, nil
}

// AccountPublished はアカウントの公開情報。アカウント自身の鍵でも署名する（自己証明型 → federation.md 4.3）。
type AccountPublished struct {
	AccountID    shared.AccountID `json:"account_id"`
	DisplayName  string           `json:"display_name"`
	HomeInstance string           `json:"home_instance"`
	AccountSig   string           `json:"account_sig,omitempty"`
}

// SigningBytes はアカウント署名の対象。
func (a AccountPublished) SigningBytes() []byte {
	a.AccountSig = ""
	b, err := shared.Canonical(a)
	if err != nil {
		panic(err)
	}
	return b
}

// 検証エラー。
var (
	ErrBadInstanceSignature = errors.New("instance signature is invalid")
	ErrBrokenChain          = errors.New("hash chain is broken")
	ErrUnexpectedOrigin     = errors.New("event origin does not match the peer")
)

// VerifyChain は peer から受け取ったイベントの署名・連番・ハッシュチェーンを検証する（federation.md 6.3 の 1〜3）。
func VerifyChain(e Event, peer string, lastSeq int64, lastHash string, verifyInstance Verifier) error {
	if e.InstanceID != peer {
		return ErrUnexpectedOrigin
	}
	if !verifyInstance(e.SigningBytes(), e.Sig) {
		return ErrBadInstanceSignature
	}
	if e.Seq != lastSeq+1 || e.Prev != lastHash {
		return fmt.Errorf("%w: seq %d (want %d)", ErrBrokenChain, e.Seq, lastSeq+1)
	}
	return nil
}

// VerifyDID は did:key のアカウント ID で署名を検証する関数。
type VerifyDID func(did string, msg []byte, sig string) bool

// DecodeAccountPublished は payload を検証して取り出す。
func DecodeAccountPublished(e Event, verify VerifyDID) (AccountPublished, error) {
	var a AccountPublished
	if err := json.Unmarshal(e.Payload, &a); err != nil {
		return a, err
	}
	if a.HomeInstance != e.InstanceID {
		return a, errors.New("AccountPublished: home_instance must be the issuing instance")
	}
	if !verify(string(a.AccountID), a.SigningBytes(), a.AccountSig) {
		return a, errors.New("AccountPublished: account signature is invalid")
	}
	return a, nil
}

// DecodeMatchRecorded は payload を検証して取り出す（federation.md 6.3 の 4・7）。
func DecodeMatchRecorded(e Event, verify VerifyDID) (shared.MatchRecord, error) {
	var r shared.MatchRecord
	if err := json.Unmarshal(e.Payload, &r); err != nil {
		return r, err
	}
	if string(r.HostInstance) != e.InstanceID {
		return r, errors.New("MatchRecorded: host_instance must be the issuing instance")
	}
	if r.Ruleset != shared.RulesetStandard {
		return r, fmt.Errorf("MatchRecorded: unsupported ruleset %q", r.Ruleset)
	}
	if len(r.Players) != 2 || r.Players[0].AccountID == r.Players[1].AccountID || (r.Winner != 0 && r.Winner != 1) {
		return r, errors.New("MatchRecorded: invalid players or winner")
	}
	if len(r.Signatures) != 2 {
		return r, errors.New("MatchRecorded: both players must sign")
	}
	msg := r.MatchStatement.Bytes()
	for i, p := range r.Players {
		s := r.Signatures[i]
		if s.AccountID != p.AccountID || !verify(string(p.AccountID), msg, s.Sig) {
			return r, fmt.Errorf("MatchRecorded: signature of player %d is invalid", i+1)
		}
	}
	return r, nil
}
