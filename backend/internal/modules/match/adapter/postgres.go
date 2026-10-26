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

// Postgres は domain.Repository を PostgreSQL で実装する。matches テーブルの対戦はすべて自インスタンス（Self）がホスト。
type Postgres struct {
	DB   *database.DB
	Self shared.InstanceID
}

const matchColumns = `id, player1_id, player2_id, fighter1, fighter2, room_id, status, report1, report2,
	COALESCE(winner_id, ''), played_at, expires_at, COALESCE(resolved_at, 'epoch'::timestamptz), player2_home, sig1, sig2`

func (p Postgres) scanMatch(row pgx.Row) (domain.Match, error) {
	m := domain.Match{Host: p.Self}
	err := row.Scan(&m.ID, &m.Player1, &m.Player2, &m.Fighter1, &m.Fighter2, &m.RoomID, &m.Status,
		&m.Report1, &m.Report2, &m.Winner, &m.PlayedAt, &m.ExpiresAt, &m.ResolvedAt, &m.Player2Home, &m.Sig1, &m.Sig2)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, shared.ErrNotFound
	}
	if m.ResolvedAt.Equal(time.Unix(0, 0)) {
		m.ResolvedAt = time.Time{}
	}
	return m, err
}

func (p Postgres) collect(rows pgx.Rows, err error) ([]domain.Match, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Match, error) { return p.scanMatch(row) })
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (p Postgres) Create(ctx context.Context, m domain.Match) error {
	_, err := p.DB.Q(ctx).Exec(ctx,
		`INSERT INTO matches (id, player1_id, player2_id, fighter1, fighter2, room_id, status, report1, report2, played_at, expires_at, player2_home)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		m.ID, m.Player1, m.Player2, m.Fighter1, m.Fighter2, m.RoomID, m.Status, m.Report1, m.Report2, m.PlayedAt, m.ExpiresAt, m.Player2Home)
	return err
}

func (p Postgres) Get(ctx context.Context, id shared.MatchID) (domain.Match, error) {
	return p.scanMatch(p.DB.Q(ctx).QueryRow(ctx, `SELECT `+matchColumns+` FROM matches WHERE id = $1`, id))
}

func (p Postgres) GetForUpdate(ctx context.Context, id shared.MatchID) (domain.Match, error) {
	return p.scanMatch(p.DB.Q(ctx).QueryRow(ctx, `SELECT `+matchColumns+` FROM matches WHERE id = $1 FOR UPDATE`, id))
}

func (p Postgres) Save(ctx context.Context, m domain.Match) error {
	_, err := p.DB.Q(ctx).Exec(ctx,
		`UPDATE matches SET room_id = $2, status = $3, report1 = $4, report2 = $5, winner_id = NULLIF($6, ''), resolved_at = $7,
		 sig1 = $8, sig2 = $9 WHERE id = $1`,
		m.ID, m.RoomID, m.Status, m.Report1, m.Report2, string(m.Winner), nullTime(m.ResolvedAt), m.Sig1, m.Sig2)
	return err
}

func (p Postgres) Active(ctx context.Context, id shared.AccountID) (domain.Match, error) {
	return p.scanMatch(p.DB.Q(ctx).QueryRow(ctx,
		`SELECT `+matchColumns+` FROM matches
		 WHERE status = 'in_progress' AND (player1_id = $1 OR player2_id = $1)
		 ORDER BY played_at DESC LIMIT 1`, id))
}

func (p Postgres) History(ctx context.Context, id shared.AccountID, limit int) ([]domain.Match, error) {
	return p.collect(p.DB.Q(ctx).Query(ctx,
		`SELECT `+matchColumns+` FROM matches WHERE player1_id = $1 OR player2_id = $1
		 ORDER BY played_at DESC LIMIT $2`, id, limit))
}

func (p Postgres) ConfirmedBetween(ctx context.Context, from, to time.Time) ([]domain.Match, error) {
	return p.collect(p.DB.Q(ctx).Query(ctx,
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

// RemoteRefs は domain.RemoteRefRepository を PostgreSQL で実装する。
type RemoteRefs struct {
	DB *database.DB
}

func (r RemoteRefs) Create(ctx context.Context, ref domain.RemoteRef) error {
	_, err := r.DB.Q(ctx).Exec(ctx,
		`INSERT INTO remote_matches (match_id, host, account_id, status, created_at) VALUES ($1, $2, $3, $4, $5)`,
		ref.MatchID, ref.Host, ref.AccountID, ref.Status, ref.CreatedAt)
	return err
}

func scanRef(row pgx.Row) (domain.RemoteRef, error) {
	var ref domain.RemoteRef
	err := row.Scan(&ref.MatchID, &ref.Host, &ref.AccountID, &ref.Status, &ref.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ref, shared.ErrNotFound
	}
	return ref, err
}

func (r RemoteRefs) Get(ctx context.Context, id shared.MatchID) (domain.RemoteRef, error) {
	return scanRef(r.DB.Q(ctx).QueryRow(ctx,
		`SELECT match_id, host, account_id, status, created_at FROM remote_matches WHERE match_id = $1`, id))
}

func (r RemoteRefs) Active(ctx context.Context, account shared.AccountID) ([]domain.RemoteRef, error) {
	rows, err := r.DB.Q(ctx).Query(ctx,
		`SELECT match_id, host, account_id, status, created_at FROM remote_matches
		 WHERE account_id = $1 AND status = 'in_progress' ORDER BY created_at DESC`, account)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.RemoteRef, error) { return scanRef(row) })
}

func (r RemoteRefs) SetStatus(ctx context.Context, id shared.MatchID, s domain.Status) error {
	_, err := r.DB.Q(ctx).Exec(ctx, `UPDATE remote_matches SET status = $2 WHERE match_id = $1`, id, s)
	return err
}
