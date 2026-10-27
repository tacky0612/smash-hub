// Package domain はアカウントコンテキストのドメインモデルを定義する。
package domain

import (
	"context"
	"time"
	"unicode/utf8"

	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Account はプレイヤーのアカウント。個人情報は持たず、利用者が入力した公開情報だけを持つ。
// ID は公開鍵から導出した did:key で、連合全体で一意（docs/architecture/federation.md 4.1）。
type Account struct {
	ID          shared.AccountID
	DisplayName string
	MainFighter shared.FighterID
	CreatedAt   time.Time
	// 登録時に同意した利用規約の版と日時（公開しない）
	TermsVersion  string
	TermsAgreedAt time.Time
}

// NewAccount は入力値を検証してアカウントを作る。
func NewAccount(id shared.AccountID, displayName string, mainFighter shared.FighterID, now time.Time) (Account, error) {
	a := Account{ID: id, CreatedAt: now}
	if err := a.ChangeProfile(displayName, mainFighter); err != nil {
		return Account{}, err
	}
	return a, nil
}

// ChangeProfile は表示名とメインファイターを変更する。
func (a *Account) ChangeProfile(displayName string, mainFighter shared.FighterID) error {
	n := utf8.RuneCountInString(displayName)
	if n < 1 || n > 20 {
		return shared.Invalid("表示名は1〜20文字で入力してください")
	}
	if !shared.IsValidFighter(mainFighter, false) {
		return shared.Invalid("メインファイターを選んでください")
	}
	a.DisplayName = displayName
	a.MainFighter = mainFighter
	return nil
}

// AgreeTerms は利用規約の版 version に同意したことを記録する。
func (a *Account) AgreeTerms(version string, now time.Time) {
	a.TermsVersion, a.TermsAgreedAt = version, now
}

// Session はログイン状態。外部プロバイダーでの認証が済んでいれば、アカウント登録前でも存在する。
type Session struct {
	TokenHash      string
	Provider       string
	IdentifierHash string
	AccountID      shared.AccountID // 登録前は空
	ExpiresAt      time.Time
}

// SessionTTL はセッションの有効期間。
const SessionTTL = 30 * 24 * time.Hour

// Registered はアカウント登録済みかを返す。
func (s Session) Registered() bool { return s.AccountID != "" }

// AccountRepository はアカウントの永続化。
type AccountRepository interface {
	// Create はアカウントと署名鍵（公開鍵と、暗号化した秘密鍵）を保存する。
	Create(ctx context.Context, a Account, publicKey, sealedPrivateKey []byte) error
	// SealedPrivateKey は暗号化した秘密鍵を返す。
	SealedPrivateKey(ctx context.Context, id shared.AccountID) ([]byte, error)
	Get(ctx context.Context, id shared.AccountID) (Account, error)
	Update(ctx context.Context, a Account) error
}

// IdentityRepository は外部プロバイダーとの紐づけの永続化。
type IdentityRepository interface {
	FindAccountID(ctx context.Context, identifierHash string) (shared.AccountID, error)
	Create(ctx context.Context, identifierHash, provider string, accountID shared.AccountID, now time.Time) error
}

// SessionRepository はセッションの永続化。
type SessionRepository interface {
	Create(ctx context.Context, s Session, now time.Time) error
	Get(ctx context.Context, tokenHash string, now time.Time) (Session, error)
	AttachAccount(ctx context.Context, tokenHash string, accountID shared.AccountID) error
	Delete(ctx context.Context, tokenHash string) error
}

// OAuthStateRepository は OAuth の state と PKCE verifier の一時保存。
type OAuthStateRepository interface {
	Save(ctx context.Context, state, verifier string, now time.Time) error
	// Take は state に対応する verifier を取り出して削除する。有効期限切れなら ErrNotFound。
	Take(ctx context.Context, state string, now time.Time) (string, error)
}
