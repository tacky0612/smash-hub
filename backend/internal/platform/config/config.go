// Package config は環境変数から設定を読み込む。
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config はアプリケーションの設定。
type Config struct {
	Env              string // "development" / "production"
	HTTPAddr         string
	DatabaseURL      string
	PublicBaseURL    string // フロントエンドを配信している URL（OAuth のリダイレクト先）
	SourceCodeURL    string // AGPL-3.0 第13条: 利用者に提示するソースコードの URL
	CookieSecure     bool
	IdentifierPepper []byte

	DevLoginEnabled bool

	// 接続元の IP アドレスを伝えるヘッダー（frontend の nginx が付ける X-Real-IP）。空なら接続元のアドレスを使う
	ClientIPHeader string
	// アクセスログの保持期間（docs/security/data-protection.md、初期値 14日）
	AccessLogRetention time.Duration

	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURL  string

	SeasonID          string
	SeasonStart       time.Time
	RankingMinMatches int

	// 連合（docs/architecture/federation.md）
	InstanceURL        string        // インスタンス ID。連合先から到達できるベース URL
	InstanceName       string        // 表示用のインスタンス名
	InstancePrivateKey string        // インスタンスの署名鍵（Ed25519 のシード、base64）。DB とは別に保管する
	AccountKeySecret   []byte        // アカウントの秘密鍵を暗号化する鍵暗号化鍵（KEK）。32バイト以上
	Peers              []string      // 連合先（許可リスト）のベース URL
	SyncInterval       time.Duration // 連合先のログを取りに行く間隔
}

// DiscordEnabled は Discord ログインが設定されているかを返す。
func (c Config) DiscordEnabled() bool {
	return c.DiscordClientID != "" && c.DiscordClientSecret != "" && c.DiscordRedirectURL != ""
}

// Load は環境変数から設定を読み込み、検証する。
func Load() (Config, error) {
	c := Config{
		Env:                 env("APP_ENV", "development"),
		HTTPAddr:            env("HTTP_ADDR", ":8081"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		PublicBaseURL:       env("PUBLIC_BASE_URL", "http://localhost:8080"),
		SourceCodeURL:       env("SOURCE_CODE_URL", "https://github.com/tacky0612/smash-hub"),
		CookieSecure:        envBool("COOKIE_SECURE", true),
		IdentifierPepper:    []byte(os.Getenv("IDENTIFIER_PEPPER")),
		DevLoginEnabled:     envBool("DEV_LOGIN_ENABLED", false),
		ClientIPHeader:      os.Getenv("CLIENT_IP_HEADER"),
		AccessLogRetention:  time.Duration(envInt("ACCESS_LOG_RETENTION_DAYS", 14)) * 24 * time.Hour,
		DiscordClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURL:  os.Getenv("DISCORD_REDIRECT_URL"),
		SeasonID:            env("SEASON_ID", "S1"),
		RankingMinMatches:   envInt("RANKING_MIN_MATCHES", 20),
	}
	c.InstanceURL = strings.TrimRight(os.Getenv("INSTANCE_URL"), "/")
	c.InstanceName = env("INSTANCE_NAME", "SmashHub")
	c.InstancePrivateKey = os.Getenv("INSTANCE_PRIVATE_KEY")
	c.AccountKeySecret = []byte(os.Getenv("ACCOUNT_KEY_SECRET"))
	for _, p := range strings.Split(os.Getenv("PEERS"), ",") {
		if p = strings.TrimRight(strings.TrimSpace(p), "/"); p != "" {
			c.Peers = append(c.Peers, p)
		}
	}
	sync, err := time.ParseDuration(env("FEDERATION_SYNC_INTERVAL", "5s"))
	if err != nil {
		return c, fmt.Errorf("FEDERATION_SYNC_INTERVAL: %w", err)
	}
	c.SyncInterval = sync
	if c.InstanceURL == "" || c.InstancePrivateKey == "" {
		return c, errors.New("INSTANCE_URL and INSTANCE_PRIVATE_KEY are required")
	}
	if len(c.AccountKeySecret) < 32 {
		return c, errors.New("ACCOUNT_KEY_SECRET must be at least 32 bytes")
	}

	start, err := time.Parse(time.RFC3339, env("SEASON_START", "2026-01-01T00:00:00+09:00"))
	if err != nil {
		return c, fmt.Errorf("SEASON_START: %w", err)
	}
	c.SeasonStart = start

	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if len(c.IdentifierPepper) < 32 {
		return c, errors.New("IDENTIFIER_PEPPER must be at least 32 bytes")
	}
	if c.AccessLogRetention <= 0 {
		return c, errors.New("ACCESS_LOG_RETENTION_DAYS must be at least 1")
	}
	if c.Env == "production" && c.DevLoginEnabled {
		return c, errors.New("DEV_LOGIN_ENABLED must not be true in production")
	}
	return c, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return def
	}
	return v
}

func envInt(key string, def int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return def
	}
	return v
}
