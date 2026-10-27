// Package adapter はモデレーションコンテキストの永続化と HTTP ハンドラを実装する。
package adapter

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/tacky0612/smash-hub/backend/internal/modules/moderation/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Postgres はモデレーションの永続化を PostgreSQL で実装する。
type Postgres struct {
	DB *database.DB
}

// Reports は domain.ReportRepository を返す。
func (p Postgres) Reports() domain.ReportRepository { return reports{p.DB} }

// Sanctions は domain.SanctionRepository を返す。
func (p Postgres) Sanctions() domain.SanctionRepository { return sanctions{p.DB} }

// nullTime はゼロ値の時刻を NULL として書き込む。
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func fromNull(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

type reports struct{ db *database.DB }

const reportColumns = `id, reporter_id, target_id, match_host, match_id, category, body, status, resolution, created_at, resolved_at`

func scanReport(row pgx.Row) (domain.Report, error) {
	var r domain.Report
	var resolved *time.Time
	err := row.Scan(&r.ID, &r.Reporter, &r.Target, &r.MatchHost, &r.MatchID, &r.Category, &r.Body, &r.Status, &r.Resolution, &r.CreatedAt, &resolved)
	r.ResolvedAt = fromNull(resolved)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, shared.ErrNotFound
	}
	return r, err
}

func (s reports) Create(ctx context.Context, r domain.Report) error {
	_, err := s.db.Q(ctx).Exec(ctx,
		`INSERT INTO reports (`+reportColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		r.ID, r.Reporter, r.Target, r.MatchHost, r.MatchID, r.Category, r.Body, r.Status, r.Resolution, r.CreatedAt, nullTime(r.ResolvedAt))
	return err
}

func (s reports) GetForUpdate(ctx context.Context, id string) (domain.Report, error) {
	return scanReport(s.db.Q(ctx).QueryRow(ctx, `SELECT `+reportColumns+` FROM reports WHERE id = $1 FOR UPDATE`, id))
}

func (s reports) Save(ctx context.Context, r domain.Report) error {
	_, err := s.db.Q(ctx).Exec(ctx,
		`UPDATE reports SET status = $2, resolution = $3, resolved_at = $4 WHERE id = $1`,
		r.ID, r.Status, r.Resolution, nullTime(r.ResolvedAt))
	return err
}

func (s reports) List(ctx context.Context, status domain.ReportStatus, limit int) ([]domain.Report, error) {
	// 未対応は古い順（対応の順番）、対応済みは新しい順
	order := `created_at`
	if status == domain.ReportResolved {
		order = `resolved_at DESC`
	}
	rows, err := s.db.Q(ctx).Query(ctx,
		`SELECT `+reportColumns+` FROM reports WHERE status = $1 ORDER BY `+order+` LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Report, error) { return scanReport(row) })
}

func (s reports) CountByReporterSince(ctx context.Context, reporter shared.AccountID, since time.Time) (int, error) {
	var n int
	err := s.db.Q(ctx).QueryRow(ctx, `SELECT count(*) FROM reports WHERE reporter_id = $1 AND created_at > $2`, reporter, since).Scan(&n)
	return n, err
}

func (s reports) PurgeResolved(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.db.Q(ctx).Exec(ctx, `DELETE FROM reports WHERE status = 'resolved' AND resolved_at < $1`, before)
	return tag.RowsAffected(), err
}

type sanctions struct{ db *database.DB }

const sanctionColumns = `id, account_id, kind, reason, note, created_at, ends_at, lifted_at`

func scanSanction(row pgx.Row) (domain.Sanction, error) {
	var s domain.Sanction
	var ends, lifted *time.Time
	err := row.Scan(&s.ID, &s.Account, &s.Kind, &s.Reason, &s.Note, &s.CreatedAt, &ends, &lifted)
	s.EndsAt, s.LiftedAt = fromNull(ends), fromNull(lifted)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, shared.ErrNotFound
	}
	return s, err
}

func (r sanctions) Create(ctx context.Context, s domain.Sanction) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`INSERT INTO sanctions (`+sanctionColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		s.ID, s.Account, s.Kind, s.Reason, s.Note, s.CreatedAt, nullTime(s.EndsAt), nullTime(s.LiftedAt))
	return err
}

func (r sanctions) GetForUpdate(ctx context.Context, id string) (domain.Sanction, error) {
	return scanSanction(r.db.Q(ctx).QueryRow(ctx, `SELECT `+sanctionColumns+` FROM sanctions WHERE id = $1 FOR UPDATE`, id))
}

func (r sanctions) Save(ctx context.Context, s domain.Sanction) error {
	_, err := r.db.Q(ctx).Exec(ctx, `UPDATE sanctions SET lifted_at = $2 WHERE id = $1`, s.ID, nullTime(s.LiftedAt))
	return err
}

func (r sanctions) ListFor(ctx context.Context, account shared.AccountID) ([]domain.Sanction, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT `+sanctionColumns+` FROM sanctions WHERE account_id = $1 ORDER BY created_at DESC, id`, account)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Sanction, error) { return scanSanction(row) })
}
