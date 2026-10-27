//go:build integration

package integrationtest

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/application"
	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// recordedPicks は自インスタンスのログから対戦 id の MatchRecorded を取り出し、署名を検証して picks を返す。
func recordedPicks(t *testing.T, p *player, id string) []shared.Pick {
	t.Helper()
	var log struct {
		Events []json.RawMessage `json:"events"`
	}
	p.call("GET", "/federation/log?since=0", nil, http.StatusOK, &log)
	for _, raw := range log.Events {
		var ev domain.Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != domain.TypeMatchRecorded {
			continue
		}
		r, err := domain.DecodeMatchRecorded(ev, crypto.VerifyDID)
		if err != nil {
			t.Fatal(err)
		}
		if string(r.MatchID) == id {
			return r.Picks
		}
	}
	t.Fatalf("対戦 %s の記録がない", id)
	return nil
}

// 両者のゲームごとのファイターが一致すれば、それを含めた両者署名付きの記録になる。
func TestReport_AgreedPicksAreRecorded(t *testing.T) {
	e := newEnv(t)
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := matchPair(t, taro, hanako) // 1戦目は両者 mario
	g := games([2]string{"mario", "mario"}, [2]string{"mario", "fox"}, [2]string{"link", "fox"})

	taro.reportPicks(id, "win", g)
	m := hanako.reportPicks(id, "lose", g)
	if m.Status != "confirmed" || len(m.RecordedPicks) != 3 {
		t.Fatalf("一致したファイターで確定する: %+v", m)
	}
	if picks := recordedPicks(t, taro, id); len(picks) != 3 || picks[2].Characters[0] != "link" {
		t.Fatalf("記録の picks = %+v", picks)
	}
}

// ファイターの入力が食い違っても、勝敗が一致すれば確定し、1戦目だけを記録する。
func TestReport_MismatchedPicksFallBackToFirstGame(t *testing.T) {
	e := newEnv(t)
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := matchPair(t, taro, hanako)

	taro.reportPicks(id, "win", games([2]string{"mario", "mario"}, [2]string{"mario", "fox"}))
	if m := hanako.report(id, "lose"); m.Status != "confirmed" || len(m.RecordedPicks) != 1 {
		t.Fatalf("勝敗が一致すれば確定する: %+v", m)
	}
	if picks := recordedPicks(t, taro, id); len(picks) != 1 || picks[0].Characters[0] != "mario" {
		t.Fatalf("記録の picks = %+v", picks)
	}
}

func TestReport_RejectsPicksAgainstRules(t *testing.T) {
	e := newEnv(t)
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := matchPair(t, taro, hanako)

	// 両者が同時にファイターを変えることはできない
	bad := games([2]string{"mario", "mario"}, [2]string{"fox", "link"})
	taro.call("POST", "/api/matches/"+id+"/report", map[string]any{"result": "win", "picks": bad}, http.StatusBadRequest, nil)
	if m := taro.match(id); m.Player1.Reported {
		t.Fatal("拒否した報告が記録された")
	}
}

// 2-0 の報告で、1戦目の勝者（= 対戦の勝者）が2戦目にファイターを変えた内容は受け付けない。
// 報告する側がどちらでも（勝者・敗者のどちらの視点の報告でも）同じように拒否する。
func TestReport_RejectsWinnerChangingFighter(t *testing.T) {
	e := newEnv(t)
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := matchPair(t, taro, hanako)
	m := taro.match(id)
	// taro の勝ちで、taro 側のファイターを2戦目に変える
	taroSide := 0
	if m.MyRole == "player2" {
		taroSide = 1
	}
	second := [2]string{"mario", "mario"}
	second[taroSide] = "link"
	bad := games([2]string{"mario", "mario"}, second)

	taro.call("POST", "/api/matches/"+id+"/report", map[string]any{"result": "win", "picks": bad}, http.StatusBadRequest, nil)
	hanako.call("POST", "/api/matches/"+id+"/report", map[string]any{"result": "lose", "picks": bad}, http.StatusBadRequest, nil)

	// 敗者（hanako）側の変更なら受け付ける
	ok := [2]string{"mario", "mario"}
	ok[1-taroSide] = "fox"
	taro.reportPicks(id, "win", games([2]string{"mario", "mario"}, ok))
	if m := hanako.reportPicks(id, "lose", games([2]string{"mario", "mario"}, ok)); m.Status != "confirmed" || len(m.RecordedPicks) != 2 {
		t.Fatalf("敗者の変更は受け付けて記録する: %+v", m)
	}
}

// fakePeer は、署名付きのログを返す偽の連合先。
type fakePeer struct {
	server *httptest.Server
	priv   ed25519.PrivateKey
	events []domain.Event
}

func newFakePeer(t *testing.T) *fakePeer {
	t.Helper()
	pub, priv := crypto.GenerateKey()
	p := &fakePeer{priv: priv}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/federation/instance":
			_ = json.NewEncoder(w).Encode(application.InstanceInfo{InstanceID: p.server.URL, Name: "fake", PublicKey: base64.StdEncoding.EncodeToString(pub)})
		case "/federation/log":
			_ = json.NewEncoder(w).Encode(map[string]any{"instance_id": p.server.URL, "events": p.events})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(p.server.Close)
	return p
}

// appendRecord は両プレイヤーの署名付きの MatchRecorded をログに追記する。
func (p *fakePeer) appendRecord(t *testing.T, matchID string, picks []shared.Pick) shared.MatchRecord {
	t.Helper()
	_, a := crypto.GenerateKey()
	_, b := crypto.GenerateKey()
	didA := crypto.DIDKey(a.Public().(ed25519.PublicKey))
	didB := crypto.DIDKey(b.Public().(ed25519.PublicKey))
	st := shared.MatchStatement{
		MatchID: shared.MatchID(matchID), HostInstance: shared.InstanceID(p.server.URL), PlayedAt: "2026-11-01T19:00:00.000Z",
		Ruleset: shared.RulesetStandard, Players: []shared.PlayerRef{{AccountID: shared.AccountID(didA)}, {AccountID: shared.AccountID(didB)}},
		Picks: picks, Winner: 0,
	}
	rec := shared.MatchRecord{MatchStatement: st, Signatures: []shared.PlayerSignature{
		{AccountID: shared.AccountID(didA), Sig: crypto.Sign(a, st.Bytes())},
		{AccountID: shared.AccountID(didB), Sig: crypto.Sign(b, st.Bytes())},
	}}
	prev := ""
	if n := len(p.events); n > 0 {
		prev = p.events[n-1].Hash()
	}
	ev, err := domain.NewEvent(domain.TypeMatchRecorded, shared.InstanceID(p.server.URL), int64(len(p.events)+1), prev,
		"2026-11-01T19:10:00.000Z", rec, func(m []byte) string { return crypto.Sign(p.priv, m) })
	if err != nil {
		t.Fatal(err)
	}
	p.events = append(p.events, ev)
	return rec
}

// 連合先の記録は、picks の形だけを検証して取り込む。自インスタンスが知らないファイター ID
// （新しいバージョンで追加されたもの）でも取り込み、形が正しくない記録はそこで取り込みを止める。
func TestSync_ImportsPicksWithUnknownFighterAndStopsAtMalformedPicks(t *testing.T) {
	e := newEnv(t)
	peer := newFakePeer(t)
	e.app.Federation.Peers = []string{peer.server.URL}
	ctx := context.Background()

	unknown := peer.appendRecord(t, "m1", []shared.Pick{
		{Game: 1, Characters: []shared.FighterID{"mario", "fox"}},
		{Game: 2, Characters: []shared.FighterID{"mario", "new_fighter"}},
	})
	peer.appendRecord(t, "m2", []shared.Pick{
		{Game: 1, Characters: []shared.FighterID{"mario", "fox"}},
		{Game: 2, Characters: []shared.FighterID{"mario", shared.FighterRandom}}, // 2ゲーム以上の記録に random は入らない
	})

	n, err := e.app.Federation.SyncPeer(ctx, peer.server.URL)
	if n != 1 || err == nil {
		t.Fatalf("1件目だけ取り込み、2件目で止まる: imported=%d err=%v", n, err)
	}
	var ids []string
	rows, err := e.db.Pool.Query(ctx, `SELECT record_id FROM match_records ORDER BY record_id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if len(ids) != 1 || ids[0] != unknown.RecordID() {
		t.Fatalf("取り込んだ記録 = %v, want [%s]", ids, unknown.RecordID())
	}
}
