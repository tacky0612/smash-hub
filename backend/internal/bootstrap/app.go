// Package bootstrap はコンポジションルート。各モジュールを組み立て、モジュール間の port を接続する。
package bootstrap

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"log/slog"
	"net/http"
	"time"

	accountadapter "github.com/tacky0612/smash-hub/backend/internal/modules/account/adapter"
	accountapp "github.com/tacky0612/smash-hub/backend/internal/modules/account/application"
	fedadapter "github.com/tacky0612/smash-hub/backend/internal/modules/federation/adapter"
	fedapp "github.com/tacky0612/smash-hub/backend/internal/modules/federation/application"
	matchadapter "github.com/tacky0612/smash-hub/backend/internal/modules/match/adapter"
	matchapp "github.com/tacky0612/smash-hub/backend/internal/modules/match/application"
	mmadapter "github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/adapter"
	mmapp "github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/application"
	ratingadapter "github.com/tacky0612/smash-hub/backend/internal/modules/rating/adapter"
	ratingapp "github.com/tacky0612/smash-hub/backend/internal/modules/rating/application"
	ratingdomain "github.com/tacky0612/smash-hub/backend/internal/modules/rating/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/accesslog"
	"github.com/tacky0612/smash-hub/backend/internal/platform/auth"
	"github.com/tacky0612/smash-hub/backend/internal/platform/config"
	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
	"github.com/tacky0612/smash-hub/backend/internal/platform/ratelimit"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// App は組み立て済みのアプリケーション。
type App struct {
	Handler     http.Handler
	Accounts    *accountapp.Service
	Matchmaking *mmapp.Service
	Matches     *matchapp.Service
	Ratings     *ratingapp.Service
	Federation  *fedapp.Service
	AccessLog   *accesslog.Recorder

	clock              shared.Clock
	accessLogRetention time.Duration
}

// New は設定と DB からアプリケーションを組み立てる。clock は nil なら実時刻。
func New(cfg config.Config, db *database.DB, clock shared.Clock) (*App, error) {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	self := shared.InstanceID(cfg.InstanceURL)
	instanceKey, err := crypto.PrivateKeyFromSeed(cfg.InstancePrivateKey)
	if err != nil {
		return nil, err
	}
	sealer, err := crypto.NewSealer(cfg.AccountKeySecret)
	if err != nil {
		return nil, err
	}

	// 連合
	fedStore := fedadapter.Postgres{DB: db}
	client := fedadapter.NewClient(cfg.InstanceURL, instanceKey)
	federation := &fedapp.Service{
		Tx: db, Store: fedStore, Client: client, Clock: clock,
		Self: self, Name: cfg.InstanceName,
		PublicKey: base64.StdEncoding.EncodeToString(instanceKey.Public().(ed25519.PublicKey)),
		Sign:      func(msg []byte) string { return crypto.Sign(instanceKey, msg) },
		VerifyKey: fedadapter.VerifyKey,
		VerifyDID: crypto.VerifyDID,
		Peers:     cfg.Peers,
	}

	// アカウント
	ap := accountadapter.Postgres{DB: db}
	accounts := &accountapp.Service{
		Tx: db, Accounts: ap.Accounts(), Identities: ap.Identities(), Sessions: ap.Sessions(), States: ap.States(),
		Hasher: crypto.IdentifierHasher{Pepper: cfg.IdentifierPepper}, Tokens: tokens{}, Clock: clock,
		Keys: accountadapter.Keys{Sealer: sealer}, Events: accountEvents{federation},
	}
	if cfg.DiscordEnabled() {
		accounts.Discord = auth.NewDiscord(cfg.DiscordClientID, cfg.DiscordClientSecret, cfg.DiscordRedirectURL)
	}

	// レーティング・対戦・マッチング
	ratings := &ratingapp.Service{
		Standings:  ratingadapter.Postgres{DB: db},
		Results:    confirmedResults{federation},
		Season:     ratingdomain.Season{ID: cfg.SeasonID, Start: cfg.SeasonStart},
		MinMatches: cfg.RankingMinMatches,
	}
	federation.RecordsChanged = ratings.Recompute
	matches := &matchapp.Service{
		Tx: db, Matches: matchadapter.Postgres{DB: db, Self: self}, Remote: matchadapter.RemoteRefs{DB: db},
		Clock: clock, Self: self, Changed: recompute{ratings},
		Signer: accounts, Publisher: federation, Records: recordReader{fedStore},
		Hosts: remoteHosts{client}, VerifyDID: crypto.VerifyDID,
	}
	matchmaking := &mmapp.Service{
		Tx: db, Requests: mmadapter.Postgres{DB: db}, Matches: matches, Ratings: ratings, Clock: clock,
		Remote: &remoteRequests{federation: federation, client: client},
	}

	accountHTTP := &accountadapter.HTTP{
		Service: accounts, CookieSecure: cfg.CookieSecure, DevLoginEnabled: cfg.DevLoginEnabled,
		PublicBaseURL: cfg.PublicBaseURL, SourceCodeURL: cfg.SourceCodeURL, InstanceName: cfg.InstanceName,
	}
	dir := directory{accounts: accounts, ratings: ratings, federation: fedStore}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Pool.Ping(r.Context()); err != nil {
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	accountHTTP.Register(mux)
	(&mmadapter.HTTP{Service: matchmaking}).Register(mux)
	(&matchadapter.HTTP{Service: matches, Players: dir}).Register(mux)
	(&ratingadapter.HTTP{Service: ratings, Names: dir}).Register(mux)
	(&fedadapter.HTTP{Service: federation, Matchmaking: inboundMatchmaking{matchmaking}, Matches: inboundMatches{matches}}).Register(mux)

	// 外側から: 接続元の IP → アクセスログ → CSRF → セッション → レート制限（IP とアカウントで数える）
	accessLog := accesslog.NewRecorder(db, clock)
	var handler http.Handler = ratelimit.New(clock).Middleware(mux)
	handler = accountHTTP.Middleware(handler)
	handler = httpx.CSRF(handler)
	handler = accessLog.Middleware(handler)
	handler = httpx.ClientIPFrom(cfg.ClientIPHeader)(handler)
	handler = httpx.SecurityHeaders(handler)

	return &App{
		Handler:     handler,
		Accounts:    accounts,
		Matchmaking: matchmaking,
		Matches:     matches,
		Ratings:     ratings,
		Federation:  federation,
		AccessLog:   accessLog,

		clock:              clock,
		accessLogRetention: cfg.AccessLogRetention,
	}, nil
}

// RunSweeper は期限切れの対戦・離脱した募集・保持期間を過ぎたアクセスログを定期的に片付ける。ctx が終わるまで戻らない。
func (a *App) RunSweeper(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := a.Matches.ExpireDue(ctx); err != nil {
				slog.Error("expire matches", "err", err)
			} else if n > 0 {
				slog.Info("expired matches", "count", n)
			}
			if _, err := a.Matchmaking.CancelStale(ctx); err != nil {
				slog.Error("cancel stale requests", "err", err)
			}
			if _, err := a.PurgeAccessLogs(ctx); err != nil {
				slog.Error("purge access logs", "err", err)
			}
		}
	}
}

// PurgeAccessLogs は保持期間を過ぎたアクセスログを削除し、削除した件数を返す。
func (a *App) PurgeAccessLogs(ctx context.Context) (int64, error) {
	return a.AccessLog.Purge(ctx, a.clock.Now().Add(-a.accessLogRetention))
}

// RunFederationSync は連合先のログを定期的に取り込む（pull）。ctx が終わるまで戻らない。
func (a *App) RunFederationSync(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.Federation.SyncAll(ctx)
		}
	}
}

// tokens は accountapp.Tokens の実装。
type tokens struct{}

func (tokens) New() string              { return crypto.RandomToken() }
func (tokens) Hash(token string) string { return crypto.HashToken(token) }

// recompute は対戦の確定を受けてレートを再計算する（matchapp.ResultsChanged）。
type recompute struct {
	ratings *ratingapp.Service
}

func (r recompute) OnResultsChanged(ctx context.Context) error { return r.ratings.Recompute(ctx) }
