// Package config は環境変数から設定を読み込む。
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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

	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURL  string

	SeasonID          string
	SeasonStart       time.Time
	RankingMinMatches int
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
		DiscordClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURL:  os.Getenv("DISCORD_REDIRECT_URL"),
		SeasonID:            env("SEASON_ID", "S1"),
		RankingMinMatches:   envInt("RANKING_MIN_MATCHES", 20),
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
