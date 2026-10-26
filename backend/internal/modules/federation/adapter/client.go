package adapter

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/application"
	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
)

// インスタンス間のリクエストに付ける署名ヘッダー（docs/security/README.md 4.3）。
const (
	HeaderInstance  = "X-Smash-Hub-Instance"
	HeaderDate      = "X-Smash-Hub-Date"
	HeaderSignature = "X-Smash-Hub-Signature"
	// MaxClockSkew は署名に含める日時の許容範囲（再送攻撃の対策）。
	MaxClockSkew = 5 * time.Minute
)

// SigningString はリクエスト署名の対象。
func SigningString(method, pathAndQuery, date string, body []byte) []byte {
	sum := sha256.Sum256(body)
	return []byte(strings.Join([]string{method, pathAndQuery, date, hex.EncodeToString(sum[:])}, "\n"))
}

// Client は連合先へのリクエストを、インスタンスの鍵で署名して送る。
type Client struct {
	Self       string
	PrivateKey ed25519.PrivateKey
	HTTP       *http.Client
}

var _ application.PeerClient = (*Client)(nil)

// NewClient は Client を作る。
func NewClient(self string, priv ed25519.PrivateKey) *Client {
	return &Client{Self: self, PrivateKey: priv, HTTP: &http.Client{Timeout: 5 * time.Second}}
}

// Do は署名付きのリクエストを送り、JSON の応答を out に読み込む。
func (c *Client) Do(ctx context.Context, method, peer, path string, in, out any) error {
	var body []byte
	if in != nil {
		var err error
		if body, err = json.Marshal(in); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, peer+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	date := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderInstance, c.Self)
	req.Header.Set(HeaderDate, date)
	req.Header.Set(HeaderSignature, crypto.Sign(c.PrivateKey, SigningString(method, req.URL.RequestURI(), date, body)))
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if res.StatusCode >= 300 {
		return &PeerError{Status: res.StatusCode, Body: string(data)}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// PeerError は連合先がエラーを返したことを表す。
type PeerError struct {
	Status int
	Body   string
}

func (e *PeerError) Error() string { return fmt.Sprintf("peer returned %d: %s", e.Status, e.Body) }

// InstanceInfo は連合先の公開情報を取得する。
func (c *Client) InstanceInfo(ctx context.Context, peer string) (application.InstanceInfo, error) {
	var info application.InstanceInfo
	err := c.Do(ctx, http.MethodGet, peer, "/federation/instance", nil, &info)
	return info, err
}

// Log は連合先のログを since の次から取得する。
func (c *Client) Log(ctx context.Context, peer string, since int64, limit int) ([]domain.Event, error) {
	var res struct {
		Events []domain.Event `json:"events"`
	}
	q := url.Values{"since": {strconv.FormatInt(since, 10)}, "limit": {strconv.Itoa(limit)}}
	err := c.Do(ctx, http.MethodGet, peer, "/federation/log?"+q.Encode(), nil, &res)
	return res.Events, err
}

// VerifyKey は base64 の公開鍵で署名を検証する。
func VerifyKey(pubB64 string, msg []byte, sig string) bool {
	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		return false
	}
	return crypto.Verify(ed25519.PublicKey(pub), msg, sig)
}
