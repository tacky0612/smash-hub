// Package ratelimit は API への大量リクエストを抑えるレート制限（トークンバケット）を提供する。
// 制限の単位と上限は docs/security/README.md の 4.4 を参照。
package ratelimit

import (
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Rule は1つの制限。Burst 回までは連続して受け付け、その後は Per ごとに1回分ずつ回復する。
type Rule struct {
	Name  string
	Burst int
	Per   time.Duration
}

// 制限の種類。上限はリクエストの元（アカウントまたは IP アドレス）ごと。
var (
	// Auth はログイン（/api/auth/）。外部プロバイダーへの往復を含むため最も厳しくする。
	Auth = Rule{Name: "auth", Burst: 10, Per: 6 * time.Second}
	// Write は状態を変更する API（結果報告・募集・プロフィール変更など）。
	Write = Rule{Name: "write", Burst: 30, Per: time.Second}
	// Read は参照する API。画面のポーリング（数秒ごと）を妨げない上限にする。
	Read = Rule{Name: "read", Burst: 120, Per: 200 * time.Millisecond}
	// Federation はインスタンス間の API（ログの pull・対戦の中継）。連合先の IP アドレスごと。
	Federation = Rule{Name: "federation", Burst: 300, Per: 100 * time.Millisecond}
)

// Classify はリクエストに適用する制限と、制限の単位（キー）を返す。制限しない場合は ok = false。
// ログイン中の利用者はアカウントごと、それ以外は IP アドレスごとに数える。
// 同じ IP アドレスを複数の利用者が共有する環境（携帯回線など）でも、ログイン後は互いに影響しない。
func Classify(r *http.Request) (rule Rule, key string, ok bool) {
	path := r.URL.Path
	ip := ipKey(httpx.ClientIP(r.Context()))
	switch {
	case path == "/api/health":
		return Rule{}, "", false
	case strings.HasPrefix(path, "/federation/"):
		return Federation, ip, true
	case strings.HasPrefix(path, "/api/auth/"):
		return Auth, ip, true
	case strings.HasPrefix(path, "/api/"):
		rule = Write
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			rule = Read
		}
		if id, err := httpx.AccountID(r.Context()); err == nil {
			return rule, "account:" + string(id), true
		}
		return rule, ip, true
	}
	return Rule{}, "", false
}

// ipKey は IP アドレスを制限の単位にする。IPv6 は1契約に /64 が割り当てられることが多いため /64 でまとめる。
func ipKey(ip netip.Addr) string {
	if ip.Is6() {
		if p, err := ip.Prefix(64); err == nil {
			return "ip:" + p.String()
		}
	}
	return "ip:" + ip.String()
}

// Limiter はキーごとのトークンバケットをメモリ上に持つ。インスタンス（プロセス）ごとに数える。
type Limiter struct {
	clock shared.Clock

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	tokens float64
	at     time.Time
	full   time.Time // 満タンに戻る時刻。これを過ぎたバケットは捨ててよい
}

// New は Limiter を作る。
func New(clock shared.Clock) *Limiter {
	return &Limiter{clock: clock, buckets: map[string]*bucket{}}
}

// Allow は rule と key の組で1回分を消費できるかを返す。できない場合は次に受け付けるまでの時間も返す。
func (l *Limiter) Allow(rule Rule, key string) (bool, time.Duration) {
	now := l.clock.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)

	k := rule.Name + "|" + key
	b, ok := l.buckets[k]
	if !ok {
		b = &bucket{tokens: float64(rule.Burst), at: now}
		l.buckets[k] = b
	}
	if elapsed := now.Sub(b.at); elapsed > 0 {
		b.tokens = math.Min(float64(rule.Burst), b.tokens+float64(elapsed)/float64(rule.Per))
		b.at = now
	}
	if b.tokens < 1 {
		return false, time.Duration((1 - b.tokens) * float64(rule.Per))
	}
	b.tokens--
	b.full = now.Add(time.Duration((float64(rule.Burst) - b.tokens) * float64(rule.Per)))
	return true, 0
}

// sweep は満タンに戻ったバケットを捨て、メモリの使用量を抑える（1分に1回）。
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	for k, b := range l.buckets {
		if !now.Before(b.full) {
			delete(l.buckets, k)
		}
	}
}

// Middleware は制限を超えたリクエストに 429 Too Many Requests を返す。
// httpx.ClientIPFrom とセッションのミドルウェアの内側に置く（IP アドレスとアカウント ID を使うため）。
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rule, key, ok := Classify(r)
		if ok {
			if allowed, wait := l.Allow(rule, key); !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
				httpx.WriteJSON(w, http.StatusTooManyRequests, map[string]string{
					"error": "rate_limited", "message": "リクエストが多すぎます。しばらく待ってからやり直してください",
				})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
