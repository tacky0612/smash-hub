// Package application はレーティングコンテキストのユースケースを定義する。
package application

import (
	"context"
	"errors"
	"sort"

	"github.com/tacky0612/smash-hub/backend/internal/modules/rating/domain"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// ResultSource は確定した対戦の結果を提供する（match コンテキストが実装し、bootstrap で接続する）。
type ResultSource interface {
	ConfirmedResults(ctx context.Context, season domain.Season) ([]domain.Result, error)
}

// Service はレーティングのユースケース。
type Service struct {
	Standings  domain.StandingRepository
	Results    ResultSource
	Season     domain.Season
	MinMatches int // ランキング掲載に必要な対戦数
}

// Recompute は現在のシーズンの成績を、確定した対戦記録から計算し直す。
// レートは保存の正ではなく派生データなので、いつ・どこで実行しても同じ結果になる。
// 呼び出し側のトランザクション内で実行する。
func (s *Service) Recompute(ctx context.Context) error {
	if err := s.Standings.Lock(ctx, s.Season.ID); err != nil {
		return err
	}
	results, err := s.Results.ConfirmedResults(ctx, s.Season)
	if err != nil {
		return err
	}
	computed := domain.Compute(results)
	standings := make([]domain.Standing, 0, len(computed))
	for _, st := range computed {
		standings = append(standings, st)
	}
	sort.Slice(standings, func(i, j int) bool { return standings[i].AccountID < standings[j].AccountID })
	return s.Standings.Replace(ctx, s.Season.ID, standings)
}

// Standing は現在のシーズンの成績を返す。対戦がなければ初期レート。
func (s *Service) Standing(ctx context.Context, id shared.AccountID) (domain.Standing, error) {
	st, err := s.Standings.Get(ctx, s.Season.ID, id)
	if errors.Is(err, shared.ErrNotFound) {
		return domain.Standing{AccountID: id, Rating: domain.InitialRating}, nil
	}
	return st, err
}

// CurrentRating は現在のシーズンのレートを返す（matchmaking から使う）。
func (s *Service) CurrentRating(ctx context.Context, id shared.AccountID) (int, error) {
	st, err := s.Standing(ctx, id)
	return st.Rating, err
}

// Ranking はランキング掲載条件を満たすプレイヤーをレート順に返す。
func (s *Service) Ranking(ctx context.Context, limit int) ([]domain.Standing, error) {
	return s.Standings.Ranking(ctx, s.Season.ID, s.MinMatches, limit)
}
