//go:build integration

package integrationtest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

type reportRes struct {
	ID         string `json:"id"`
	HostedHere bool   `json:"hostedHere"`
	MatchID    string `json:"matchId"`
	Category   string `json:"category"`
	Body       string `json:"body"`
	Status     string `json:"status"`
	Reporter   struct {
		ID string `json:"id"`
	} `json:"reporter"`
	Target struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	} `json:"target"`
}

type moderationRes struct {
	Admin      bool `json:"admin"`
	Suspension *struct {
		Kind string `json:"kind"`
		Note string `json:"note"`
	} `json:"suspension"`
	Sanctions []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Note string `json:"note"`
	} `json:"sanctions"`
}

func (p *player) moderation() moderationRes {
	p.t.Helper()
	var m moderationRes
	p.call("GET", "/api/me/moderation", nil, http.StatusOK, &m)
	return m
}

// confirmedMatch は2人を対戦させ、a の勝ちで確定させる。
func confirmedMatch(t *testing.T, a, b *player) string {
	t.Helper()
	id := matchPair(t, a, b)
	a.report(id, "win")
	if m := b.report(id, "lose"); m.Status != "confirmed" {
		t.Fatalf("確定しない: %+v", m)
	}
	return id
}

// 通報は運営者（管理者）だけが見られ、通報者にも相手にも内容は返らない。
func TestReport_OnlyAdminsCanSeeReports(t *testing.T) {
	e := newEnv(t)
	admin, taro, hanako := newPlayer(t, e, "admin"), newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := confirmedMatch(t, taro, hanako)

	if !admin.moderation().Admin || taro.moderation().Admin {
		t.Fatal("管理者の判定が正しくない")
	}
	hanako.call("POST", "/api/reports", map[string]any{
		"targetId": taro.ID, "matchHost": "http://instance.test", "matchId": id, "category": "collusion", "body": "わざと負けたように見えた",
	}, http.StatusNoContent, nil)

	// 管理者でなければ一覧は見られない
	taro.call("GET", "/api/admin/reports", nil, http.StatusForbidden, nil)
	hanako.call("GET", "/api/admin/reports", nil, http.StatusForbidden, nil)

	var list []reportRes
	admin.call("GET", "/api/admin/reports", nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].Reporter.ID != hanako.ID || list[0].Target.ID != taro.ID || !list[0].HostedHere || list[0].MatchID != id {
		t.Fatalf("通報の一覧 = %+v", list)
	}

	// 対応済みにすると未対応の一覧から消える
	admin.call("POST", "/api/admin/reports/"+list[0].ID+"/resolve", map[string]string{"note": "証拠不十分"}, http.StatusNoContent, nil)
	admin.call("GET", "/api/admin/reports", nil, http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("対応済みが残る: %+v", list)
	}
	admin.call("GET", "/api/admin/reports?status=resolved", nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].Status != "resolved" {
		t.Fatalf("対応済みの一覧 = %+v", list)
	}
}

func TestReport_Validation(t *testing.T) {
	e := newEnv(t)
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	body := func(target string) map[string]any {
		return map[string]any{"targetId": target, "category": "abuse", "body": "暴言を言われた"}
	}
	taro.call("POST", "/api/reports", body(taro.ID), http.StatusBadRequest, nil)
	taro.call("POST", "/api/reports", body("did:key:zUnknown"), http.StatusNotFound, nil)
	// 24時間に5件まで
	for range 5 {
		taro.call("POST", "/api/reports", body(hanako.ID), http.StatusNoContent, nil)
	}
	taro.call("POST", "/api/reports", body(hanako.ID), http.StatusBadRequest, nil)
	e.clock.Advance(24 * time.Hour)
	taro.call("POST", "/api/reports", body(hanako.ID), http.StatusNoContent, nil)
}

// 対応済みの通報は 30日で削除する。処分の記録は残す。
func TestReport_PurgedAfterRetention(t *testing.T) {
	e := newEnv(t)
	admin, taro, hanako := newPlayer(t, e, "admin"), newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	hanako.call("POST", "/api/reports", map[string]any{"targetId": taro.ID, "category": "abuse", "body": "暴言"}, http.StatusNoContent, nil)
	var list []reportRes
	admin.call("GET", "/api/admin/reports", nil, http.StatusOK, &list)
	admin.call("POST", "/api/admin/sanctions", map[string]any{"accountId": taro.ID, "kind": "yellow", "reason": "abuse"}, http.StatusCreated, nil)
	admin.call("POST", "/api/admin/reports/"+list[0].ID+"/resolve", map[string]string{"note": ""}, http.StatusNoContent, nil)

	ctx := context.Background()
	e.clock.Advance(29 * 24 * time.Hour)
	if n, err := e.app.Moderation.PurgeReports(ctx); err != nil || n != 0 {
		t.Fatalf("30日前に削除した: n=%d err=%v", n, err)
	}
	e.clock.Advance(2 * 24 * time.Hour)
	if n, err := e.app.Moderation.PurgeReports(ctx); err != nil || n != 1 {
		t.Fatalf("30日後に削除しない: n=%d err=%v", n, err)
	}
	// 30日を過ぎてセッションは切れているため、DB で確かめる
	var sanctions int
	if err := e.db.Pool.QueryRow(ctx, `SELECT count(*) FROM sanctions WHERE account_id = $1`, taro.ID).Scan(&sanctions); err != nil || sanctions != 1 {
		t.Errorf("処分の記録が残っていない: %d %v", sanctions, err)
	}
}

// レッドカード（利用停止）の間は対戦相手を探せない。待機中の募集は取り消し、解除すれば探せる。
func TestSanction_SuspensionBlocksMatchmaking(t *testing.T) {
	e := newEnv(t)
	admin, taro, hanako := newPlayer(t, e, "admin"), newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")

	if st := taro.search(0); st.State != "waiting" {
		t.Fatalf("待機にならない: %+v", st)
	}
	taro.call("POST", "/api/admin/sanctions", map[string]any{"accountId": hanako.ID, "kind": "red", "reason": "collusion"}, http.StatusForbidden, nil)
	var sc struct {
		ID     string  `json:"id"`
		EndsAt *string `json:"endsAt"`
		Active bool    `json:"active"`
	}
	admin.call("POST", "/api/admin/sanctions", map[string]any{
		"accountId": taro.ID, "kind": "red", "reason": "collusion", "note": "運営者だけのメモ", "days": 7,
	}, http.StatusCreated, &sc)
	if !sc.Active || sc.EndsAt == nil {
		t.Fatalf("処分 = %+v", sc)
	}

	if st := taro.status(); st.State != "idle" {
		t.Errorf("待機中の募集が取り消されない: %+v", st)
	}
	var errRes struct {
		Message string `json:"message"`
	}
	taro.call("POST", "/api/matchmaking/search", map[string]any{"fighter": "mario", "rateRange": 0}, http.StatusForbidden, &errRes)
	if errRes.Message == "" {
		t.Error("利用停止の理由が返らない")
	}
	// 本人には停止中であることを返すが、運営者のメモは返さない
	if m := taro.moderation(); m.Suspension == nil || m.Suspension.Note != "" || m.Sanctions[0].Note != "" {
		t.Errorf("本人に見せる状態 = %+v", m)
	}
	// 他の利用者は待機しても taro とはマッチングしない
	if st := hanako.search(0); st.State != "waiting" {
		t.Errorf("hanako: %+v", st)
	}

	admin.call("POST", "/api/admin/sanctions/"+sc.ID+"/lift", nil, http.StatusOK, nil)
	if st := taro.search(0); st.State != "matched" {
		t.Errorf("解除後に探せない: %+v", st)
	}
}

// 期間付きの利用停止は、期間が過ぎれば自動で終わる。
func TestSanction_SuspensionEnds(t *testing.T) {
	e := newEnv(t)
	admin, taro := newPlayer(t, e, "admin"), newPlayer(t, e, "taro")
	admin.call("POST", "/api/admin/sanctions", map[string]any{"accountId": taro.ID, "kind": "red", "reason": "abuse", "days": 3}, http.StatusCreated, nil)
	taro.call("POST", "/api/matchmaking/search", map[string]any{"fighter": "mario", "rateRange": 0}, http.StatusForbidden, nil)
	e.clock.Advance(3 * 24 * time.Hour)
	if st := taro.search(0); st.State != "waiting" {
		t.Errorf("期間後も探せない: %+v", st)
	}
}

// ホストが無効化した記録はレート計算から除外され、ログに MatchVoided が追記される。記録自体は残る。
func TestVoid_ExcludesRecordFromRating(t *testing.T) {
	e := newEnv(t)
	admin, taro, hanako := newPlayer(t, e, "admin"), newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := confirmedMatch(t, taro, hanako)
	if s := taro.rating(taro.ID); s.Rating == 1500 || s.Matches != 1 {
		t.Fatalf("確定でレートが変わらない: %+v", s)
	}

	taro.call("POST", "/api/admin/voids", map[string]string{"matchId": id, "reason": "collusion"}, http.StatusForbidden, nil)
	admin.call("POST", "/api/admin/voids", map[string]string{"matchId": id, "reason": "八百長"}, http.StatusBadRequest, nil)
	admin.call("POST", "/api/admin/voids", map[string]string{"matchId": id, "reason": "collusion"}, http.StatusNoContent, nil)
	admin.call("POST", "/api/admin/voids", map[string]string{"matchId": id, "reason": "collusion"}, http.StatusConflict, nil)

	for _, p := range []*player{taro, hanako} {
		if s := p.rating(p.ID); s.Rating != 1500 || s.Matches != 0 {
			t.Errorf("無効化後のレート: %+v", s)
		}
	}
	if m := taro.match(id); m.Status != "confirmed" {
		t.Errorf("対戦の状態 = %s", m.Status)
	}
	var history []struct {
		ID     string `json:"id"`
		Voided bool   `json:"voided"`
	}
	taro.call("GET", "/api/accounts/"+taro.ID+"/matches", nil, http.StatusOK, &history)
	if len(history) != 1 || !history[0].Voided {
		t.Errorf("戦績に無効化が表示されない: %+v", history)
	}

	var log struct {
		Events []domain.Event `json:"events"`
	}
	taro.call("GET", "/federation/log?since=0", nil, http.StatusOK, &log)
	last := log.Events[len(log.Events)-1]
	v, err := domain.DecodeMatchVoided(last)
	if last.Type != domain.TypeMatchVoided || err != nil || v.Reason != "collusion" {
		t.Fatalf("ログの最後 = %s %+v %v", last.Type, v, err)
	}
	var count int
	if err := e.db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM match_records WHERE record_id = $1`, v.RecordID).Scan(&count); err != nil || count != 1 {
		t.Errorf("記録が残っていない: %d %v", count, err)
	}
}

// 無効化できるのは対戦から 7日以内（確定レートを変えないため）。
func TestVoid_OnlyWithinDeadline(t *testing.T) {
	e := newEnv(t)
	admin, taro, hanako := newPlayer(t, e, "admin"), newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := confirmedMatch(t, taro, hanako)
	e.clock.Advance(domain.VoidDeadline + time.Minute)
	admin.call("POST", "/api/admin/voids", map[string]string{"matchId": id, "reason": "collusion"}, http.StatusBadRequest, nil)
	admin.call("POST", "/api/admin/voids", map[string]string{"matchId": "unknown", "reason": "collusion"}, http.StatusNotFound, nil)
}

// appendVoid は MatchVoided を偽の連合先のログに追記する。
func (p *fakePeer) appendVoid(t *testing.T, recordID, createdAt string) {
	t.Helper()
	prev := ""
	if n := len(p.events); n > 0 {
		prev = p.events[n-1].Hash()
	}
	ev, err := domain.NewEvent(domain.TypeMatchVoided, shared.InstanceID(p.server.URL), int64(len(p.events)+1), prev,
		createdAt, domain.MatchVoided{RecordID: recordID, Reason: domain.VoidCollusion}, func(m []byte) string { return crypto.Sign(p.priv, m) })
	if err != nil {
		t.Fatal(err)
	}
	p.events = append(p.events, ev)
}

func recordCount(t *testing.T, e *env) (records, effective int) {
	t.Helper()
	all, err := e.app.Federation.Store.Records(context.Background(), time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.app.Federation.ConfirmedResults(context.Background(), time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	return len(all), len(res)
}

// 連合先の無効化は、記録のホストが期限内に発行したものだけをレート計算に反映する。
// 期限を過ぎた無効化も、ログとしては取り込んで先へ進む（どのインスタンスでも同じ記録の集合になる）。
func TestSync_ImportsVoidFromHost(t *testing.T) {
	e := newEnv(t)
	peer := newFakePeer(t)
	e.app.Federation.Peers = []string{peer.server.URL}
	ctx := context.Background()
	picks := []shared.Pick{{Game: 1, Characters: []shared.FighterID{"mario", "fox"}}}

	voided := peer.appendRecord(t, "m1", picks)
	late := peer.appendRecord(t, "m2", picks)
	peer.appendVoid(t, voided.RecordID(), "2026-11-02T19:00:00.000Z") // played_at（11/01 19:00）から1日後
	peer.appendVoid(t, late.RecordID(), "2026-11-08T19:00:01.000Z")   // 7日を過ぎている

	if n, err := e.app.Federation.SyncPeer(ctx, peer.server.URL); n != 4 || err != nil {
		t.Fatalf("imported=%d err=%v", n, err)
	}
	if all, effective := recordCount(t, e); all != 2 || effective != 1 {
		t.Fatalf("記録 %d 件・レート計算に使う記録 %d 件, want 2・1", all, effective)
	}
	res, _ := e.app.Federation.ConfirmedResults(ctx, time.Time{}, time.Time{})
	if res[0].RecordID() != late.RecordID() {
		t.Errorf("除外する記録が違う")
	}
}

// ホスト以外のインスタンスは、記録を無効化できない（その無効化で取り込みを止める）。
func TestSync_RejectsVoidFromNonHost(t *testing.T) {
	e := newEnv(t)
	peer := newFakePeer(t)
	e.app.Federation.Peers = []string{peer.server.URL}
	ctx := context.Background()

	// 自インスタンスがホストした記録
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	confirmedMatch(t, taro, hanako)
	all, err := e.app.Federation.Store.Records(ctx, time.Time{}, time.Time{})
	if err != nil || len(all) != 1 {
		t.Fatal(all, err)
	}
	peer.appendVoid(t, all[0].RecordID(), "2026-11-01T21:00:00.000Z")
	if n, err := e.app.Federation.SyncPeer(ctx, peer.server.URL); n != 0 || err == nil {
		t.Fatalf("ホスト以外の無効化を取り込んだ: imported=%d err=%v", n, err)
	}
	if _, effective := recordCount(t, e); effective != 1 {
		t.Error("ホスト以外の無効化でレート計算から除外された")
	}

	// 存在しない記録の無効化も取り込まない
	peer2 := newFakePeer(t)
	e.app.Federation.Peers = []string{peer2.server.URL}
	peer2.appendVoid(t, "0000000000000000000000000000000000000000000000000000000000000000", "2026-11-01T21:00:00.000Z")
	if n, err := e.app.Federation.SyncPeer(ctx, peer2.server.URL); n != 0 || err == nil {
		t.Fatalf("存在しない記録の無効化を取り込んだ: imported=%d err=%v", n, err)
	}
}

// 登録には、現在の版の利用規約への同意が必要。
func TestRegister_RequiresCurrentTerms(t *testing.T) {
	e := newEnv(t)
	p := &player{t: t, env: e, client: newClient(t)}
	p.call("POST", "/api/auth/dev/login", map[string]string{"name": "taro"}, http.StatusOK, nil)
	var tos struct {
		Text    string `json:"text"`
		Version string `json:"version"`
		Custom  bool   `json:"custom"`
	}
	p.call("GET", "/api/terms", nil, http.StatusOK, &tos)
	if tos.Text == "" || tos.Custom {
		t.Fatalf("利用規約 = %+v", tos)
	}
	reg := func(version string, want int) {
		p.call("POST", "/api/register", map[string]string{"displayName": "taro", "mainFighter": "mario", "termsVersion": version}, want, nil)
	}
	reg("", http.StatusBadRequest)
	reg("0123456789abcdef", http.StatusBadRequest)
	reg(tos.Version, http.StatusCreated)

	var version string
	if err := e.db.Pool.QueryRow(context.Background(), `SELECT terms_version FROM accounts`).Scan(&version); err != nil || version != tos.Version {
		t.Errorf("同意した版が記録されない: %q %v", version, err)
	}
}

// 利用規約が更新されたら、登録済みの利用者にも同意し直してもらう。同意するまでは対戦相手を探せない。
func TestTerms_ReconsentAfterUpdate(t *testing.T) {
	e := newEnv(t)
	taro := newPlayer(t, e, "taro")
	accepted := func() bool {
		var me struct {
			TermsAccepted bool `json:"termsAccepted"`
		}
		taro.call("GET", "/api/me", nil, http.StatusOK, &me)
		return me.TermsAccepted
	}
	if !accepted() {
		t.Fatal("登録直後に同意済みにならない")
	}

	old := e.app.Accounts.TermsVersion
	e.app.Accounts.TermsVersion = "fedcba9876543210" // 利用規約を更新した
	if accepted() {
		t.Fatal("更新後も同意済みのまま")
	}
	taro.call("POST", "/api/matchmaking/search", map[string]any{"fighter": "mario", "rateRange": 0}, http.StatusForbidden, nil)
	taro.call("POST", "/api/me/terms", map[string]string{"termsVersion": old}, http.StatusBadRequest, nil)
	taro.call("POST", "/api/me/terms", map[string]string{"termsVersion": "fedcba9876543210"}, http.StatusNoContent, nil)
	if !accepted() {
		t.Fatal("同意しても同意済みにならない")
	}
	if st := taro.search(0); st.State != "waiting" {
		t.Errorf("同意後に探せない: %+v", st)
	}
}
