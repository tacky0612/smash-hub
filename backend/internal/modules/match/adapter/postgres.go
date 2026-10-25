// Package adapter は対戦コンテキストの永続化と HTTP ハンドラを実装する。
package adapter

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/tacky0612/smash-hub/backend/internal/modules/match/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Postgres は domain.Repository を PostgreSQL で実装する。
type Postgres struct {
	DB *database.DB
}

const matchColumns = `id, player1_id, player2_id, fighter1, fighter2, room_id, status, report1, report2,
	COALESCE(winner_id, ''), played_at, expires_at, COALESCE(resolved_at, 'epoch'::timestamptz)`

func scanMatch(row pgx.Row) (domain.Match, error) {
	var m domain.Match
	err := row.Scan(&m.ID, &m.Player1, &m.Player2, &m.Fighter1, &m.Fighter2, &m.RoomID, &m.Status,
		&m.Report1, &m.Report2, &m.Winner, &m.PlayedAt, &m.ExpiresAt, &m.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, shared.ErrNotFound
	}
	if m.ResolvedAt.Equal(time.Unix(0, 0)) {
		m.ResolvedAt = time.Time{}
	}
	return m, err
}

func collect(rows pgx.Rows, err error) ([]domain.Match, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Match, error) { return scanMatch(row) })
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (p Postgres) Create(ctx context.Context, m domain.Match) error {
	_, err := p.DB.Q(ctx).Exec(ctx,
		`INSERT INTO matches (id, player1_id, player2_id, fighter1, fighter2, room_id, status, report1, report2, played_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		m.ID, m.Player1, m.Player2, m.Fighter1, m.Fighter2, m.RoomID, m.Status, m.Report1, m.Report2, m.PlayedAt, m.ExpiresAt)
	return err
}

func (p Postgres) Get(ctx context.Context, id shared.MatchID) (domain.Match, error) {
	return scanMatch(p.DB.Q(ctx).QueryRow(ctx, `SELECT `+matchColumns+` FROM matches WHERE id = $1`, id))
}

func (p Postgres) GetForUpdate(ctx context.Context, id shared.MatchID) (domain.Match, error) {
	return scanMatch(p.DB.Q(ctx).QueryRow(ctx, `SELECT `+matchColumns+` FROM matches WHERE id = $1 FOR UPDATE`, id))
}

func (p Postgres) Save(ctx context.Context, m domain.Match) error {
	_, err := p.DB.Q(ctx).Exec(ctx,
		`UPDATE matches SET room_id = $2, status = $3, report1 = $4, report2 = $5, winner_id = NULLIF($6, ''), resolved_at = $7
		 WHERE id = $1`,
		m.ID, m.RoomID, m.Status, m.Report1, m.Report2, string(m.Winner), nullTime(m.ResolvedAt))
	return err
}

func (p Postgres) Active(ctx context.Context, id shared.AccountID) (domain.Match, error) {
	return scanMatch(p.DB.Q(ctx).QueryRow(ctx,
		`SELECT `+matchColumns+` FROM matches
		 WHERE status = 'in_progress' AND (player1_id = $1 OR player2_id = $1)
		 ORDER BY played_at DESC LIMIT 1`, id))
}

func (p Postgres) History(ctx context.Context, id shared.AccountID, limit int) ([]domain.Match, error) {
	return collect(p.DB.Q(ctx).Query(ctx,
		`SELECT `+matchColumns+` FROM matches WHERE player1_id = $1 OR player2_id = $1
		 ORDER BY played_at DESC LIMIT $2`, id, limit))
}

func (p Postgres) ConfirmedBetween(ctx context.Context, from, to time.Time) ([]domain.Match, error) {
	return collect(p.DB.Q(ctx).Query(ctx,
		`SELECT `+matchColumns+` FROM matches
		 WHERE status = 'confirmed' AND played_at >= $1 AND ($2::timestamptz IS NULL OR played_at < $2)
		 ORDER BY played_at, id`, from, nullTime(to)))
}

func (p Postgres) DueForExpiry(ctx context.Context, now time.Time, limit int) ([]shared.MatchID, error) {
	rows, err := p.DB.Q(ctx).Query(ctx,
		`SELECT id FROM matches WHERE status = 'in_progress' AND expires_at <= $1 ORDER BY expires_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[shared.MatchID])
}
