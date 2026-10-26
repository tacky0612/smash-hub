//go:build integration

package integrationtest

import (
	"context"
	"testing"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/adapter"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

func testRecord(host shared.InstanceID, matchID shared.MatchID) shared.MatchRecord {
	return shared.MatchRecord{
		MatchStatement: shared.MatchStatement{
			MatchID: matchID, HostInstance: host, PlayedAt: "2026-10-26T10:30:00.000Z", Ruleset: shared.RulesetStandard,
			Players: []shared.PlayerRef{{AccountID: "did:key:zA"}, {AccountID: "did:key:zB"}},
			Picks:   []shared.Pick{{Game: 1, Characters: []shared.FighterID{"mario", "fox"}}},
		},
		Signatures: []shared.PlayerSignature{{AccountID: "did:key:zA", Sig: "c2lnQQ=="}, {AccountID: "did:key:zB", Sig: "c2lnQg=="}},
	}
}

// 記録は記録 ID で識別する。別のホストが同じ match_id を使っても両方の記録が残り、
// 同じ記録を2回取り込んでも1件になる（取り込む記録の集合が到着順に依存しない）。
func TestMatchRecords_IdentifiedByRecordID(t *testing.T) {
	e := newEnv(t)
	store := adapter.Postgres{DB: e.db}
	ctx := context.Background()
	now := time.Date(2026, 10, 26, 11, 0, 0, 0, time.UTC)
	a, b := testRecord("https://a.example", "m1"), testRecord("https://b.example", "m1")
	for _, r := range []shared.MatchRecord{a, b, a} {
		if err := store.InsertRecord(ctx, r, now); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := store.Records(ctx, now.Add(-24*time.Hour), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("記録の件数 = %d, want 2", len(recs))
	}
	// 同時刻の記録は記録 ID の昇順
	if recs[0].RecordID() > recs[1].RecordID() {
		t.Errorf("記録 ID の昇順になっていない: %s, %s", recs[0].RecordID(), recs[1].RecordID())
	}
}

// マイグレーション 0003 で既存の記録に記録 ID を埋める式が、Go の RecordID と一致する。
func TestMigration0003_BackfillMatchesRecordID(t *testing.T) {
	e := newEnv(t)
	r := testRecord("https://a.example", "m1")
	body, err := shared.Canonical(r)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	// 0003_record_id.sql と同じ式
	err = e.db.Pool.QueryRow(context.Background(),
		`SELECT encode(sha256(convert_to(regexp_replace($1, ',"signatures":\[[^\]]*\]', ''), 'UTF8')), 'hex')`,
		string(body)).Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if got != r.RecordID() {
		t.Errorf("SQL の記録 ID = %s, want %s", got, r.RecordID())
	}
}
