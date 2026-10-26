// Package accesslog はアクセスログを DB に記録し、保持期間を過ぎたものを削除する。
// アクセスログには IP アドレスが含まれるため、不正対策に必要な期間（初期値 14日）だけ保持する
// （docs/security/data-protection.md）。連合では共有しない。
package accesslog

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Entry は1件のアクセスログ。クエリ文字列（OAuth の認可コードなど）・Cookie・ヘッダーは記録しない。
type Entry struct {
	At       time.Time
	IP       string
	Method   string
	Path     string
	Status   int
	Duration time.Duration
}

const (
	queueSize = 4096
	maxPath   = 512
	maxMethod = 16
)

// Recorder はアクセスログを溜めておき、まとめて DB に書き込む。
// リクエストの処理を DB への書き込みで待たせないため、溜められる件数を超えた分は捨てる。
type Recorder struct {
	db      *database.DB
	clock   shared.Clock
	queue   chan Entry
	dropped atomic.Int64
}

// NewRecorder は Recorder を作る。
func NewRecorder(db *database.DB, clock shared.Clock) *Recorder {
	return &Recorder{db: db, clock: clock, queue: make(chan Entry, queueSize)}
}

// Record はアクセスログを書き込み待ちに加える。待ちがいっぱいなら捨てる。
func (r *Recorder) Record(e Entry) {
	select {
	case r.queue <- e:
	default:
		r.dropped.Add(1)
	}
}

// Run は interval ごとに書き込み待ちを DB に書き込む。ctx が終わるまで戻らない。
// 停止時に残ったものは、HTTP サーバーを止めた後に Flush で書き込む。
func (r *Recorder) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Flush(ctx)
		}
	}
}

// Flush は書き込み待ちのアクセスログをすべて DB に書き込む。
func (r *Recorder) Flush(ctx context.Context) {
	var rows [][]any
drain:
	for len(rows) < queueSize {
		select {
		case e := <-r.queue:
			rows = append(rows, []any{e.At, e.IP, e.Method, e.Path, e.Status, e.Duration.Milliseconds()})
		default:
			break drain
		}
	}
	if n := r.dropped.Swap(0); n > 0 {
		slog.Warn("access logs dropped", "count", n)
	}
	if len(rows) == 0 {
		return
	}
	_, err := r.db.Pool.CopyFrom(ctx, pgx.Identifier{"access_logs"},
		[]string{"at", "ip", "method", "path", "status", "duration_ms"}, pgx.CopyFromRows(rows))
	if err != nil {
		slog.Error("write access logs", "err", err, "count", len(rows))
	}
}

// Purge は before より前のアクセスログを削除し、削除した件数を返す。
func (r *Recorder) Purge(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM access_logs WHERE at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Middleware はリクエストごとにアクセスログを記録する。httpx.ClientIPFrom の内側に置く。
// ヘルスチェック（/api/health）は数秒ごとに呼ばれるため記録しない。
func (r *Recorder) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/health" {
			next.ServeHTTP(w, req)
			return
		}
		start := r.clock.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, req)
		r.Record(Entry{
			At:     start,
			IP:     httpx.ClientIP(req.Context()).String(),
			Method: truncate(req.Method, maxMethod),
			// エスケープした形で記録する（DB の text に不正な UTF-8 を入れないため）
			Path:     truncate(req.URL.EscapedPath(), maxPath),
			Status:   sw.status,
			Duration: r.clock.Now().Sub(start),
		})
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status, w.wroteHeader = code, true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
