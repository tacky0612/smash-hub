//go:build integration

package integrationtest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// player は HTTP API を操作するテスト用のプレイヤー。
type player struct {
	t      *testing.T
	env    *env
	client *http.Client
	ID     string
}

// call は API を呼び出し、ステータスコードを確認して JSON を out に読み込む。
func (p *player) call(method, path string, body any, wantStatus int, out any) {
	p.t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			p.t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, p.env.server.URL+path, r)
	if err != nil {
		p.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Smash-Hub", "1")
	res, err := p.client.Do(req)
	if err != nil {
		p.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != wantStatus {
		p.t.Fatalf("%s %s: status %d, want %d: %s", method, path, res.StatusCode, wantStatus, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			p.t.Fatalf("%s %s: %v: %s", method, path, err, data)
		}
	}
}

// newPlayer は開発用ログインと登録を済ませたプレイヤーを返す。
func newPlayer(t *testing.T, e *env, name string) *player {
	t.Helper()
	p := &player{t: t, env: e, client: newClient(t)}
	var me struct {
		LoggedIn   bool `json:"loggedIn"`
		Registered bool `json:"registered"`
	}
	p.call("POST", "/api/auth/dev/login", map[string]string{"name": name}, http.StatusOK, &me)
	if !me.LoggedIn || me.Registered {
		t.Fatalf("ログイン直後の状態が正しくない: %+v", me)
	}
	var account struct {
		ID string `json:"id"`
	}
	p.call("POST", "/api/register", map[string]string{"displayName": name, "mainFighter": "mario"}, http.StatusCreated, &account)
	p.ID = account.ID
	return p
}

type statusRes struct {
	State   string `json:"state"`
	MatchID string `json:"matchId"`
}

func (p *player) search(rateRange int) statusRes {
	p.t.Helper()
	var st statusRes
	p.call("POST", "/api/matchmaking/search", map[string]any{"fighter": "mario", "rateRange": rateRange}, http.StatusOK, &st)
	return st
}

func (p *player) status() statusRes {
	p.t.Helper()
	var st statusRes
	p.call("GET", "/api/matchmaking/status", nil, http.StatusOK, &st)
	return st
}

type side struct {
	Player struct {
		ID     string `json:"id"`
		Rating int    `json:"rating"`
	} `json:"player"`
	Reported bool   `json:"reported"`
	Report   string `json:"report"`
}

type matchRes struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Mismatched   bool   `json:"mismatched"`
	Winner       string `json:"winner"`
	MyRole       string `json:"myRole"`
	RoomID       string `json:"roomId"`
	RoomPassword string `json:"roomPassword"`
	Player1      side   `json:"player1"`
	Player2      side   `json:"player2"`
}

func (p *player) match(id string) matchRes {
	p.t.Helper()
	var m matchRes
	p.call("GET", "/api/matches/"+id, nil, http.StatusOK, &m)
	return m
}

func (p *player) report(id, result string) matchRes {
	p.t.Helper()
	var m matchRes
	p.call("POST", "/api/matches/"+id+"/report", map[string]string{"result": result}, http.StatusOK, &m)
	return m
}

type standingRes struct {
	Rating  int `json:"rating"`
	Matches int `json:"matches"`
	Wins    int `json:"wins"`
	Losses  int `json:"losses"`
}

func (p *player) rating(id string) standingRes {
	p.t.Helper()
	var s standingRes
	p.call("GET", "/api/accounts/"+id+"/rating", nil, http.StatusOK, &s)
	return s
}

// matchPair は2人をマッチングさせ、対戦 ID を返す（a がプレイヤー1）。
func matchPair(t *testing.T, a, b *player) string {
	t.Helper()
	if st := a.search(0); st.State != "waiting" {
		t.Fatalf("1人目が待機にならない: %+v", st)
	}
	a.env.clock.Advance(1) // 先に待っていた側をプレイヤー1にするため
	st := b.search(0)
	if st.State != "matched" || st.MatchID == "" {
		t.Fatalf("2人目でマッチングしない: %+v", st)
	}
	if st2 := a.status(); st2.State != "matched" || st2.MatchID != st.MatchID {
		t.Fatalf("1人目の状態が matched にならない: %+v", st2)
	}
	return st.MatchID
}
