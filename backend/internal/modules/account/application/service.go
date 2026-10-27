// Package application はアカウントコンテキストのユースケースを定義する。
package application

import (
	"context"
	"errors"
	"strings"

	"github.com/tacky0612/smash-hub/backend/internal/modules/account/domain"
	sharedapp "github.com/tacky0612/smash-hub/backend/internal/shared/application"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Provider 名。
const (
	ProviderDev     = "dev"
	ProviderDiscord = "discord"
)

// IdentifierHasher は外部プロバイダーの利用者 ID を識別子ハッシュに変換する。
type IdentifierHasher interface {
	Hash(provider, providerUserID string) string
}

// Tokens はセッショントークンの生成とハッシュ化。
type Tokens interface {
	New() string
	Hash(token string) string
}

// KeyManager はアカウントの署名鍵を扱う。秘密鍵は暗号化した状態でだけ保存する。
type KeyManager interface {
	// Generate は鍵ペアを作り、公開鍵から導出したアカウント ID・公開鍵・暗号化した秘密鍵を返す。
	Generate() (id shared.AccountID, publicKey, sealedPrivateKey []byte)
	Sign(sealedPrivateKey, msg []byte) (string, error)
}

// AccountEvents はアカウントの公開情報が変わったことを連合に伝える（federation コンテキストが実装する）。
// sign はアカウント自身の鍵で署名する関数。
type AccountEvents interface {
	AccountChanged(ctx context.Context, a domain.Account, sign func(msg []byte) (string, error)) error
}

// OAuthProvider は外部プロバイダーの OAuth 2.0 クライアント。
type OAuthProvider interface {
	AuthCodeURL(state, verifier string) string
	UserID(ctx context.Context, code, verifier string) (string, error)
}

// Service はアカウントのユースケース。
type Service struct {
	Tx         sharedapp.TxManager
	Accounts   domain.AccountRepository
	Identities domain.IdentityRepository
	Sessions   domain.SessionRepository
	States     domain.OAuthStateRepository
	Hasher     IdentifierHasher
	Tokens     Tokens
	Clock      shared.Clock
	Keys       KeyManager
	Events     AccountEvents // 未設定なら連合に伝えない
	Discord    OAuthProvider // 未設定なら nil
	// TermsVersion は現在の利用規約の版。登録時に、利用者が読んだ版と一致することを確かめる
	TermsVersion string
}

// Me はログイン中の利用者の状態。
type Me struct {
	Registered bool
	Account    *domain.Account
}

// Login は外部プロバイダーで認証済みの利用者 ID でログインし、セッショントークンを返す。
// 利用者 ID は識別子ハッシュに変換してから扱い、そのものは保存しない。
func (s *Service) Login(ctx context.Context, provider, providerUserID string) (string, error) {
	if providerUserID == "" {
		return "", shared.Invalid("利用者 ID が空です")
	}
	hash := s.Hasher.Hash(provider, providerUserID)
	now := s.Clock.Now()
	token := s.Tokens.New()
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		accountID, err := s.Identities.FindAccountID(ctx, hash)
		if err != nil && !errors.Is(err, shared.ErrNotFound) {
			return err
		}
		return s.Sessions.Create(ctx, domain.Session{
			TokenHash:      s.Tokens.Hash(token),
			Provider:       provider,
			IdentifierHash: hash,
			AccountID:      accountID,
			ExpiresAt:      now.Add(domain.SessionTTL),
		}, now)
	})
	return token, err
}

// DevLogin は開発用ログイン。名前を利用者 ID とみなす（開発環境でのみ有効にする）。
func (s *Service) DevLogin(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 40 {
		return "", shared.Invalid("名前を1〜40文字で入力してください")
	}
	return s.Login(ctx, ProviderDev, name)
}

// StartDiscord は Discord の認可画面の URL を返す。
func (s *Service) StartDiscord(ctx context.Context, state, verifier string) (string, error) {
	if s.Discord == nil {
		return "", shared.ErrNotFound
	}
	if err := s.States.Save(ctx, state, verifier, s.Clock.Now()); err != nil {
		return "", err
	}
	return s.Discord.AuthCodeURL(state, verifier), nil
}

// CompleteDiscord は Discord からのコールバックを処理してログインする。
func (s *Service) CompleteDiscord(ctx context.Context, state, code string) (string, error) {
	if s.Discord == nil {
		return "", shared.ErrNotFound
	}
	verifier, err := s.States.Take(ctx, state, s.Clock.Now())
	if err != nil {
		return "", shared.ErrUnauthorized
	}
	userID, err := s.Discord.UserID(ctx, code, verifier)
	if err != nil {
		return "", err
	}
	return s.Login(ctx, ProviderDiscord, userID)
}

// Authenticate はセッショントークンからセッションを取得する。
func (s *Service) Authenticate(ctx context.Context, token string) (domain.Session, error) {
	if token == "" {
		return domain.Session{}, shared.ErrUnauthorized
	}
	sess, err := s.Sessions.Get(ctx, s.Tokens.Hash(token), s.Clock.Now())
	if errors.Is(err, shared.ErrNotFound) {
		return domain.Session{}, shared.ErrUnauthorized
	}
	return sess, err
}

// GetMe はログイン中の利用者の状態を返す。
func (s *Service) GetMe(ctx context.Context, sess domain.Session) (Me, error) {
	if !sess.Registered() {
		return Me{}, nil
	}
	a, err := s.Accounts.Get(ctx, sess.AccountID)
	if err != nil {
		return Me{}, err
	}
	return Me{Registered: true, Account: &a}, nil
}

// Register はログイン済み・未登録の利用者のアカウントを作る。
// termsVersion は利用者が読んで同意した利用規約の版で、現在の版と一致しなければ登録しない。
func (s *Service) Register(ctx context.Context, sess domain.Session, displayName string, mainFighter shared.FighterID, termsVersion string) (domain.Account, error) {
	if sess.Registered() {
		return domain.Account{}, shared.ErrConflict
	}
	if termsVersion == "" {
		return domain.Account{}, shared.Invalid("利用規約と運営ポリシーへの同意が必要です")
	}
	if termsVersion != s.TermsVersion {
		return domain.Account{}, shared.Invalid("利用規約が更新されました。読み直してから同意してください")
	}
	now := s.Clock.Now()
	id, pub, sealed := s.Keys.Generate()
	a, err := domain.NewAccount(id, strings.TrimSpace(displayName), mainFighter, now)
	if err != nil {
		return domain.Account{}, err
	}
	a.AgreeTerms(termsVersion, now)
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		// 同じ外部アカウントで別のセッションから既に登録されていないか確認する
		if _, err := s.Identities.FindAccountID(ctx, sess.IdentifierHash); err == nil {
			return shared.ErrConflict
		} else if !errors.Is(err, shared.ErrNotFound) {
			return err
		}
		if err := s.Accounts.Create(ctx, a, pub, sealed); err != nil {
			return err
		}
		if err := s.Identities.Create(ctx, sess.IdentifierHash, sess.Provider, a.ID, now); err != nil {
			return err
		}
		if err := s.Sessions.AttachAccount(ctx, sess.TokenHash, a.ID); err != nil {
			return err
		}
		return s.publish(ctx, a)
	})
	return a, err
}

// TermsAccepted は a が現在の版の利用規約に同意しているかを返す。
func (s *Service) TermsAccepted(a domain.Account) bool {
	return a.TermsVersion == s.TermsVersion
}

// AgreeTerms は登録済みの利用者が、更新された利用規約に同意したことを記録する。
// version は利用者が読んだ版で、現在の版と一致しなければ記録しない。
func (s *Service) AgreeTerms(ctx context.Context, id shared.AccountID, version string) (domain.Account, error) {
	if version != s.TermsVersion {
		return domain.Account{}, shared.Invalid("利用規約が更新されました。読み直してから同意してください")
	}
	var a domain.Account
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if a, err = s.Accounts.Get(ctx, id); err != nil {
			return err
		}
		a.AgreeTerms(version, s.Clock.Now())
		return s.Accounts.Update(ctx, a)
	})
	return a, err
}

// CanPlay は、現在の版の利用規約に同意していなければ理由付きの ErrForbidden を返す（matchmaking から使う）。
// 他インスタンスの利用者は、そのホームインスタンスの利用規約に従うため確かめない。
func (s *Service) CanPlay(ctx context.Context, id shared.AccountID) error {
	a, err := s.Accounts.Get(ctx, id)
	if errors.Is(err, shared.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !s.TermsAccepted(a) {
		return shared.Forbidden("利用規約が更新されました。新しい利用規約に同意してから対戦してください")
	}
	return nil
}

// UpdateProfile はプロフィールを変更する。
func (s *Service) UpdateProfile(ctx context.Context, id shared.AccountID, displayName string, mainFighter shared.FighterID) (domain.Account, error) {
	var a domain.Account
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if a, err = s.Accounts.Get(ctx, id); err != nil {
			return err
		}
		if err := a.ChangeProfile(strings.TrimSpace(displayName), mainFighter); err != nil {
			return err
		}
		if err := s.Accounts.Update(ctx, a); err != nil {
			return err
		}
		return s.publish(ctx, a)
	})
	return a, err
}

// SignAs はアカウント自身の鍵で msg に署名する（結果報告・アカウントの公開情報の署名に使う）。
func (s *Service) SignAs(ctx context.Context, id shared.AccountID, msg []byte) (string, error) {
	sealed, err := s.Accounts.SealedPrivateKey(ctx, id)
	if err != nil {
		return "", err
	}
	return s.Keys.Sign(sealed, msg)
}

func (s *Service) publish(ctx context.Context, a domain.Account) error {
	if s.Events == nil {
		return nil
	}
	return s.Events.AccountChanged(ctx, a, func(msg []byte) (string, error) { return s.SignAs(ctx, a.ID, msg) })
}

// DevAccountID は開発用ログインの名前に紐づくアカウント ID を返す（開発環境で管理者を指定するために使う）。
func (s *Service) DevAccountID(ctx context.Context, name string) (shared.AccountID, error) {
	return s.Identities.FindAccountID(ctx, s.Hasher.Hash(ProviderDev, name))
}

// GetAccount はアカウントを取得する。
func (s *Service) GetAccount(ctx context.Context, id shared.AccountID) (domain.Account, error) {
	return s.Accounts.Get(ctx, id)
}

// Logout はセッションを削除する。
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.Sessions.Delete(ctx, s.Tokens.Hash(token))
}
