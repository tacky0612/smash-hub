// Package domain はレーティングコンテキストのドメインモデルを定義する。
// 計算式は docs/domain/rating.md の「スマメイト式」で、整数演算のみで決定的に計算する。
package domain

import (
	"context"
	"sort"
	"time"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// 計算式のパラメータ。
const (
	InitialRating = 1500
	FloorRating   = 1000
	MinDelta      = 1
	MaxDelta      = 30
)

// Delta は勝者と敗者の対戦前のレートから変動値を求める。
// Δ = floor(16 + (敗者 − 勝者) ÷ 25) を [1, 30] に収める。
func Delta(winner, loser int) int {
	d := floorDiv(400+(loser-winner), 25)
	return min(max(d, MinDelta), MaxDelta)
}

// Apply は1回の対戦の結果を適用した、勝者と敗者の新しいレートを返す。敗者は 1000 より下がらない。
func Apply(winner, loser int) (newWinner, newLoser int) {
	d := Delta(winner, loser)
	return winner + d, max(loser-d, FloorRating)
}

// floorDiv は −∞ 方向に丸める整数除算（Go の / は 0 方向に丸めるため補正する）。
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// Result はレート計算の入力となる、確定した1回の対戦の結果。
type Result struct {
	RecordID string // 記録 ID（同時刻の記録の並び順に使う）
	PlayedAt time.Time
	Winner   shared.AccountID
	Loser    shared.AccountID
}

// Standing はシーズン内のプレイヤーの成績。
type Standing struct {
	AccountID shared.AccountID
	Rating    int
	Matches   int
	Wins      int
	Losses    int
}

// Compute は対戦結果の集合からシーズンの成績を計算する。
// 入力の順番に依存しないよう、played_at → 記録 ID の順に並べてから1件ずつ適用する。
func Compute(results []Result) map[shared.AccountID]Standing {
	sorted := make([]Result, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		if !sorted[i].PlayedAt.Equal(sorted[j].PlayedAt) {
			return sorted[i].PlayedAt.Before(sorted[j].PlayedAt)
		}
		return sorted[i].RecordID < sorted[j].RecordID
	})
	st := map[shared.AccountID]Standing{}
	get := func(id shared.AccountID) Standing {
		s, ok := st[id]
		if !ok {
			s = Standing{AccountID: id, Rating: InitialRating}
		}
		return s
	}
	for _, r := range sorted {
		w, l := get(r.Winner), get(r.Loser)
		w.Rating, l.Rating = Apply(w.Rating, l.Rating)
		w.Matches++
		w.Wins++
		l.Matches++
		l.Losses++
		st[r.Winner], st[r.Loser] = w, l
	}
	return st
}

// Season はレートを集計する期間。End がゼロ値なら終了日時は未定。
type Season struct {
	ID    string
	Start time.Time
	End   time.Time
}

// StandingRepository はシーズンの成績の永続化。
type StandingRepository interface {
	// Replace はシーズンの成績をすべて置き換える。
	Replace(ctx context.Context, seasonID string, standings []Standing) error
	Get(ctx context.Context, seasonID string, id shared.AccountID) (Standing, error)
	Ranking(ctx context.Context, seasonID string, minMatches, limit int) ([]Standing, error)
	// Lock はシーズンの再計算を直列化する（トランザクション内で呼ぶ）。
	Lock(ctx context.Context, seasonID string) error
}
