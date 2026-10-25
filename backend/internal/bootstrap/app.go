// Package bootstrap はコンポジションルート。各モジュールを組み立て、モジュール間の port を接続する。
package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	accountadapter "github.com/tacky0612/smash-hub/backend/internal/modules/account/adapter"
	accountapp "github.com/tacky0612/smash-hub/backend/internal/modules/account/application"
	matchadapter "github.com/tacky0612/smash-hub/backend/internal/modules/match/adapter"
	matchapp "github.com/tacky0612/smash-hub/backend/internal/modules/match/application"
	mmadapter "github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/adapter"
	mmapp "github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/application"
	ratingadapter "github.com/tacky0612/smash-hub/backend/internal/modules/rating/adapter"
	ratingapp "github.com/tacky0612/smash-hub/backend/internal/modules/rating/application"
	ratingdomain "github.com/tacky0612/smash-hub/backend/internal/modules/rating/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/auth"
	"github.com/tacky0612/smash-hub/backend/internal/platform/config"
	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
	"github.com/tacky0612/smash-hub/backend/internal/platform/httpx"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// App は組み立て済みのアプリケーション。
type App struct {
	Handler     http.Handler
	Accounts    *accountapp.Service
	Matchmaking *mmapp.Service
	Matches     *matchapp.Service
	Ratings     *ratingapp.Service
}

// New は設定と DB からアプリケーションを組み立てる。clock は nil なら実時刻。
func New(cfg config.Config, db *database.DB, clock shared.Clock) *App {
	if clock == nil {
		clock = shared.SystemClock{}
	}

	ap := accountadapter.Postgres{DB: db}
	accounts := &accountapp.Service{
		Tx: db, Accounts: ap.Accounts(), Identities: ap.Identities(), Sessions: ap.Sessions(), States: ap.States(),
		Hasher: crypto.IdentifierHasher{Pepper: cfg.IdentifierPepper}, Tokens: tokens{}, Clock: clock,
	}
	if cfg.DiscordEnabled() {
		accounts.Discord = auth.NewDiscord(cfg.DiscordClientID, cfg.DiscordClientSecret, cfg.DiscordRedirectURL)
	}

	matchRepo := matchadapter.Postgres{DB: db}
	ratings := &ratingapp.Service{
		Standings:  ratingadapter.Postgres{DB: db},
		Results:    confirmedResults{matchRepo},
		Season:     ratingdomain.Season{ID: cfg.SeasonID, Start: cfg.SeasonStart},
		MinMatches: cfg.RankingMinMatches,
	}
	matches := &matchapp.Service{Tx: db, Matches: matchRepo, Clock: clock, Changed: recompute{ratings}}
	matchmaking := &mmapp.Service{Tx: db, Requests: mmadapter.Postgres{DB: db}, Matches: matches, Ratings: ratings, Clock: clock}

	accountHTTP := &accountadapter.HTTP{
		Service: accounts, CookieSecure: cfg.CookieSecure, DevLoginEnabled: cfg.DevLoginEnabled,
		PublicBaseURL: cfg.PublicBaseURL, SourceCodeURL: cfg.SourceCodeURL,
	}
	dir := directory{accounts: accounts, ratings: ratings}

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

	return &App{
		Handler:     httpx.SecurityHeaders(httpx.CSRF(accountHTTP.Middleware(mux))),
		Accounts:    accounts,
		Matchmaking: matchmaking,
		Matches:     matches,
		Ratings:     ratings,
	}
}

// RunSweeper は期限切れの対戦と離脱した募集を定期的に片付ける。ctx が終わるまで戻らない。
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
		}
	}
}

// tokens は accountapp.Tokens の実装。
type tokens struct{}

func (tokens) New() string              { return crypto.RandomToken() }
func (tokens) Hash(token string) string { return crypto.HashToken(token) }

// confirmedResults は match の確定した対戦を rating の入力に変換する（ratingapp.ResultSource）。
type confirmedResults struct {
	matches matchadapter.Postgres
}

func (c confirmedResults) ConfirmedResults(ctx context.Context, season ratingdomain.Season) ([]ratingdomain.Result, error) {
	ms, err := c.matches.ConfirmedBetween(ctx, season.Start, season.End)
	if err != nil {
		return nil, err
	}
	results := make([]ratingdomain.Result, 0, len(ms))
	for _, m := range ms {
		results = append(results, ratingdomain.Result{
			MatchID: m.ID, PlayedAt: m.PlayedAt, Winner: m.Winner, Loser: m.Opponent(m.Winner),
		})
	}
	return results, nil
}

// recompute は対戦の確定を受けてレートを再計算する（matchapp.ResultsChanged）。
type recompute struct {
	ratings *ratingapp.Service
}

func (r recompute) OnResultsChanged(ctx context.Context) error { return r.ratings.Recompute(ctx) }

// directory は表示用のプレイヤー情報を account と rating から組み立てる。
type directory struct {
	accounts *accountapp.Service
	ratings  *ratingapp.Service
}

func (d directory) Player(ctx context.Context, id shared.AccountID) (matchadapter.Player, error) {
	a, err := d.accounts.GetAccount(ctx, id)
	if err != nil {
		return matchadapter.Player{}, err
	}
	rating, err := d.ratings.CurrentRating(ctx, id)
	if err != nil {
		return matchadapter.Player{}, err
	}
	return matchadapter.Player{ID: id, DisplayName: a.DisplayName, Rating: rating}, nil
}

func (d directory) DisplayName(ctx context.Context, id shared.AccountID) (string, error) {
	a, err := d.accounts.GetAccount(ctx, id)
	return a.DisplayName, err
}
