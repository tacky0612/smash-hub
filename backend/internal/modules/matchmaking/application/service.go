// Package application は対戦募集（マッチング）コンテキストのユースケースを定義する。
package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/domain"
	sharedapp "github.com/tacky0612/smash-hub/backend/internal/shared/application"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// MatchStarter は対戦を開始する（match コンテキストが実装し、bootstrap で接続する）。
type MatchStarter interface {
	// Start は自インスタンスをホストとして対戦を作る。p2Home はプレイヤー2が他インスタンスの利用者ならそのインスタンス。
	Start(ctx context.Context, p1, p2 shared.AccountID, f1, f2 shared.FighterID, p2Home shared.InstanceID) (shared.MatchID, error)
	// JoinRemote は他インスタンスがホストする対戦への参加を記録する。
	JoinRemote(ctx context.Context, host shared.InstanceID, id shared.MatchID, account shared.AccountID) error
	ActiveMatchID(ctx context.Context, id shared.AccountID) (shared.MatchID, error) // なければ空文字
}

// RatingReader は現在のレートを返す（rating コンテキストが実装し、bootstrap で接続する）。
// 他インスタンスの利用者のレートも、連合で共有された対戦記録から計算した値を返す。
type RatingReader interface {
	CurrentRating(ctx context.Context, id shared.AccountID) (int, error)
}

// RemoteRequests は他インスタンスとの対戦募集のやり取り（federation コンテキストが実装し、bootstrap で接続する）。
type RemoteRequests interface {
	// Candidates は他インスタンスから配信された有効な募集を返す（Origin が設定される）。
	Candidates(ctx context.Context) ([]domain.Request, error)
	// Claim は配信元（ホスト）に対戦を申し込み、成立したら対戦 ID を返す。
	Claim(ctx context.Context, target, mine domain.Request) (shared.MatchID, error)
	// Announce は自インスタンスの募集を連合先へ配信する（非同期・失敗しても続行する）。
	Announce(r domain.Request)
	// Withdraw は配信した募集を取り消す（非同期）。
	Withdraw(id string)
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
	Remote   RemoteRequests // 未設定なら他インスタンスとはマッチングしない
}

const candidateLimit = 50

// ErrRequestUnavailable は申し込まれた募集が既に終了している（または他の申し込みを処理中）。
var ErrRequestUnavailable = errors.New("match request is no longer available")

// Search は「対戦相手を探す」。条件に合う相手が待っていれば対戦を成立させ、いなければ募集を作って待つ。
func (s *Service) Search(ctx context.Context, account shared.AccountID, fighter shared.FighterID, rateRange int) (Status, error) {
	var st Status
	var req domain.Request
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
			s.withdraw(old.ID)
		} else if !errors.Is(err, shared.ErrNotFound) {
			return err
		}
		rating, err := s.Ratings.CurrentRating(ctx, account)
		if err != nil {
			return err
		}
		if req, err = domain.NewRequest(account, fighter, rateRange, rating, now); err != nil {
			return err
		}
		if err := s.Requests.Create(ctx, req); err != nil {
			return err
		}
		st, err = s.tryMatch(ctx, req, now)
		return err
	})
	s.afterMatchTry(req, st, err)
	return st, err
}

// Status はマッチングの状態を返す。待機中なら募集者がまだいることを記録し、改めて相手を探す。
// 「制限なし」はレート差が時間とともに広がるため、待機中の確認ごとに探し直す。
func (s *Service) Status(ctx context.Context, account shared.AccountID) (Status, error) {
	var st Status
	var req domain.Request
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		now := s.Clock.Now()
		if id, err := s.Matches.ActiveMatchID(ctx, account); err != nil {
			return err
		} else if id != "" {
			st = Status{State: StateMatched, MatchID: id}
			return nil
		}
		var err error
		req, err = s.Requests.WaitingForUpdate(ctx, account)
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
	s.afterMatchTry(req, st, err)
	return st, err
}

// afterMatchTry は、待機中なら募集を連合先に配信し（有効期限の更新を兼ねる）、成立したら配信を取り消す。
func (s *Service) afterMatchTry(req domain.Request, st Status, err error) {
	if err != nil || req.ID == "" || s.Remote == nil {
		return
	}
	switch st.State {
	case StateWaiting:
		s.Remote.Announce(req)
	case StateMatched:
		s.Remote.Withdraw(req.ID)
	}
}

func (s *Service) withdraw(id string) {
	if s.Remote != nil {
		s.Remote.Withdraw(id)
	}
}

// Cancel はマッチング待ちを取り消す。
func (s *Service) Cancel(ctx context.Context, account shared.AccountID) error {
	var id string
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		req, err := s.Requests.WaitingForUpdate(ctx, account)
		if errors.Is(err, shared.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		req.Status, id = domain.StatusCancelled, req.ID
		return s.Requests.Save(ctx, req)
	})
	if err == nil && id != "" {
		s.withdraw(id)
	}
	return err
}

// CancelStale は離脱した募集を取り消す（定期実行）。
func (s *Service) CancelStale(ctx context.Context) (int, error) {
	return s.Requests.CancelStale(ctx, s.Clock.Now().Add(-domain.StaleAfter))
}

// AcceptClaim はホストとして、他インスタンスの利用者からの対戦の申し込みを受け付ける。
// 申し込まれた募集が待機中で、条件（レート差）を満たせば、自インスタンスをホストとして対戦を作る。
// 申し込んだ側のレートは、申告値ではなく自インスタンスが対戦記録から計算した値を使う。
func (s *Service) AcceptClaim(ctx context.Context, from shared.InstanceID, requestID string, account shared.AccountID, fighter shared.FighterID, rateRange int, createdAt time.Time) (shared.MatchID, error) {
	var matchID shared.MatchID
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		now := s.Clock.Now()
		req, err := s.Requests.WaitingByIDForUpdate(ctx, requestID)
		if errors.Is(err, shared.ErrNotFound) || (err == nil && req.IsStale(now)) {
			return ErrRequestUnavailable
		}
		if err != nil {
			return err
		}
		rating, err := s.Ratings.CurrentRating(ctx, account)
		if err != nil {
			return err
		}
		claimant, err := domain.NewRequest(account, fighter, rateRange, rating, createdAt)
		if err != nil {
			return err
		}
		claimant.Origin = from
		if !domain.Compatible(req, claimant, now) {
			return ErrRequestUnavailable
		}
		if matchID, err = s.Matches.Start(ctx, req.AccountID, account, req.Fighter, fighter, from); err != nil {
			return err
		}
		req.Status, req.MatchID = domain.StatusMatched, matchID
		return s.Requests.Save(ctx, req)
	})
	if err == nil {
		s.withdraw(requestID)
	}
	return matchID, err
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
		matchID, err := s.Matches.Start(ctx, p1.AccountID, p2.AccountID, p1.Fighter, p2.Fighter, "")
		if err != nil {
			return Status{}, err
		}
		for _, r := range []domain.Request{req, c} {
			r.Status, r.MatchID = domain.StatusMatched, matchID
			if err := s.Requests.Save(ctx, r); err != nil {
				return Status{}, err
			}
		}
		withdrawLater := c.ID
		defer s.withdraw(withdrawLater)
		return Status{State: StateMatched, MatchID: matchID}, nil
	}
	if st, ok, err := s.tryRemote(ctx, req, now); err != nil || ok {
		return st, err
	}
	return Status{State: StateWaiting, WaitingFor: now.Sub(req.CreatedAt), AllowedDiff: req.AllowedDiff(now)}, nil
}

// tryRemote は他インスタンスから配信された募集に申し込む。
// 先に待っていた側がホストになるため、自分より先に作られた募集にだけ申し込む（双方が同時に申し込み合うのを防ぐ）。
func (s *Service) tryRemote(ctx context.Context, req domain.Request, now time.Time) (Status, bool, error) {
	if s.Remote == nil {
		return Status{}, false, nil
	}
	remotes, err := s.Remote.Candidates(ctx)
	if err != nil {
		slog.Warn("remote match requests unavailable", "err", err)
		return Status{}, false, nil
	}
	for _, c := range remotes {
		if c.AccountID == req.AccountID || !c.CreatedAt.Before(req.CreatedAt) {
			continue
		}
		// 配信された申告値ではなく、自インスタンスが対戦記録から計算したレートで判定する
		if c.Rating, err = s.Ratings.CurrentRating(ctx, c.AccountID); err != nil {
			return Status{}, false, err
		}
		if !domain.Compatible(req, c, now) {
			continue
		}
		matchID, err := s.Remote.Claim(ctx, c, req)
		if err != nil {
			slog.Info("claim rejected", "host", c.Origin, "request", c.ID, "err", err)
			continue
		}
		if err := s.Matches.JoinRemote(ctx, c.Origin, matchID, req.AccountID); err != nil {
			return Status{}, false, err
		}
		req.Status, req.MatchID = domain.StatusMatched, matchID
		if err := s.Requests.Save(ctx, req); err != nil {
			return Status{}, false, err
		}
		return Status{State: StateMatched, MatchID: matchID}, true, nil
	}
	return Status{}, false, nil
}
