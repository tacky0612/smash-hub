//go:build e2e

// Package e2e は、Docker Compose で立ち上げた2つのインスタンス（Instance1・Instance2）に対して、
// SmashHubプロトコルでの対戦の一連の流れを検証するテスト。
// 実行方法: make test-e2e（docs: backend/README.md）
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"
)

func instanceURL(env, def string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}

var (
	instance1 = instanceURL("INSTANCE1_URL", "http://localhost:8080")
	instance2 = instanceURL("INSTANCE2_URL", "http://localhost:8090")
)

// syncTimeout は連合先のログが取り込まれるまで待つ上限。
const syncTimeout = 30 * time.Second

// uniqueName はテストの実行ごとに異なる名前を返す（DB を使い回しても別の利用者になる）。
func uniqueName(base string) string {
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}

// player は1つのインスタンスにログインしたプレイヤー。
type player struct {
	t      *testing.T
	base   string
	client *http.Client
	ID     string
	Name   string
}

func (p *player) do(method, path string, body any) (int, []byte) {
	p.t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			p.t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, p.base+path, r)
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
	return res.StatusCode, data
}

func (p *player) call(method, path string, body any, want int, out any) {
	p.t.Helper()
	status, data := p.do(method, path, body)
	if status != want {
		p.t.Fatalf("[%s] %s %s: status %d, want %d: %s", p.Name, method, path, status, want, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			p.t.Fatalf("[%s] %s %s: %v: %s", p.Name, method, path, err, data)
		}
	}
}

// newPlayer は base のインスタンスに開発用ログインで登録したプレイヤーを返す。
func newPlayer(t *testing.T, base, name string) *player {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	p := &player{t: t, base: base, client: &http.Client{Jar: jar, Timeout: 10 * time.Second}, Name: uniqueName(name)}
	p.call("POST", "/api/auth/dev/login", map[string]string{"name": p.Name}, http.StatusOK, nil)
	var a struct {
		ID string `json:"id"`
	}
	display := p.Name
	if len([]rune(display)) > 20 {
		display = string([]rune(display)[:20])
	}
	p.call("POST", "/api/register", map[string]string{"displayName": display, "mainFighter": "mario", "termsVersion": p.termsVersion()}, http.StatusCreated, &a)
	p.ID = a.ID
	return p
}

// termsVersion は現在の利用規約の版を返す（登録時に同意する版）。
func (p *player) termsVersion() string {
	p.t.Helper()
	var tos struct {
		Version string `json:"version"`
	}
	p.call("GET", "/api/terms", nil, http.StatusOK, &tos)
	return tos.Version
}

// adminName は管理者にする開発用ログインの名前（compose.yaml の DEV_ADMIN_NAMES）。
const adminName = "e2e-admin"

// newAdmin は base のインスタンスの管理者でログインする。DB を使い回した場合は登録済みのアカウントを使う。
func newAdmin(t *testing.T, base string) *player {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	p := &player{t: t, base: base, client: &http.Client{Jar: jar, Timeout: 10 * time.Second}, Name: adminName}
	var me struct {
		Registered bool `json:"registered"`
		Account    struct {
			ID string `json:"id"`
		} `json:"account"`
	}
	p.call("POST", "/api/auth/dev/login", map[string]string{"name": adminName}, http.StatusOK, &me)
	if !me.Registered {
		p.call("POST", "/api/register", map[string]string{"displayName": "admin", "mainFighter": "mario", "termsVersion": p.termsVersion()}, http.StatusCreated, &me.Account)
	}
	p.ID = me.Account.ID
	return p
}

type status struct {
	State   string `json:"state"`
	MatchID string `json:"matchId"`
}

func (p *player) search(fighter string) status {
	p.t.Helper()
	var st status
	p.call("POST", "/api/matchmaking/search", map[string]any{"fighter": fighter, "rateRange": 0}, http.StatusOK, &st)
	return st
}

func (p *player) status() status {
	p.t.Helper()
	var st status
	p.call("GET", "/api/matchmaking/status", nil, http.StatusOK, &st)
	return st
}

type side struct {
	Player struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		Rating      int    `json:"rating"`
	} `json:"player"`
	Fighter  string `json:"fighter"`
	Reported bool   `json:"reported"`
	Report   string `json:"report"`
}

type match struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	Winner        string `json:"winner"`
	MyRole        string `json:"myRole"`
	RoomID        string `json:"roomId"`
	RoomPassword  string `json:"roomPassword"`
	Player1       side   `json:"player1"`
	Player2       side   `json:"player2"`
	RecordedPicks []pick `json:"recordedPicks"`
}

func (p *player) match(id string) match {
	p.t.Helper()
	var m match
	p.call("GET", "/api/matches/"+id, nil, http.StatusOK, &m)
	return m
}

// pick は1ゲームのファイター（プレイヤー1、プレイヤー2 の順）。
type pick struct {
	Game       int      `json:"game"`
	Characters []string `json:"characters"`
}

// games はゲームごとのファイターの組（プレイヤー1、プレイヤー2 の順）から picks を作る。
func games(gs ...[2]string) []pick {
	out := make([]pick, len(gs))
	for i, g := range gs {
		out[i] = pick{Game: i + 1, Characters: []string{g[0], g[1]}}
	}
	return out
}

// reportPicks はゲームごとのファイターを付けて結果を報告する。
func (p *player) reportPicks(id, result string, picks []pick) match {
	p.t.Helper()
	var m match
	p.call("POST", "/api/matches/"+id+"/report", map[string]any{"result": result, "picks": picks}, http.StatusOK, &m)
	return m
}

func (p *player) report(id, result string) match {
	p.t.Helper()
	var m match
	p.call("POST", "/api/matches/"+id+"/report", map[string]string{"result": result}, http.StatusOK, &m)
	return m
}

type standing struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Rating      int    `json:"rating"`
	Matches     int    `json:"matches"`
	Wins        int    `json:"wins"`
	Losses      int    `json:"losses"`
}

// standingOn は base のインスタンスから見た account の成績を返す。
func (p *player) standingOn(base, account string) standing {
	p.t.Helper()
	q := &player{t: p.t, base: base, client: &http.Client{Timeout: 10 * time.Second}, Name: "anonymous"}
	var s standing
	q.call("GET", "/api/accounts/"+account+"/rating", nil, http.StatusOK, &s)
	return s
}

// eventually は cond が true になるまで待つ。
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(syncTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("%s: %v 待っても条件を満たさない", what, syncTimeout)
}

// matchUntilBoth は a が先に待ち、b が後から探して、2人が同じ対戦でマッチするまで状態を確認し続ける
// （フロントエンドのポーリングと同じ）。a がプレイヤー1（ホスト側）になる。
func matchUntilBoth(t *testing.T, a, b *player) string {
	t.Helper()
	if st := a.search("mario"); st.State != "waiting" {
		t.Fatalf("先に探した %s が待機にならない: %+v", a.Name, st)
	}
	b.search("fox")
	var id string
	eventually(t, "マッチング", func() bool {
		sa, sb := a.status(), b.status()
		if sa.State == "matched" && sb.State == "matched" && sa.MatchID == sb.MatchID {
			id = sa.MatchID
			return true
		}
		return false
	})
	return id
}
