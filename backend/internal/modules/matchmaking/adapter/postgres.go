// Package adapter は対戦募集コンテキストの永続化と HTTP ハンドラを実装する。
package adapter

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Postgres は domain.Repository を PostgreSQL で実装する。
type Postgres struct {
	DB *database.DB
}

const requestColumns = `id, account_id, fighter, rate_range, rating, status, COALESCE(match_id, ''), created_at, last_seen_at`

func scanRequest(row pgx.Row) (domain.Request, error) {
	var r domain.Request
	err := row.Scan(&r.ID, &r.AccountID, &r.Fighter, &r.RateRange, &r.Rating, &r.Status, &r.MatchID, &r.CreatedAt, &r.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, shared.ErrNotFound
	}
	return r, err
}

func (p Postgres) Create(ctx context.Context, r domain.Request) error {
	_, err := p.DB.Q(ctx).Exec(ctx,
		`INSERT INTO match_requests (id, account_id, fighter, rate_range, rating, status, match_id, created_at, last_seen_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9)`,
		r.ID, r.AccountID, r.Fighter, r.RateRange, r.Rating, r.Status, string(r.MatchID), r.CreatedAt, r.LastSeenAt)
	return err
}

func (p Postgres) Save(ctx context.Context, r domain.Request) error {
	_, err := p.DB.Q(ctx).Exec(ctx,
		`UPDATE match_requests SET status = $2, match_id = NULLIF($3, ''), last_seen_at = $4 WHERE id = $1`,
		r.ID, r.Status, string(r.MatchID), r.LastSeenAt)
	return err
}

func (p Postgres) WaitingForUpdate(ctx context.Context, account shared.AccountID) (domain.Request, error) {
	return scanRequest(p.DB.Q(ctx).QueryRow(ctx,
		`SELECT `+requestColumns+` FROM match_requests WHERE account_id = $1 AND status = 'waiting' FOR UPDATE`, account))
}

func (p Postgres) WaitingByIDForUpdate(ctx context.Context, id string) (domain.Request, error) {
	return scanRequest(p.DB.Q(ctx).QueryRow(ctx,
		`SELECT `+requestColumns+` FROM match_requests WHERE id = $1 AND status = 'waiting' FOR UPDATE SKIP LOCKED`, id))
}

func (p Postgres) CandidatesForUpdate(ctx context.Context, account shared.AccountID, seenAfter time.Time, limit int) ([]domain.Request, error) {
	rows, err := p.DB.Q(ctx).Query(ctx,
		`SELECT `+requestColumns+` FROM match_requests
		 WHERE status = 'waiting' AND account_id <> $1 AND last_seen_at >= $2
		 ORDER BY created_at LIMIT $3 FOR UPDATE SKIP LOCKED`, account, seenAfter, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Request, error) { return scanRequest(row) })
}

func (p Postgres) CancelStale(ctx context.Context, seenBefore time.Time) (int, error) {
	tag, err := p.DB.Q(ctx).Exec(ctx,
		`UPDATE match_requests SET status = 'cancelled' WHERE status = 'waiting' AND last_seen_at < $1`, seenBefore)
	return int(tag.RowsAffected()), err
}
