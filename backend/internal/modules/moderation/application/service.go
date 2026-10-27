// Package application はモデレーションコンテキストのユースケースを定義する。
// 利用者の通報と、管理者による通報の確認・処分（注意・イエローカード・レッドカード）・対戦記録の無効化を扱う。
package application

import (
	"context"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/moderation/domain"
	sharedapp "github.com/tacky0612/smash-hub/backend/internal/shared/application"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Admins はアカウントが管理者かを返す（設定から bootstrap で作る）。
type Admins interface {
	IsAdmin(ctx context.Context, id shared.AccountID) (bool, error)
}

// Accounts は通報の対象のアカウントが存在するかを返す（account・federation コンテキストを bootstrap で接続する）。
// 自インスタンスのアカウントと、取り込み済みの他インスタンスのアカウントを対象にする。
type Accounts interface {
	Exists(ctx context.Context, id shared.AccountID) (bool, error)
}

// MatchVoider は自インスタンスがホストした対戦の記録を無効化する（federation コンテキストが実装する）。
type MatchVoider interface {
	VoidMatch(ctx context.Context, id shared.MatchID, reason string) error
}

// Matchmaking は利用停止にした利用者の対戦募集を取り消す（matchmaking コンテキストが実装する）。
type Matchmaking interface {
	Cancel(ctx context.Context, id shared.AccountID) error
}

// Service はモデレーションのユースケース。
type Service struct {
	Tx          sharedapp.TxManager
	Reports     domain.ReportRepository
	Sanctions   domain.SanctionRepository
	Clock       shared.Clock
	Admins      Admins
	Accounts    Accounts
	Voider      MatchVoider
	Matchmaking Matchmaking
}

// Status は利用者本人に見せるモデレーションの状態。
type Status struct {
	Admin      bool
	Suspension *domain.Sanction // 利用停止中ならその処分
	Sanctions  []domain.Sanction
}

// Report は利用者からの通報を受け付ける。
func (s *Service) Report(ctx context.Context, reporter, target shared.AccountID, matchHost shared.InstanceID, matchID shared.MatchID, category domain.Category, body string) (domain.Report, error) {
	now := s.Clock.Now()
	r, err := domain.NewReport(shared.NewID(), reporter, target, matchHost, matchID, category, body, now)
	if err != nil {
		return r, err
	}
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		if ok, err := s.Accounts.Exists(ctx, target); err != nil {
			return err
		} else if !ok {
			return shared.ErrNotFound
		}
		n, err := s.Reports.CountByReporterSince(ctx, reporter, now.Add(-24*time.Hour))
		if err != nil {
			return err
		}
		if n >= domain.ReportsPerDay {
			return shared.Invalid("通報は24時間に5件までです。時間をおいてから送ってください")
		}
		return s.Reports.Create(ctx, r)
	})
	return r, err
}

// StatusOf は利用者本人のモデレーションの状態を返す。
func (s *Service) StatusOf(ctx context.Context, id shared.AccountID) (Status, error) {
	admin, err := s.Admins.IsAdmin(ctx, id)
	if err != nil {
		return Status{}, err
	}
	sanctions, err := s.Sanctions.ListFor(ctx, id)
	if err != nil {
		return Status{}, err
	}
	st := Status{Admin: admin, Sanctions: sanctions}
	now := s.Clock.Now()
	for i := range sanctions {
		if sanctions[i].Suspends(now) {
			st.Suspension = &sanctions[i]
			break
		}
	}
	return st, nil
}

// CanPlay は id がレーティング対戦をできるかを確かめる。利用停止中なら理由付きの ErrForbidden を返す。
func (s *Service) CanPlay(ctx context.Context, id shared.AccountID) error {
	sanctions, err := s.Sanctions.ListFor(ctx, id)
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	for _, sc := range sanctions {
		if !sc.Suspends(now) {
			continue
		}
		if sc.EndsAt.IsZero() {
			return shared.Forbidden("レーティング対戦の利用を停止しています（無期限）")
		}
		return shared.Forbidden("レーティング対戦の利用を " + sc.EndsAt.In(jst).Format("2006/01/02 15:04") + " まで停止しています")
	}
	return nil
}

var jst = time.FixedZone("JST", 9*60*60)

// --- 管理者の操作 ---

func (s *Service) requireAdmin(ctx context.Context, id shared.AccountID) error {
	ok, err := s.Admins.IsAdmin(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return shared.ErrForbidden
	}
	return nil
}

// ListReports は通報の一覧を返す（管理者のみ）。
func (s *Service) ListReports(ctx context.Context, admin shared.AccountID, status domain.ReportStatus) ([]domain.Report, error) {
	if err := s.requireAdmin(ctx, admin); err != nil {
		return nil, err
	}
	if status != domain.ReportOpen && status != domain.ReportResolved {
		return nil, shared.Invalid("対応状況の指定が正しくありません")
	}
	return s.Reports.List(ctx, status, 100)
}

// ResolveReport は通報を対応済みにする（管理者のみ）。
func (s *Service) ResolveReport(ctx context.Context, admin shared.AccountID, id, note string) (domain.Report, error) {
	if err := s.requireAdmin(ctx, admin); err != nil {
		return domain.Report{}, err
	}
	var r domain.Report
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if r, err = s.Reports.GetForUpdate(ctx, id); err != nil {
			return err
		}
		if err := r.Resolve(note, s.Clock.Now()); err != nil {
			return err
		}
		return s.Reports.Save(ctx, r)
	})
	return r, err
}

// SanctionsFor はアカウントの処分の記録を返す（管理者のみ）。
func (s *Service) SanctionsFor(ctx context.Context, admin, account shared.AccountID) ([]domain.Sanction, error) {
	if err := s.requireAdmin(ctx, admin); err != nil {
		return nil, err
	}
	return s.Sanctions.ListFor(ctx, account)
}

// Sanction はアカウントを処分する（管理者のみ）。レッドカードなら待機中の対戦募集も取り消す。
// 進行中の対戦の結果報告はできる（利用停止で禁止するのは新しい対戦だけ）。
func (s *Service) Sanction(ctx context.Context, admin, account shared.AccountID, kind domain.SanctionKind, reason domain.Category, note string, endsAt time.Time) (domain.Sanction, error) {
	if err := s.requireAdmin(ctx, admin); err != nil {
		return domain.Sanction{}, err
	}
	if admin == account {
		return domain.Sanction{}, shared.Invalid("自分自身は処分できません")
	}
	sc, err := domain.NewSanction(shared.NewID(), account, kind, reason, note, endsAt, s.Clock.Now())
	if err != nil {
		return sc, err
	}
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		if ok, err := s.Accounts.Exists(ctx, account); err != nil {
			return err
		} else if !ok {
			return shared.ErrNotFound
		}
		return s.Sanctions.Create(ctx, sc)
	})
	if err != nil {
		return sc, err
	}
	if sc.Kind == domain.SanctionRed && s.Matchmaking != nil {
		if err := s.Matchmaking.Cancel(ctx, account); err != nil {
			return sc, err
		}
	}
	return sc, nil
}

// LiftSanction は処分を解除する（管理者のみ）。
func (s *Service) LiftSanction(ctx context.Context, admin shared.AccountID, id string) (domain.Sanction, error) {
	if err := s.requireAdmin(ctx, admin); err != nil {
		return domain.Sanction{}, err
	}
	var sc domain.Sanction
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if sc, err = s.Sanctions.GetForUpdate(ctx, id); err != nil {
			return err
		}
		if err := sc.Lift(s.Clock.Now()); err != nil {
			return err
		}
		return s.Sanctions.Save(ctx, sc)
	})
	return sc, err
}

// VoidMatch は自インスタンスがホストした対戦の記録を無効化する（管理者のみ）。
// 無効化は連合で共有され、どのインスタンスでもその記録はレート計算から除外される。
func (s *Service) VoidMatch(ctx context.Context, admin shared.AccountID, id shared.MatchID, reason string) error {
	if err := s.requireAdmin(ctx, admin); err != nil {
		return err
	}
	return s.Voider.VoidMatch(ctx, id, reason)
}

// PurgeReports は対応完了から保持期間を過ぎた通報を削除する（定期実行）。処分の記録は残す。
func (s *Service) PurgeReports(ctx context.Context) (int64, error) {
	return s.Reports.PurgeResolved(ctx, s.Clock.Now().Add(-domain.ReportRetention))
}
