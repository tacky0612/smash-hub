// Package auth は外部認証プロバイダーとの OAuth 2.0 のやり取りを提供する。
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
)

// Discord は Discord の OAuth 2.0 クライアント。要求するスコープは identify のみ。
type Discord struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTPClient   *http.Client
	// テストで差し替えられるようにエンドポイントを持つ
	AuthorizeURL string
	TokenURL     string
	UserURL      string
}

// NewDiscord は本番の Discord エンドポイントを使うクライアントを返す。
func NewDiscord(clientID, clientSecret, redirectURL string) *Discord {
	return &Discord{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		HTTPClient:   &http.Client{Timeout: 10 * time.Second},
		AuthorizeURL: "https://discord.com/oauth2/authorize",
		TokenURL:     "https://discord.com/api/oauth2/token",
		UserURL:      "https://discord.com/api/users/@me",
	}
}

// AuthCodeURL は認可画面の URL を返す。
func (d *Discord) AuthCodeURL(state, verifier string) string {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {d.ClientID},
		"redirect_uri":          {d.RedirectURL},
		"scope":                 {"identify"},
		"state":                 {state},
		"code_challenge":        {crypto.PKCEChallenge(verifier)},
		"code_challenge_method": {"S256"},
	}
	return d.AuthorizeURL + "?" + q.Encode()
}

// UserID は認可コードをトークンに交換し、Discord の利用者 ID だけを返す。
// アクセストークンや利用者名などは保存せず、この関数の中で破棄する。
func (d *Discord) UserID(ctx context.Context, code, verifier string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {d.RedirectURL},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(d.ClientID, d.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var tok struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := d.do(req, &tok); err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	if tok.AccessToken == "" {
		return "", errors.New("token exchange: empty access token")
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, d.UserURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	var user struct {
		ID string `json:"id"`
	}
	if err := d.do(req, &user); err != nil {
		return "", fmt.Errorf("fetch user: %w", err)
	}
	if user.ID == "" {
		return "", errors.New("fetch user: empty id")
	}
	return user.ID, nil
}

func (d *Discord) do(req *http.Request, v any) error {
	res, err := d.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(v)
}
