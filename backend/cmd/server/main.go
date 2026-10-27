// Command server は SmashHub のバックエンドを起動する。
//
//	server                                       サーバーを起動する
//	server snapshot [-o ファイル]                 ログのスナップショットを書き出す（既定は標準出力。DB の設定が必要）
//	server verify-snapshot [-key URL=鍵] ファイル  スナップショットを検証し、レートを再計算して突き合わせる（DB は不要）
//
// スナップショットは docs/architecture/federation.md の「8.3 スナップショット」。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/bootstrap"
	"github.com/tacky0612/smash-hub/backend/internal/platform/config"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
)

func main() {
	if len(os.Args) > 1 {
		// サブコマンドは標準出力に結果を書くため、ログは標準エラー出力に出す
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		var err error
		switch os.Args[1] {
		case "snapshot":
			err = runSnapshot(os.Args[2:])
		case "verify-snapshot":
			err = runVerifySnapshot(os.Args[2:], os.Stdout)
		default:
			err = fmt.Errorf("unknown command %q (snapshot / verify-snapshot)", os.Args[1])
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "エラー:", err)
			os.Exit(1)
		}
		return
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := openWithRetry(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return err
	}

	app, err := bootstrap.New(cfg, db, nil)
	if err != nil {
		return err
	}
	go app.RunSweeper(ctx, 30*time.Second)
	go app.RunFederationSync(ctx, cfg.SyncInterval)
	go app.AccessLog.Run(ctx, time.Second)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	slog.Info("listening", "addr", cfg.HTTPAddr, "instance", cfg.InstanceURL, "peers", cfg.Peers, "devLogin", cfg.DevLoginEnabled, "discord", cfg.DiscordEnabled())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	// 処理中のリクエストが終わるのを待ち、書き込み待ちのアクセスログを DB に書き込んでから止まる
	<-shutdownDone
	flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	app.AccessLog.Flush(flushCtx)
	return nil
}

// openWithRetry は DB の起動を待って接続する。
func openWithRetry(ctx context.Context, url string) (*database.DB, error) {
	var lastErr error
	for range 30 {
		db, err := database.Open(ctx, url)
		if err == nil {
			return db, nil
		}
		lastErr = err
		slog.Info("waiting for database", "err", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, lastErr
}
