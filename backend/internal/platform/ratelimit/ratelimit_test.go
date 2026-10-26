package ratelimit

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func TestAllowBurstThenRefill(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	l := New(clock)
	rule := Rule{Name: "test", Burst: 3, Per: time.Second}

	for i := range 3 {
		if ok, _ := l.Allow(rule, "a"); !ok {
			t.Fatalf("%d 回目が拒否された", i+1)
		}
	}
	ok, wait := l.Allow(rule, "a")
	if ok || wait != time.Second {
		t.Fatalf("上限を超えたら拒否し、1秒後を案内する: ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.Allow(rule, "b"); !ok {
		t.Fatal("別のキーは影響を受けない")
	}

	clock.now = clock.now.Add(time.Second)
	if ok, _ := l.Allow(rule, "a"); !ok {
		t.Fatal("1秒で1回分回復する")
	}
	if ok, _ := l.Allow(rule, "a"); ok {
		t.Fatal("回復したのは1回分だけ")
	}

	clock.now = clock.now.Add(time.Hour)
	for i := range 3 {
		if ok, _ := l.Allow(rule, "a"); !ok {
			t.Fatalf("回復は Burst までで止まる: %d 回目", i+1)
		}
	}
	if ok, _ := l.Allow(rule, "a"); ok {
		t.Fatal("Burst を超えて溜まらない")
	}
}

func TestSweepDropsFullBuckets(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	l := New(clock)
	rule := Rule{Name: "test", Burst: 2, Per: time.Second}
	l.Allow(rule, "a")
	clock.now = clock.now.Add(2 * time.Minute)
	l.Allow(rule, "b")
	if _, ok := l.buckets["test|a"]; ok {
		t.Fatal("満タンに戻ったバケットは捨てる")
	}
	if _, ok := l.buckets["test|b"]; !ok {
		t.Fatal("使用中のバケットは残す")
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		method, path, ip string
		account          string
		rule             string
		key              string
	}{
		{"GET", "/api/health", "192.0.2.1", "", "", ""},
		{"GET", "/federation/log", "192.0.2.1", "", "federation", "ip:192.0.2.1"},
		{"POST", "/api/auth/dev/login", "192.0.2.1", "", "auth", "ip:192.0.2.1"},
		{"GET", "/api/ranking", "192.0.2.1", "", "read", "ip:192.0.2.1"},
		{"POST", "/api/matchmaking/search", "192.0.2.1", "did:key:z1", "write", "account:did:key:z1"},
		{"GET", "/api/me", "2001:db8:1:2:3:4:5:6", "", "read", "ip:2001:db8:1:2::/64"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		req.RemoteAddr = net.JoinHostPort(c.ip, "1234")
		var got Rule
		var key string
		var ok bool
		h := httpx.ClientIPFrom("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c.account != "" {
				r = r.WithContext(httpx.WithAccountID(r.Context(), "did:key:z1"))
			}
			got, key, ok = Classify(r)
		}))
		h.ServeHTTP(httptest.NewRecorder(), req)
		if got.Name != c.rule || key != c.key || ok != (c.rule != "") {
			t.Errorf("%s %s: rule=%q key=%q ok=%v, want rule=%q key=%q", c.method, c.path, got.Name, key, ok, c.rule, c.key)
		}
	}
}

func TestMiddlewareReturns429(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	h := httpx.ClientIPFrom("X-Real-IP")(New(clock).Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	call := func(ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/auth/dev/login", nil)
		req.Header.Set("X-Real-IP", ip)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for range Auth.Burst {
		if rec := call("198.51.100.7"); rec.Code != http.StatusNoContent {
			t.Fatalf("上限までは通す: %d", rec.Code)
		}
	}
	rec := call("198.51.100.7")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "6" {
		t.Fatalf("上限を超えたら 429 と Retry-After: %d %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec := call("198.51.100.8"); rec.Code != http.StatusNoContent {
		t.Fatalf("プロキシが伝えた IP アドレスごとに数える: %d", rec.Code)
	}
}
