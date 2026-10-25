// Package crypto は識別子ハッシュ・トークン生成などの暗号処理を提供する。
package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// IdentifierHasher は外部プロバイダーの利用者 ID を、インスタンスごとのペッパーで HMAC した識別子ハッシュに変換する。
// 利用者 ID そのものは保存しない（docs/security/data-protection.md）。
type IdentifierHasher struct {
	Pepper []byte
}

// Hash は HMAC-SHA256(pepper, provider + ":" + providerUserID) を返す。
func (h IdentifierHasher) Hash(provider, providerUserID string) string {
	mac := hmac.New(sha256.New, h.Pepper)
	mac.Write([]byte(provider + ":" + providerUserID))
	return hex.EncodeToString(mac.Sum(nil))
}

// RandomToken は URL に使えるランダムなトークンを返す。
func RandomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken はセッショントークンを DB に保存するためのハッシュに変換する。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// PKCEChallenge は PKCE の code_challenge（S256）を返す。
func PKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
