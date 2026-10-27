package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/application"
	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/domain"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Memory は application.Store をメモリ上で実装する。DB を使わずにスナップショットを検証するときに使う。
// 振る舞い（重複の扱い・並び順）は Postgres と揃える。
type Memory struct {
	mu       sync.Mutex
	events   map[string][]domain.Event // オリジンごと、seq の順
	peers    map[string]application.PeerState
	records  map[string]storedRecord // 記録 ID → 記録
	voids    map[string]application.Void
	accounts map[shared.AccountID]application.RemoteAccount
	requests map[string]application.RemoteRequest
}

type storedRecord struct {
	shared.MatchRecord
	playedAt time.Time
	loggedAt time.Time
}

// NewMemory は空の Memory を返す。
func NewMemory() *Memory {
	return &Memory{
		events:   map[string][]domain.Event{},
		peers:    map[string]application.PeerState{},
		records:  map[string]storedRecord{},
		voids:    map[string]application.Void{},
		accounts: map[shared.AccountID]application.RemoteAccount{},
		requests: map[string]application.RemoteRequest{},
	}
}

var _ application.Store = (*Memory)(nil)

func (m *Memory) AppendOwn(ctx context.Context, origin string, build func(seq int64, prev string) (domain.Event, error)) (domain.Event, error) {
	m.mu.Lock()
	var seq int64
	prev := ""
	if evs := m.events[origin]; len(evs) > 0 {
		seq, prev = evs[len(evs)-1].Seq, evs[len(evs)-1].Hash()
	}
	m.mu.Unlock()
	e, err := build(seq+1, prev)
	if err != nil {
		return e, err
	}
	return e, m.StoreEvent(ctx, e)
}

func (m *Memory) StoreEvent(_ context.Context, e domain.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	evs := m.events[e.InstanceID]
	if n := len(evs); n > 0 && evs[n-1].Seq >= e.Seq {
		return fmt.Errorf("event %s/%d already exists", e.InstanceID, e.Seq)
	}
	m.events[e.InstanceID] = append(evs, e)
	return nil
}

func (m *Memory) Events(_ context.Context, origin string, since int64, limit int) ([]json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []json.RawMessage
	for _, e := range m.events[origin] {
		if e.Seq > since && len(out) < limit {
			out = append(out, e.Body())
		}
	}
	return out, nil
}

func (m *Memory) Peer(_ context.Context, url string) (application.PeerState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.peers[url]
	if !ok {
		st = application.PeerState{URL: url}
		m.peers[url] = st
	}
	return st, nil
}

func (m *Memory) SavePeer(_ context.Context, st application.PeerState, _ string, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.peers[st.URL] = st
	return nil
}

func (m *Memory) InsertRecord(_ context.Context, r shared.MatchRecord, loggedAt time.Time) error {
	played, err := time.Parse(time.RFC3339, r.PlayedAt)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// 記録 ID が同じなら同じ記録（Postgres の ON CONFLICT DO NOTHING と同じ）
	if _, ok := m.records[r.RecordID()]; !ok {
		m.records[r.RecordID()] = storedRecord{MatchRecord: r, playedAt: played, loggedAt: loggedAt}
	}
	return nil
}

// stored は記録に無効化を付けて返す（ロックを取ってから呼ぶ）。
func (m *Memory) stored(r storedRecord) application.StoredRecord {
	out := application.StoredRecord{MatchRecord: r.MatchRecord, LoggedAt: r.loggedAt}
	if v, ok := m.voids[r.RecordID()]; ok {
		out.Void = &v
	}
	return out
}

// sortedRecords は条件に合う記録を played_at → 記録 ID の順に返す（ロックを取ってから呼ぶ）。
func (m *Memory) sortedRecords(keep func(storedRecord) bool) []storedRecord {
	var out []storedRecord
	for _, r := range m.records {
		if keep(r) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].playedAt.Equal(out[j].playedAt) {
			return out[i].playedAt.Before(out[j].playedAt)
		}
		return out[i].RecordID() < out[j].RecordID()
	})
	return out
}

func (m *Memory) Records(_ context.Context, from, to time.Time) ([]application.StoredRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	recs := m.sortedRecords(func(r storedRecord) bool {
		return !r.playedAt.Before(from) && (to.IsZero() || r.playedAt.Before(to))
	})
	out := make([]application.StoredRecord, len(recs))
	for i, r := range recs {
		out[i] = m.stored(r)
	}
	return out, nil
}

func (m *Memory) RecordsFor(_ context.Context, account shared.AccountID, limit int) ([]application.StoredRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	recs := m.sortedRecords(func(r storedRecord) bool {
		return r.Players[0].AccountID == account || r.Players[1].AccountID == account
	})
	slices.Reverse(recs)
	out := make([]application.StoredRecord, 0, min(limit, len(recs)))
	for _, r := range recs[:min(limit, len(recs))] {
		out = append(out, m.stored(r))
	}
	return out, nil
}

func (m *Memory) RecordByID(_ context.Context, recordID string) (application.StoredRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.records[recordID]
	if !ok {
		return application.StoredRecord{}, shared.ErrNotFound
	}
	return m.stored(r), nil
}

func (m *Memory) RecordByMatch(_ context.Context, key application.MatchKey) (application.StoredRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var found *storedRecord
	for _, r := range m.records {
		if r.HostInstance == key.Host && r.MatchID == key.MatchID && (found == nil || r.RecordID() < found.RecordID()) {
			found = &r
		}
	}
	if found == nil {
		return application.StoredRecord{}, shared.ErrNotFound
	}
	return m.stored(*found), nil
}

func (m *Memory) InsertVoid(_ context.Context, v application.Void) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 先に追記されたものを残す（Postgres の ON CONFLICT DO NOTHING と同じ）
	if _, ok := m.voids[v.RecordID]; !ok {
		m.voids[v.RecordID] = v
	}
	return nil
}

func (m *Memory) UpsertRemoteAccount(_ context.Context, a application.RemoteAccount, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[a.AccountID] = a
	return nil
}

func (m *Memory) RemoteAccount(_ context.Context, id shared.AccountID) (application.RemoteAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[id]
	if !ok {
		return application.RemoteAccount{AccountID: id}, shared.ErrNotFound
	}
	return a, nil
}

func (m *Memory) UpsertRemoteRequest(_ context.Context, r application.RemoteRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.requests[r.ID]; ok && old.Origin != r.Origin {
		return nil
	}
	m.requests[r.ID] = r
	return nil
}

func (m *Memory) DeleteRemoteRequest(_ context.Context, origin, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.requests[id]; ok && r.Origin == origin {
		delete(m.requests, id)
	}
	return nil
}

func (m *Memory) RemoteRequests(_ context.Context, now time.Time) ([]application.RemoteRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []application.RemoteRequest
	for id, r := range m.requests {
		if r.ExpiresAt.Before(now) {
			delete(m.requests, id)
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
