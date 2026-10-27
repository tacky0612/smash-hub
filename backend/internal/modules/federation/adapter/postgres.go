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
	// 記録 ID が同じなら同じ記録（内容が同じなので、どちらを残しても結果は変わらない）
	_, err = p.DB.Q(ctx).Exec(ctx,
		`INSERT INTO match_records (record_id, match_id, host, played_at, logged_at, player1_id, player2_id, winner, body)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (record_id) DO NOTHING`,
		r.RecordID(), r.MatchID, r.HostInstance, played, loggedAt, r.Players[0].AccountID, r.Players[1].AccountID, r.Winner, string(body))
	return err
}

// recordColumns は collectRecords が読む列。match_records r と match_voids v（LEFT JOIN）から選ぶ。
const recordColumns = `r.body, r.logged_at, v.reason, v.voided_at`

const recordsFrom = `match_records r LEFT JOIN match_voids v ON v.record_id = r.record_id`

func scanRecord(row pgx.Row) (application.StoredRecord, error) {
	var body string
	var reason *string
	var voidedAt *time.Time
	var r application.StoredRecord
	if err := row.Scan(&body, &r.LoggedAt, &reason, &voidedAt); err != nil {
		return r, err
	}
	if err := json.Unmarshal([]byte(body), &r.MatchRecord); err != nil {
		return r, err
	}
	if reason != nil && voidedAt != nil {
		r.Void = &application.Void{RecordID: r.RecordID(), Host: string(r.HostInstance), Reason: *reason, VoidedAt: *voidedAt}
	}
	return r, nil
}

func collectRecords(rows pgx.Rows, err error) ([]application.StoredRecord, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (application.StoredRecord, error) { return scanRecord(row) })
}

func (p Postgres) Records(ctx context.Context, from, to time.Time) ([]application.StoredRecord, error) {
	var toPtr *time.Time
	if !to.IsZero() {
		toPtr = &to
	}
	return collectRecords(p.DB.Q(ctx).Query(ctx,
		`SELECT `+recordColumns+` FROM `+recordsFrom+`
		 WHERE r.played_at >= $1 AND ($2::timestamptz IS NULL OR r.played_at < $2) ORDER BY r.played_at, r.record_id`, from, toPtr))
}

func (p Postgres) RecordsFor(ctx context.Context, account shared.AccountID, limit int) ([]application.StoredRecord, error) {
	return collectRecords(p.DB.Q(ctx).Query(ctx,
		`SELECT `+recordColumns+` FROM `+recordsFrom+` WHERE r.player1_id = $1 OR r.player2_id = $1
		 ORDER BY r.played_at DESC LIMIT $2`, account, limit))
}

func (p Postgres) RecordByID(ctx context.Context, recordID string) (application.StoredRecord, error) {
	r, err := scanRecord(p.DB.Q(ctx).QueryRow(ctx,
		`SELECT `+recordColumns+` FROM `+recordsFrom+` WHERE r.record_id = $1`, recordID))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, shared.ErrNotFound
	}
	return r, err
}

func (p Postgres) RecordByMatch(ctx context.Context, key application.MatchKey) (application.StoredRecord, error) {
	r, err := scanRecord(p.DB.Q(ctx).QueryRow(ctx,
		`SELECT `+recordColumns+` FROM `+recordsFrom+` WHERE r.host = $1 AND r.match_id = $2
		 ORDER BY r.record_id LIMIT 1`, key.Host, key.MatchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, shared.ErrNotFound
	}
	return r, err
}

func (p Postgres) InsertVoid(ctx context.Context, v application.Void) error {
	_, err := p.DB.Q(ctx).Exec(ctx,
		`INSERT INTO match_voids (record_id, host, reason, voided_at) VALUES ($1, $2, $3, $4) ON CONFLICT (record_id) DO NOTHING`,
		v.RecordID, v.Host, v.Reason, v.VoidedAt)
	return err
}

// VoidedMatches は keys のうち、有効な無効化がある対戦を返す（表示用）。
func (p Postgres) VoidedMatches(ctx context.Context, keys []application.MatchKey) (map[application.MatchKey]bool, error) {
	out := map[application.MatchKey]bool{}
	if len(keys) == 0 {
		return out, nil
	}
	hosts, ids := make([]string, len(keys)), make([]string, len(keys))
	for i, k := range keys {
		hosts[i], ids[i] = string(k.Host), string(k.MatchID)
	}
	rows, err := p.DB.Q(ctx).Query(ctx,
		`SELECT `+recordColumns+` FROM `+recordsFrom+`
		 JOIN unnest($1::text[], $2::text[]) AS k(host, match_id) ON k.host = r.host AND k.match_id = r.match_id
		 WHERE v.record_id IS NOT NULL`, hosts, ids)
	recs, err := collectRecords(rows, err)
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		if r.Voided() {
			out[application.MatchKey{Host: r.HostInstance, MatchID: r.MatchID}] = true
		}
	}
	return out, nil
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

// ChainHeads は保存しているログ（自インスタンスのログと、取り込んだ連合先のログ）ごとの末尾を、オリジンの順に返す。
// 連合先の公開鍵は取り込み時に固定したもの。自インスタンスのログの公開鍵は空で返す（呼び出し側が埋める）。
func (p Postgres) ChainHeads(ctx context.Context) ([]application.ChainHead, error) {
	rows, err := p.DB.Q(ctx).Query(ctx,
		`SELECT DISTINCT ON (e.origin) e.origin, e.seq, e.hash, COALESCE(fp.public_key, '')
		 FROM federation_events e LEFT JOIN federation_peers fp ON fp.url = e.origin
		 ORDER BY e.origin, e.seq DESC`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (application.ChainHead, error) {
		var h application.ChainHead
		err := row.Scan(&h.InstanceID, &h.Seq, &h.Hash, &h.PublicKey)
		return h, err
	})
}
