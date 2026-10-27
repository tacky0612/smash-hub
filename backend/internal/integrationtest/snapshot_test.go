//go:build integration

package integrationtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/tacky0612/smash-hub/backend/internal/bootstrap"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// snapshotEnv は、自インスタンスの対戦・無効化と、連合先から取り込んだ記録を持つ環境を作る。
func snapshotEnv(t *testing.T) (*env, *fakePeer, []*player) {
	t.Helper()
	e := newEnv(t)
	peer := newFakePeer(t)
	e.app.Federation.Peers = []string{peer.server.URL}
	admin, taro, hanako, jiro := newPlayer(t, e, "admin"), newPlayer(t, e, "taro"), newPlayer(t, e, "hanako"), newPlayer(t, e, "jiro")
	confirmedMatch(t, taro, hanako)
	confirmedMatch(t, hanako, jiro)
	voided := confirmedMatch(t, jiro, taro)
	admin.call("POST", "/api/admin/voids", map[string]string{"matchId": voided, "reason": "collusion"}, http.StatusNoContent, nil)

	picks := []shared.Pick{{Game: 1, Characters: []shared.FighterID{"mario", "fox"}}}
	peer.appendRecord(t, "p1", picks)
	peer.appendRecord(t, "p2", picks)
	if n, err := e.app.Federation.SyncPeer(context.Background(), peer.server.URL); n != 2 || err != nil {
		t.Fatalf("imported=%d err=%v", n, err)
	}
	return e, peer, []*player{taro, hanako, jiro}
}

func writeSnapshot(t *testing.T, e *env) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := e.app.WriteSnapshot(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// スナップショットから DB を使わずにログを取り込み直して再計算したレートは、インスタンスのレートと一致する。
// 自インスタンスのログと、連合先から取り込んだログの両方を含む。
func TestSnapshot_RecomputedRatingsMatchInstance(t *testing.T) {
	e, peer, players := snapshotEnv(t)
	snap := writeSnapshot(t, e)

	report, err := bootstrap.VerifySnapshot(context.Background(), bytes.NewReader(snap), map[string]string{
		peer.server.URL: peerPublicKey(t, e, peer),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Header.Chains) != 2 {
		t.Fatalf("ログの数 = %d, want 2（自インスタンスと連合先）", len(report.Header.Chains))
	}
	if report.Records != 5 || report.Effective != 4 {
		t.Errorf("記録 %d 件・レート計算に使う記録 %d 件, want 5・4（無効化した1件を除く）", report.Records, report.Effective)
	}
	if len(report.Mismatches) != 0 {
		t.Fatalf("食い違い: %v", report.Mismatches)
	}
	// 連合先の記録の4人と、自インスタンスの3人
	if len(report.Computed) != 7 {
		t.Fatalf("再計算した人数 = %d, want 7", len(report.Computed))
	}
	for _, p := range players {
		want := p.rating(p.ID)
		found := false
		for _, s := range report.Computed {
			if string(s.AccountID) == p.ID {
				found = true
				if s.Rating != want.Rating || s.Matches != want.Matches {
					t.Errorf("%s: 再計算 %+v, 画面のレート %+v", p.ID, s, want)
				}
			}
		}
		if !found {
			t.Errorf("%s が再計算の結果にない", p.ID)
		}
	}
}

func peerPublicKey(t *testing.T, e *env, peer *fakePeer) string {
	t.Helper()
	st, err := e.app.Federation.Store.Peer(context.Background(), peer.server.URL)
	if err != nil || st.PublicKey == "" {
		t.Fatal(st, err)
	}
	return st.PublicKey
}

// 本番のレート（ratings テーブル）がログから計算した値と違えば、食い違いとして報告する。
func TestSnapshot_ReportsMismatchWithStoredRatings(t *testing.T) {
	e, _, players := snapshotEnv(t)
	if _, err := e.db.Pool.Exec(context.Background(), `UPDATE ratings SET rating = rating + 1 WHERE account_id = $1`, players[0].ID); err != nil {
		t.Fatal(err)
	}
	report, err := bootstrap.VerifySnapshot(context.Background(), bytes.NewReader(writeSnapshot(t, e)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Mismatches) != 1 || !strings.Contains(report.Mismatches[0], players[0].ID) {
		t.Fatalf("食い違い = %v", report.Mismatches)
	}
}

// 改ざん・欠落・鍵の差し替えは検証で失敗する。
func TestSnapshot_DetectsTampering(t *testing.T) {
	e, peer, _ := snapshotEnv(t)
	snap := writeSnapshot(t, e)

	edit := func(f func(m map[string]any)) []byte {
		var m map[string]any
		if err := json.Unmarshal(snap, &m); err != nil {
			t.Fatal(err)
		}
		f(m)
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	events := func(m map[string]any) []any { return m["events"].([]any) }
	lastMatch := func(m map[string]any) map[string]any {
		evs := events(m)
		for i := len(evs) - 1; i >= 0; i-- {
			if ev := evs[i].(map[string]any); ev["type"] == "MatchRecorded" {
				return ev
			}
		}
		t.Fatal("MatchRecorded がない")
		return nil
	}

	tests := map[string]struct {
		snap []byte
		keys map[string]string
	}{
		"勝者の書き換え": {snap: edit(func(m map[string]any) {
			lastMatch(m)["payload"].(map[string]any)["winner"] = 1.0
		})},
		"末尾のイベントの削除": {snap: edit(func(m map[string]any) {
			m["events"] = events(m)[:len(events(m))-1]
		})},
		"途中のイベントの削除": {snap: edit(func(m map[string]any) {
			evs := events(m)
			m["events"] = append(evs[:1:1], evs[2:]...)
		})},
		"レートの書き換え": {snap: edit(func(m map[string]any) {
			r := m["header"].(map[string]any)["ratings"].([]any)[0].(map[string]any)
			r["rating"] = r["rating"].(float64) + 100
		})},
		"指定した鍵と違う": {snap: snap, keys: map[string]string{peer.server.URL: "AAAA"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := bootstrap.VerifySnapshot(context.Background(), bytes.NewReader(tt.snap), tt.keys)
			if !errors.Is(err, bootstrap.ErrSnapshotInvalid) {
				t.Fatalf("err = %v, want ErrSnapshotInvalid", err)
			}
			t.Log(err)
		})
	}
}
