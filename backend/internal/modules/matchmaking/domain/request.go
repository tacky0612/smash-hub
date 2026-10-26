// Package domain は対戦募集（マッチング）コンテキストのドメインモデルを定義する。
package domain

import (
	"context"
	"time"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Status は対戦募集の状態。
type Status string

// 対戦募集の状態。
const (
	StatusWaiting   Status = "waiting"
	StatusMatched   Status = "matched"
	StatusCancelled Status = "cancelled"
)

// マッチングのパラメータ（docs/domain/match-rules.md の「8.1 マッチング条件」）。
const (
	// UnlimitedInitialRange は「制限なし」の初期のレート差。
	UnlimitedInitialRange = 200
	// UnlimitedStepPerMinute は「制限なし」で1分待つごとに広げるレート差。
	UnlimitedStepPerMinute = 100
	// StaleAfter はこの時間状態の確認がない募集を、離脱したとみなす時間。
	StaleAfter = 30 * time.Second
)

// RateRanges は選べるレート差。0 は「制限なし」。
var RateRanges = []int{0, 50, 100, 150, 200}

// Request は「対戦相手を探す」で作られるマッチング待ちの募集。
type Request struct {
	ID         string
	AccountID  shared.AccountID
	Fighter    shared.FighterID // 1戦目のファイター
	RateRange  int              // 0 は「制限なし」
	Rating     int              // 募集時のレート
	Status     Status
	MatchID    shared.MatchID
	CreatedAt  time.Time
	LastSeenAt time.Time
	// Origin は他インスタンスから配信された募集の配信元（自インスタンスの募集なら空）
	Origin shared.InstanceID
}

// NewRequest は入力値を検証して募集を作る。
func NewRequest(account shared.AccountID, fighter shared.FighterID, rateRange, rating int, now time.Time) (Request, error) {
	if !shared.IsValidFighter(fighter, true) {
		return Request{}, shared.Invalid("1戦目のファイターを選んでください")
	}
	valid := false
	for _, r := range RateRanges {
		valid = valid || r == rateRange
	}
	if !valid {
		return Request{}, shared.Invalid("レート差の指定が正しくありません")
	}
	return Request{
		ID: shared.NewID(), AccountID: account, Fighter: fighter, RateRange: rateRange, Rating: rating,
		Status: StatusWaiting, CreatedAt: now, LastSeenAt: now,
	}, nil
}

// AllowedDiff は now の時点で許容するレート差を返す。
func (r Request) AllowedDiff(now time.Time) int {
	if r.RateRange > 0 {
		return r.RateRange
	}
	waited := int(now.Sub(r.CreatedAt) / time.Minute)
	return UnlimitedInitialRange + UnlimitedStepPerMinute*max(waited, 0)
}

// IsStale は募集者が離脱したとみなせるかを返す。
func (r Request) IsStale(now time.Time) bool {
	return now.Sub(r.LastSeenAt) > StaleAfter
}

// Compatible は2つの募集をマッチングしてよいかを返す。両者の許容するレート差のうち狭いほうを使う。
func Compatible(a, b Request, now time.Time) bool {
	if a.AccountID == b.AccountID || a.Status != StatusWaiting || b.Status != StatusWaiting {
		return false
	}
	diff := a.Rating - b.Rating
	if diff < 0 {
		diff = -diff
	}
	return diff <= min(a.AllowedDiff(now), b.AllowedDiff(now))
}

// Pair は先に待っていた側をプレイヤー1（部屋を立てる側）として返す。
func Pair(a, b Request) (p1, p2 Request) {
	if b.CreatedAt.Before(a.CreatedAt) {
		return b, a
	}
	return a, b
}

// Repository は対戦募集の永続化。
type Repository interface {
	Create(ctx context.Context, r Request) error
	Save(ctx context.Context, r Request) error
	// WaitingForUpdate は account の待機中の募集を行ロックを取って返す。なければ ErrNotFound。
	WaitingForUpdate(ctx context.Context, account shared.AccountID) (Request, error)
	// WaitingByIDForUpdate は待機中の募集を ID で行ロックを取って返す。他のトランザクションがロック中なら ErrNotFound。
	WaitingByIDForUpdate(ctx context.Context, id string) (Request, error)
	// CandidatesForUpdate は account 以外の、seenAfter 以降に確認のあった待機中の募集を古い順に返す。
	// 他のトランザクションがロック中の募集は飛ばす。
	CandidatesForUpdate(ctx context.Context, account shared.AccountID, seenAfter time.Time, limit int) ([]Request, error)
	// CancelStale は seenBefore より前から確認のない待機中の募集を取り消す。
	CancelStale(ctx context.Context, seenBefore time.Time) (int, error)
}
