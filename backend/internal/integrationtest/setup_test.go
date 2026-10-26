//go:build integration

// Package integrationtest は PostgreSQL と結合したテスト。
// 実行方法: make test-integration（docs: backend/README.md）
package integrationtest

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/bootstrap"
	"github.com/tacky0612/smash-hub/backend/internal/platform/config"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
)

var (
	dbOnce sync.Once
	testDB *database.DB
	dbErr  error
)

// openDB はテスト用 DB に接続してマイグレーションを適用する（プロセス内で1回）。
func openDB(t *testing.T) *database.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL が設定されていません（make test-integration で実行してください）")
	}
	dbOnce.Do(func() {
		ctx := context.Background()
		testDB, dbErr = database.Open(ctx, url)
		if dbErr == nil {
			dbErr = testDB.Migrate(ctx)
		}
	})
	if dbErr != nil {
		t.Fatal(dbErr)
	}
	return testDB
}

// resetDB はすべてのテーブルを空にする。
func resetDB(t *testing.T, db *database.DB) {
	t.Helper()
	_, err := db.Pool.Exec(context.Background(),
		`TRUNCATE ratings, match_requests, matches, oauth_states, sessions, external_identities, accounts,
		 federation_events, federation_peers, match_records, remote_accounts, remote_match_requests, remote_matches`)
	if err != nil {
		t.Fatal(err)
	}
}

// fakeClock はテストから進められる時計。
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type env struct {
	db     *database.DB
	app    *bootstrap.App
	server *httptest.Server
	clock  *fakeClock
}

func testConfig() config.Config {
	return config.Config{
		Env:               "test",
		PublicBaseURL:     "http://localhost",
		SourceCodeURL:     "https://example.com/source",
		CookieSecure:      false,
		IdentifierPepper:  []byte("integration-test-pepper-0123456789abcdef"),
		DevLoginEnabled:   true,
		SeasonID:          "TEST",
		SeasonStart:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		RankingMinMatches: 1,
		InstanceURL:       "http://instance.test",
		InstanceName:      "test",
		// テスト専用の固定値（32バイトのシードを base64 にしたもの）
		InstancePrivateKey: "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=",
		AccountKeySecret:   []byte("integration-test-account-key-secret-0123456789"),
	}
}

// newEnv は空の DB の上にアプリケーションと HTTP サーバーを立てる。
func newEnv(t *testing.T) *env {
	t.Helper()
	db := openDB(t)
	resetDB(t, db)
	clock := &fakeClock{now: time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC)}
	app, err := bootstrap.New(testConfig(), db, clock)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler)
	t.Cleanup(srv.Close)
	return &env{db: db, app: app, server: srv, clock: clock}
}

// newClient は Cookie を保持する HTTP クライアントを返す。
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
