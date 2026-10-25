// Package application は対戦募集（マッチング）コンテキストのユースケースを定義する。
package application

import (
	"context"
	"errors"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/domain"
	sharedapp "github.com/tacky0612/smash-hub/backend/internal/shared/application"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// MatchStarter は対戦を開始する（match コンテキストが実装し、bootstrap で接続する）。
type MatchStarter interface {
	Start(ctx context.Context, p1, p2 shared.AccountID, f1, f2 shared.FighterID) (shared.MatchID, error)
	ActiveMatchID(ctx context.Context, id shared.AccountID) (shared.MatchID, error) // なければ空文字
}

// RatingReader は現在のレートを返す（rating コンテキストが実装し、bootstrap で接続する）。
type RatingReader interface {
	CurrentRating(ctx context.Context, id shared.AccountID) (int, error)
}

// State はマッチングの状態。
type State string

// マッチングの状態。
const (
	StateIdle    State = "idle"
	StateWaiting State = "waiting"
	StateMatched State = "matched"
)

// Status はマッチングの状態と付随情報。
type Status struct {
	State       State
	MatchID     shared.MatchID
	WaitingFor  time.Duration
	AllowedDiff int
}

// Service はマッチングのユースケース。
type Service struct {
	Tx       sharedapp.TxManager
	Requests domain.Repository
	Matches  MatchStarter
	Ratings  RatingReader
	Clock    shared.Clock
}

const candidateLimit = 50

// Search は「対戦相手を探す」。条件に合う相手が待っていれば対戦を成立させ、いなければ募集を作って待つ。
func (s *Service) Search(ctx context.Context, account shared.AccountID, fighter shared.FighterID, rateRange int) (Status, error) {
	var st Status
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		now := s.Clock.Now()
		if id, err := s.Matches.ActiveMatchID(ctx, account); err != nil {
			return err
		} else if id != "" {
			st = Status{State: StateMatched, MatchID: id}
			return nil
		}
		// 既に待機中なら、新しい条件で探し直す
		if old, err := s.Requests.WaitingForUpdate(ctx, account); err == nil {
			old.Status = domain.StatusCancelled
			if err := s.Requests.Save(ctx, old); err != nil {
				return err
			}
		} else if !errors.Is(err, shared.ErrNotFound) {
			return err
		}
		rating, err := s.Ratings.CurrentRating(ctx, account)
		if err != nil {
			return err
		}
		req, err := domain.NewRequest(account, fighter, rateRange, rating, now)
		if err != nil {
			return err
		}
		if err := s.Requests.Create(ctx, req); err != nil {
			return err
		}
		st, err = s.tryMatch(ctx, req, now)
		return err
	})
	return st, err
}

// Status はマッチングの状態を返す。待機中なら募集者がまだいることを記録し、改めて相手を探す。
// 「制限なし」はレート差が時間とともに広がるため、待機中の確認ごとに探し直す。
func (s *Service) Status(ctx context.Context, account shared.AccountID) (Status, error) {
	var st Status
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		now := s.Clock.Now()
		if id, err := s.Matches.ActiveMatchID(ctx, account); err != nil {
			return err
		} else if id != "" {
			st = Status{State: StateMatched, MatchID: id}
			return nil
		}
		req, err := s.Requests.WaitingForUpdate(ctx, account)
		if errors.Is(err, shared.ErrNotFound) {
			st = Status{State: StateIdle}
			return nil
		}
		if err != nil {
			return err
		}
		req.LastSeenAt = now
		if err := s.Requests.Save(ctx, req); err != nil {
			return err
		}
		st, err = s.tryMatch(ctx, req, now)
		return err
	})
	return st, err
}

// Cancel はマッチング待ちを取り消す。
func (s *Service) Cancel(ctx context.Context, account shared.AccountID) error {
	return s.Tx.InTx(ctx, func(ctx context.Context) error {
		req, err := s.Requests.WaitingForUpdate(ctx, account)
		if errors.Is(err, shared.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		req.Status = domain.StatusCancelled
		return s.Requests.Save(ctx, req)
	})
}

// CancelStale は離脱した募集を取り消す（定期実行）。
func (s *Service) CancelStale(ctx context.Context) (int, error) {
	return s.Requests.CancelStale(ctx, s.Clock.Now().Add(-domain.StaleAfter))
}

func (s *Service) tryMatch(ctx context.Context, req domain.Request, now time.Time) (Status, error) {
	candidates, err := s.Requests.CandidatesForUpdate(ctx, req.AccountID, now.Add(-domain.StaleAfter), candidateLimit)
	if err != nil {
		return Status{}, err
	}
	for _, c := range candidates {
		if !domain.Compatible(req, c, now) {
			continue
		}
		p1, p2 := domain.Pair(req, c)
		matchID, err := s.Matches.Start(ctx, p1.AccountID, p2.AccountID, p1.Fighter, p2.Fighter)
		if err != nil {
			return Status{}, err
		}
		for _, r := range []domain.Request{req, c} {
			r.Status, r.MatchID = domain.StatusMatched, matchID
			if err := s.Requests.Save(ctx, r); err != nil {
				return Status{}, err
			}
		}
		return Status{State: StateMatched, MatchID: matchID}, nil
	}
	return Status{State: StateWaiting, WaitingFor: now.Sub(req.CreatedAt), AllowedDiff: req.AllowedDiff(now)}, nil
}
