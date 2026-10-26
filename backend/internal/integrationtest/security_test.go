//go:build integration

package integrationtest

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// rawCall は X-Real-IP を付けて Cookie なしでリクエストし、レスポンスを返す（本文は読み捨てる）。
func rawCall(t *testing.T, e *env, method, path, ip string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, e.server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Smash-Hub", "1")
	req.Header.Set("X-Real-IP", ip)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res
}

func TestAccessLogIsRecordedAndPurged(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	rawCall(t, e, "GET", "/api/ranking?season=secret-query", "203.0.113.5")
	rawCall(t, e, "GET", "/api/health", "203.0.113.5")
	e.app.AccessLog.Flush(ctx)

	var n int
	var ip, path string
	var status int
	if err := e.db.Pool.QueryRow(ctx, `SELECT count(*) FROM access_logs`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("ヘルスチェックを除いて1件記録する: %d 件", n)
	}
	if err := e.db.Pool.QueryRow(ctx, `SELECT ip, path, status FROM access_logs`).Scan(&ip, &path, &status); err != nil {
		t.Fatal(err)
	}
	if ip != "203.0.113.5" || path != "/api/ranking" || status != http.StatusOK {
		t.Fatalf("プロキシが伝えた IP アドレスとパス（クエリ文字列なし）を記録する: ip=%s path=%s status=%d", ip, path, status)
	}

	// 保持期間（14日）以内は残す
	e.clock.Advance(13 * 24 * time.Hour)
	if deleted, err := e.app.PurgeAccessLogs(ctx); err != nil || deleted != 0 {
		t.Fatalf("保持期間内は削除しない: deleted=%d err=%v", deleted, err)
	}
	// 保持期間を過ぎたら削除する
	e.clock.Advance(2 * 24 * time.Hour)
	if deleted, err := e.app.PurgeAccessLogs(ctx); err != nil || deleted != 1 {
		t.Fatalf("保持期間を過ぎたら削除する: deleted=%d err=%v", deleted, err)
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	got429 := false
	for range 11 {
		if res := rawCall(t, e, "GET", "/api/auth/discord/start", "198.51.100.20"); res.StatusCode == http.StatusTooManyRequests {
			got429 = true
			if res.Header.Get("Retry-After") == "" {
				t.Fatal("Retry-After を返す")
			}
		}
	}
	if !got429 {
		t.Fatal("同じ IP アドレスからのログインの連続は 429 で拒否する")
	}
	if res := rawCall(t, e, "GET", "/api/auth/discord/start", "198.51.100.21"); res.StatusCode == http.StatusTooManyRequests {
		t.Fatal("別の IP アドレスは影響を受けない")
	}

	// 拒否したリクエストもアクセスログに残す
	e.app.AccessLog.Flush(ctx)
	var n int
	if err := e.db.Pool.QueryRow(ctx, `SELECT count(*) FROM access_logs WHERE status = 429`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("429 のアクセスログ: %d 件", n)
	}

	// 時間が経てば回復する
	e.clock.Advance(time.Minute)
	if res := rawCall(t, e, "GET", "/api/auth/discord/start", "198.51.100.20"); res.StatusCode == http.StatusTooManyRequests {
		t.Fatal("時間が経てば回復する")
	}
}
