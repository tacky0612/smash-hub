// Package adapter はレーティングコンテキストの永続化と HTTP ハンドラを実装する。
package adapter

import (
	"context"
	"errors"
	"hash/fnv"

	"github.com/jackc/pgx/v5"

	"github.com/tacky0612/smash-hub/backend/internal/modules/rating/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Postgres は StandingRepository を PostgreSQL で実装する。
type Postgres struct {
	DB *database.DB
}

func (p Postgres) Lock(ctx context.Context, seasonID string) error {
	h := fnv.New32a()
	h.Write([]byte("rating:" + seasonID))
	_, err := p.DB.Q(ctx).Exec(ctx, `SELECT pg_advisory_xact_lock(727002, $1)`, int32(h.Sum32()))
	return err
}

func (p Postgres) Replace(ctx context.Context, seasonID string, standings []domain.Standing) error {
	q := p.DB.Q(ctx)
	if _, err := q.Exec(ctx, `DELETE FROM ratings WHERE season_id = $1`, seasonID); err != nil {
		return err
	}
	rows := make([][]any, len(standings))
	for i, s := range standings {
		rows[i] = []any{seasonID, string(s.AccountID), s.Rating, s.Matches, s.Wins, s.Losses}
	}
	tx, ok := q.(pgx.Tx)
	if !ok {
		return errors.New("rating: Replace must run in a transaction")
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"ratings"},
		[]string{"season_id", "account_id", "rating", "matches", "wins", "losses"}, pgx.CopyFromRows(rows))
	return err
}

func (p Postgres) Get(ctx context.Context, seasonID string, id shared.AccountID) (domain.Standing, error) {
	s := domain.Standing{AccountID: id}
	err := p.DB.Q(ctx).QueryRow(ctx,
		`SELECT rating, matches, wins, losses FROM ratings WHERE season_id = $1 AND account_id = $2`, seasonID, id).
		Scan(&s.Rating, &s.Matches, &s.Wins, &s.Losses)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, shared.ErrNotFound
	}
	return s, err
}

func (p Postgres) Ranking(ctx context.Context, seasonID string, minMatches, limit int) ([]domain.Standing, error) {
	rows, err := p.DB.Q(ctx).Query(ctx,
		`SELECT account_id, rating, matches, wins, losses FROM ratings
		 WHERE season_id = $1 AND matches >= $2
		 ORDER BY rating DESC, matches DESC, account_id LIMIT $3`, seasonID, minMatches, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Standing, error) {
		var s domain.Standing
		err := row.Scan(&s.AccountID, &s.Rating, &s.Matches, &s.Wins, &s.Losses)
		return s, err
	})
}
