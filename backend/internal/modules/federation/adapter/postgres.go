// Package adapter は連合コンテキストの永続化・HTTP ハンドラ・連合先クライアントを実装する。
package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/application"
	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Postgres は application.Store を PostgreSQL で実装する。
type Postgres struct {
	DB *database.DB
}

var _ application.Store = Postgres{}

func (p Postgres) AppendOwn(ctx context.Context, origin string, build func(seq int64, prev string) (domain.Event, error)) (domain.Event, error) {
	var e domain.Event
	err := p.DB.InTx(ctx, func(ctx context.Context) error {
		q := p.DB.Q(ctx)
		// 自インスタンスのログへの追記を直列化する（seq と prev を一意に決めるため）
		if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(727003)`); err != nil {
			return err
		}
		var seq int64
		var prev string
		err := q.QueryRow(ctx, `SELECT seq, hash FROM federation_events WHERE origin = $1 ORDER BY seq DESC LIMIT 1`, origin).Scan(&seq, &prev)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if e, err = build(seq+1, prev); err != nil {
			return err
		}
		return p.StoreEvent(ctx, e)
	})
	return e, err
}

func (p Postgres) StoreEvent(ctx context.Context, e domain.Event) error {
	created, err := time.Parse(time.RFC3339, e.CreatedAt)
	if err != nil {
		return err
	}
	_, err = p.DB.Q(ctx).Exec(ctx,
		`INSERT INTO federation_events (origin, seq, type, hash, body, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		e.InstanceID, e.Seq, e.Type, e.Hash(), string(e.Body()), created)
	return err
}

func (p Postgres) Events(ctx context.Context, origin string, since int64, limit int) ([]json.RawMessage, error) {
	rows, err := p.DB.Q(ctx).Query(ctx,
		`SELECT body FROM federation_events WHERE origin = $1 AND seq > $2 ORDER BY seq LIMIT $3`, origin, since, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (json.RawMessage, error) {
		var body string
		err := row.Scan(&body)
		return json.RawMessage(body), err
	})
}

func (p Postgres) Peer(ctx context.Context, url string) (application.PeerState, error) {
	q := p.DB.Q(ctx)
	if _, err := q.Exec(ctx, `INSERT INTO federation_peers (url) VALUES ($1) ON CONFLICT DO NOTHING`, url); err != nil {
		return application.PeerState{}, err
	}
	st := application.PeerState{URL: url}
	err := q.QueryRow(ctx, `SELECT public_key, last_seq, last_hash FROM federation_peers WHERE url = $1`, url).
		Scan(&st.PublicKey, &st.LastSeq, &st.LastHash)
	return st, err
}

func (p Postgres) SavePeer(ctx context.Context, st application.PeerState, syncErr string, now time.Time) error {
	_, err := p.DB.Q(ctx).Exec(ctx,
		`UPDATE federation_peers SET public_key = $2, last_seq = $3, last_hash = $4, last_error = $5, last_synced_at = $6 WHERE url = $1`,
		st.URL, st.PublicKey, st.LastSeq, st.LastHash, syncErr, now)
	return err
}

func (p Postgres) InsertRecord(ctx context.Context, r shared.MatchRecord, loggedAt time.Time) error {
	played, err := time.Parse(time.RFC3339, r.PlayedAt)
	if err != nil {
		return err
	}
	body, err := shared.Canonical(r)
	if err != nil {
		return err
	}
	// 同じ対戦 ID の記録は1件だけ（最初に記録したものが正）
	_, err = p.DB.Q(ctx).Exec(ctx,
		`INSERT INTO match_records (match_id, host, played_at, logged_at, player1_id, player2_id, winner, body)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (match_id) DO NOTHING`,
		r.MatchID, r.HostInstance, played, loggedAt, r.Players[0].AccountID, r.Players[1].AccountID, r.Winner, string(body))
	return err
}

func collectRecords(rows pgx.Rows, err error) ([]application.StoredRecord, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (application.StoredRecord, error) {
		var body string
		var r application.StoredRecord
		if err := row.Scan(&body, &r.LoggedAt); err != nil {
			return r, err
		}
		return r, json.Unmarshal([]byte(body), &r.MatchRecord)
	})
}

func (p Postgres) Records(ctx context.Context, from, to time.Time) ([]application.StoredRecord, error) {
	var toPtr *time.Time
	if !to.IsZero() {
		toPtr = &to
	}
	return collectRecords(p.DB.Q(ctx).Query(ctx,
		`SELECT body, logged_at FROM match_records
		 WHERE played_at >= $1 AND ($2::timestamptz IS NULL OR played_at < $2) ORDER BY played_at, match_id`, from, toPtr))
}

func (p Postgres) RecordsFor(ctx context.Context, account shared.AccountID, limit int) ([]application.StoredRecord, error) {
	return collectRecords(p.DB.Q(ctx).Query(ctx,
		`SELECT body, logged_at FROM match_records WHERE player1_id = $1 OR player2_id = $1
		 ORDER BY played_at DESC LIMIT $2`, account, limit))
}

func (p Postgres) UpsertRemoteAccount(ctx context.Context, a application.RemoteAccount, now time.Time) error {
	_, err := p.DB.Q(ctx).Exec(ctx,
		`INSERT INTO remote_accounts (account_id, display_name, home_instance, updated_at) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (account_id) DO UPDATE SET display_name = $2, home_instance = $3, updated_at = $4`,
		a.AccountID, a.DisplayName, a.HomeInstance, now)
	return err
}

func (p Postgres) RemoteAccount(ctx context.Context, id shared.AccountID) (application.RemoteAccount, error) {
	a := application.RemoteAccount{AccountID: id}
	err := p.DB.Q(ctx).QueryRow(ctx, `SELECT display_name, home_instance FROM remote_accounts WHERE account_id = $1`, id).
		Scan(&a.DisplayName, &a.HomeInstance)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, shared.ErrNotFound
	}
	return a, err
}

func (p Postgres) UpsertRemoteRequest(ctx context.Context, r application.RemoteRequest) error {
	_, err := p.DB.Q(ctx).Exec(ctx,
		`INSERT INTO remote_match_requests (id, origin, account_id, fighter, rate_range, rating, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id) DO UPDATE SET rate_range = $5, rating = $6, expires_at = $8
		 WHERE remote_match_requests.origin = $2`,
		r.ID, r.Origin, r.AccountID, r.Fighter, r.RateRange, r.Rating, r.CreatedAt, r.ExpiresAt)
	return err
}

func (p Postgres) DeleteRemoteRequest(ctx context.Context, origin, id string) error {
	_, err := p.DB.Q(ctx).Exec(ctx, `DELETE FROM remote_match_requests WHERE origin = $1 AND id = $2`, origin, id)
	return err
}

func (p Postgres) RemoteRequests(ctx context.Context, now time.Time) ([]application.RemoteRequest, error) {
	q := p.DB.Q(ctx)
	if _, err := q.Exec(ctx, `DELETE FROM remote_match_requests WHERE expires_at < $1`, now); err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx,
		`SELECT id, origin, account_id, fighter, rate_range, rating, created_at, expires_at FROM remote_match_requests ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (application.RemoteRequest, error) {
		var r application.RemoteRequest
		err := row.Scan(&r.ID, &r.Origin, &r.AccountID, &r.Fighter, &r.RateRange, &r.Rating, &r.CreatedAt, &r.ExpiresAt)
		return r, err
	})
}
