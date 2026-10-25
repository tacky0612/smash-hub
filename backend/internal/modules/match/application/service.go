// Package application は対戦コンテキストのユースケースを定義する。
package application

import (
	"context"
	"errors"

	"github.com/tacky0612/smash-hub/backend/internal/modules/match/domain"
	sharedapp "github.com/tacky0612/smash-hub/backend/internal/shared/application"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// ResultsChanged は確定した対戦記録が変わったことを受け取る（rating コンテキストが実装し、bootstrap で接続する）。
// 対戦の確定と同じトランザクション内で呼ばれる。
type ResultsChanged interface {
	OnResultsChanged(ctx context.Context) error
}

// Service は対戦のユースケース。
type Service struct {
	Tx      sharedapp.TxManager
	Matches domain.Repository
	Clock   shared.Clock
	Changed ResultsChanged
}

// Start は成立した対戦を作る（matchmaking コンテキストから呼ばれる）。
func (s *Service) Start(ctx context.Context, p1, p2 shared.AccountID, f1, f2 shared.FighterID) (shared.MatchID, error) {
	m, err := domain.New(shared.MatchID(shared.NewID()), p1, p2, f1, f2, s.Clock.Now())
	if err != nil {
		return "", err
	}
	return m.ID, s.Matches.Create(ctx, m)
}

// ActiveMatchID は id の進行中の対戦 ID を返す。なければ空文字。
func (s *Service) ActiveMatchID(ctx context.Context, id shared.AccountID) (shared.MatchID, error) {
	m, err := s.Active(ctx, id)
	if errors.Is(err, shared.ErrNotFound) {
		return "", nil
	}
	return m.ID, err
}

// Get は対戦を取得する。期限を過ぎていれば期限切れにしてから返す。
func (s *Service) Get(ctx context.Context, id shared.MatchID) (domain.Match, error) {
	m, err := s.Matches.Get(ctx, id)
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

// Active は id の進行中の対戦を返す。
func (s *Service) Active(ctx context.Context, id shared.AccountID) (domain.Match, error) {
	m, err := s.Matches.Active(ctx, id)
	if err != nil {
		return m, err
	}
	if !s.Clock.Now().Before(m.ExpiresAt) {
		if err := s.expire(ctx, m.ID); err != nil {
			return m, err
		}
		return domain.Match{}, shared.ErrNotFound
	}
	return m, nil
}

// SetRoomID は専用部屋の ID を設定する。
func (s *Service) SetRoomID(ctx context.Context, by shared.AccountID, id shared.MatchID, roomID string) (domain.Match, error) {
	return s.update(ctx, id, func(m *domain.Match) (bool, error) {
		return false, m.SetRoomID(by, roomID, s.Clock.Now())
	})
}

// Report は結果を報告する。両者の報告が一致して確定したら、同じトランザクションでレートを再計算する。
func (s *Service) Report(ctx context.Context, by shared.AccountID, id shared.MatchID, r domain.Report) (domain.Match, error) {
	return s.update(ctx, id, func(m *domain.Match) (bool, error) {
		return m.SubmitReport(by, r, s.Clock.Now())
	})
}

// History は id の最近の対戦を返す。
func (s *Service) History(ctx context.Context, id shared.AccountID, limit int) ([]domain.Match, error) {
	return s.Matches.History(ctx, id, limit)
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

// update は行ロックを取って対戦を変更・保存する。fn が true を返したら確定として扱う。
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
		if confirmed && s.Changed != nil {
			if err := s.Changed.OnResultsChanged(ctx); err != nil {
				return err
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
