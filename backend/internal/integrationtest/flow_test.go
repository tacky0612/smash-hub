//go:build integration

package integrationtest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/platform/auth"
)

// ログイン → 対戦相手を探す → 部屋 ID の共有 → 両者の報告 → レート反映 → ランキング、の一連の流れ。
func TestMatchFlow_ConfirmUpdatesRatingAndRanking(t *testing.T) {
	e := newEnv(t)
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := matchPair(t, taro, hanako)

	m := taro.match(id)
	if m.MyRole != "player1" || m.RoomPassword != "1122" || m.Status != "in_progress" {
		t.Fatalf("対戦ルームの状態が正しくない: %+v", m)
	}
	// 部屋 ID を設定できるのはプレイヤー1だけ
	hanako.call("PUT", "/api/matches/"+id+"/room", map[string]string{"roomId": "AB12C"}, http.StatusForbidden, nil)
	taro.call("PUT", "/api/matches/"+id+"/room", map[string]string{"roomId": "AB12C"}, http.StatusOK, nil)
	if got := hanako.match(id).RoomID; got != "AB12C" {
		t.Fatalf("プレイヤー2に部屋 ID が見えない: %q", got)
	}

	// 片方の報告では確定しない。相手の報告内容はそろうまで見えない
	taro.report(id, "win")
	if m := hanako.match(id); m.Status != "in_progress" || !m.Player1.Reported || m.Player1.Report != "" {
		t.Fatalf("片方の報告後の状態が正しくない: %+v", m)
	}
	m = hanako.report(id, "lose")
	if m.Status != "confirmed" || m.Winner != taro.ID {
		t.Fatalf("報告が一致しても確定しない: %+v", m)
	}

	if r := taro.rating(taro.ID); r.Rating != 1516 || r.Wins != 1 || r.Matches != 1 {
		t.Errorf("勝者のレート = %+v", r)
	}
	if r := taro.rating(hanako.ID); r.Rating != 1484 || r.Losses != 1 {
		t.Errorf("敗者のレート = %+v", r)
	}

	var ranking struct {
		Entries []struct {
			Rank        int    `json:"rank"`
			DisplayName string `json:"displayName"`
			Rating      int    `json:"rating"`
		} `json:"entries"`
	}
	taro.call("GET", "/api/ranking", nil, http.StatusOK, &ranking)
	if len(ranking.Entries) != 2 || ranking.Entries[0].DisplayName != "taro" || ranking.Entries[0].Rating != 1516 {
		t.Errorf("ランキング = %+v", ranking.Entries)
	}

	// 確定後はマッチング待ちでも対戦中でもない
	if st := taro.status(); st.State != "idle" {
		t.Errorf("確定後の状態 = %+v", st)
	}
}

func TestMatchFlow_MismatchThenReReport(t *testing.T) {
	e := newEnv(t)
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := matchPair(t, taro, hanako)

	taro.report(id, "win")
	m := hanako.report(id, "win")
	if m.Status != "in_progress" || !m.Mismatched || m.Player1.Report != "win" {
		t.Fatalf("食い違いの状態が正しくない: %+v", m)
	}
	if r := taro.rating(taro.ID); r.Matches != 0 {
		t.Fatalf("食い違いでレートが動いた: %+v", r)
	}
	if m := hanako.report(id, "lose"); m.Status != "confirmed" || m.Winner != taro.ID {
		t.Fatalf("報告し直しで確定しない: %+v", m)
	}
}

func TestMatchFlow_CancelByBoth(t *testing.T) {
	e := newEnv(t)
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := matchPair(t, taro, hanako)
	taro.report(id, "cancel")
	if m := hanako.report(id, "cancel"); m.Status != "cancelled" {
		t.Fatalf("中止にならない: %+v", m)
	}
	if r := taro.rating(taro.ID); r.Rating != 1500 || r.Matches != 0 {
		t.Errorf("中止でレートが動いた: %+v", r)
	}
}

func TestMatchFlow_ExpiresAfter24Hours(t *testing.T) {
	e := newEnv(t)
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := matchPair(t, taro, hanako)
	taro.report(id, "win")

	e.clock.Advance(24 * time.Hour)
	n, err := e.app.Matches.ExpireDue(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("ExpireDue = %d, %v", n, err)
	}
	if m := taro.match(id); m.Status != "expired" {
		t.Fatalf("期限切れにならない: %+v", m)
	}
	hanako.call("POST", "/api/matches/"+id+"/report", map[string]string{"result": "lose"}, http.StatusBadRequest, nil)
	if r := taro.rating(taro.ID); r.Matches != 0 {
		t.Errorf("期限切れでレートが動いた: %+v", r)
	}
}

func TestMatchmaking_RateRangeAndExpansion(t *testing.T) {
	e := newEnv(t)
	strong, weak := newPlayer(t, e, "strong"), newPlayer(t, e, "weak")
	// strong のレートを weak より 200 を超えて高くするため、別の相手に勝ち続けさせる
	sr := 1500
	for i := 0; sr-1500 <= 200; i++ {
		if i >= 40 {
			t.Fatal("レートが上がらない")
		}
		other := newPlayer(t, e, fmt.Sprintf("sparring%d", i))
		id := matchPair(t, strong, other)
		strong.report(id, "win")
		other.report(id, "lose")
		e.clock.Advance(time.Minute)
		sr = strong.rating(strong.ID).Rating
	}
	if sr-1500 <= 200 || sr-1500 > 300 {
		t.Fatalf("前提のレート差が 201〜300 になっていない: %d", sr)
	}

	// レート差が 200 を超えるので、すぐにはマッチしない
	if st := strong.search(0); st.State != "waiting" {
		t.Fatalf("strong: %+v", st)
	}
	if st := weak.search(0); st.State != "waiting" {
		t.Fatalf("レート差 %d でマッチした: %+v", sr-1500, st)
	}
	// 両者が状態を確認し続ける（フロントエンドのポーリング）。
	// 1分待つと「制限なし」のレート差が 300 に広がり、マッチする
	for i := 1; i <= 3; i++ {
		e.clock.Advance(20 * time.Second)
		s1, s2 := strong.status(), weak.status()
		matched := s1.State == "matched" || s2.State == "matched"
		if i < 3 && matched {
			t.Fatalf("%d秒後にマッチした", i*20)
		}
		if i == 3 && !matched {
			t.Fatalf("1分後にマッチしない: %+v %+v", s1, s2)
		}
	}
}

func TestMatchmaking_ConcurrentSearchNeverDoubleMatches(t *testing.T) {
	e := newEnv(t)
	const n = 10
	players := make([]*player, n)
	for i := range n {
		players[i] = newPlayer(t, e, fmt.Sprintf("p%d", i))
	}
	var wg sync.WaitGroup
	for _, p := range players {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.search(0)
		}()
	}
	wg.Wait()
	// 同時に探した場合は待機のままになることがあるので、状態の確認でマッチングを進める
	for range 3 {
		for _, p := range players {
			p.status()
		}
	}
	seen := map[string]int{}
	for _, p := range players {
		st := p.status()
		if st.State == "matched" {
			seen[st.MatchID]++
		}
	}
	var count int
	if err := e.db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM matches`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != n/2 {
		t.Errorf("対戦数 = %d, want %d", count, n/2)
	}
	for id, c := range seen {
		if c != 2 {
			t.Errorf("対戦 %s の参加者が %d 人", id, c)
		}
	}
}

func TestSecurity_CSRFHeaderRequired(t *testing.T) {
	e := newEnv(t)
	res, err := http.Post(e.server.URL+"/api/auth/dev/login", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("CSRF ヘッダーなしの POST が %d", res.StatusCode)
	}
}

func TestSecurity_UnregisteredSessionCannotSearch(t *testing.T) {
	e := newEnv(t)
	p := &player{t: t, env: e, client: newClient(t)}
	p.call("POST", "/api/auth/dev/login", map[string]string{"name": "guest"}, http.StatusOK, nil)
	p.call("POST", "/api/matchmaking/search", map[string]any{"fighter": "mario", "rateRange": 0}, http.StatusUnauthorized, nil)
}

// Discord ログイン（偽の Discord サーバー）で、利用者 ID を識別子ハッシュとしてのみ保存することを確認する。
func TestDiscordLogin_StoresOnlyIdentifierHash(t *testing.T) {
	e := newEnv(t)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil || r.Form.Get("code") != "good-code" || r.Form.Get("code_verifier") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, `{"access_token":"at","token_type":"Bearer"}`)
		case "/users/@me":
			if r.Header.Get("Authorization") != "Bearer at" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"id":"80351110224678912","username":"secret_name","email":"x@example.com"}`)
		}
	}))
	t.Cleanup(fake.Close)
	d := auth.NewDiscord("cid", "csecret", e.server.URL+"/api/auth/discord/callback")
	d.AuthorizeURL, d.TokenURL, d.UserURL = fake.URL+"/authorize", fake.URL+"/token", fake.URL+"/users/@me"
	e.app.Accounts.Discord = d

	p := &player{t: t, env: e, client: newClient(t)}
	res, err := p.client.Get(e.server.URL + "/api/auth/discord/start")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	loc, err := res.Location()
	if err != nil {
		t.Fatal(err)
	}
	if loc.Query().Get("scope") != "identify" || loc.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("認可 URL が正しくない: %s", loc)
	}
	state := loc.Query().Get("state")

	// state が違うコールバックはログインさせない
	res, _ = p.client.Get(e.server.URL + "/api/auth/discord/callback?state=wrong&code=good-code")
	res.Body.Close()
	if l, _ := res.Location(); l == nil || l.Query().Get("error") != "discord" {
		t.Fatalf("不正な state を受け付けた: %v", l)
	}

	res, err = p.client.Get(e.server.URL + "/api/auth/discord/callback?state=" + state + "&code=good-code")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	var me struct {
		LoggedIn bool `json:"loggedIn"`
	}
	p.call("GET", "/api/me", nil, http.StatusOK, &me)
	if !me.LoggedIn {
		t.Fatal("Discord ログインできない")
	}
	p.call("POST", "/api/register", map[string]string{"displayName": "discord user", "mainFighter": "fox", "termsVersion": p.termsVersion()}, http.StatusCreated, nil)

	// DB に Discord の利用者 ID・ユーザー名・メールアドレスが残っていないこと
	var leaked int
	err = e.db.Pool.QueryRow(context.Background(), `
		SELECT count(*) FROM (
			SELECT identifier_hash AS v FROM external_identities
			UNION ALL SELECT identifier_hash FROM sessions
			UNION ALL SELECT display_name FROM accounts
		) t WHERE v LIKE '%80351110224678912%' OR v LIKE '%secret_name%' OR v LIKE '%example.com%'`).Scan(&leaked)
	if err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Errorf("外部プロバイダーの情報が保存されている: %d 件", leaked)
	}
	// state は使い捨て
	var states int
	_ = e.db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM oauth_states`).Scan(&states)
	if states != 0 {
		t.Errorf("使用済みの state が残っている: %d", states)
	}
}
