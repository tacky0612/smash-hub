// Package adapter はアカウントコンテキストの永続化と HTTP ハンドラを実装する。
package adapter

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/tacky0612/smash-hub/backend/internal/modules/account/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/database"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// oauthStateTTL は OAuth の state の有効期間。
const oauthStateTTL = 10 * time.Minute

// Postgres は account のリポジトリを PostgreSQL で実装する。
type Postgres struct {
	DB *database.DB
}

// Accounts / Identities / Sessions / States はそれぞれのリポジトリインターフェースを返す。
func (p Postgres) Accounts() domain.AccountRepository    { return accountRepo(p) }
func (p Postgres) Identities() domain.IdentityRepository { return identityRepo(p) }
func (p Postgres) Sessions() domain.SessionRepository    { return sessionRepo(p) }
func (p Postgres) States() domain.OAuthStateRepository   { return stateRepo(p) }

type accountRepo Postgres

func (r accountRepo) Create(ctx context.Context, a domain.Account, publicKey, sealedPrivateKey []byte) error {
	_, err := r.DB.Q(ctx).Exec(ctx,
		`INSERT INTO accounts (id, display_name, main_fighter, created_at, public_key, sealed_private_key, terms_version, terms_agreed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.DisplayName, a.MainFighter, a.CreatedAt, publicKey, sealedPrivateKey, a.TermsVersion, a.TermsAgreedAt)
	return err
}

func (r accountRepo) SealedPrivateKey(ctx context.Context, id shared.AccountID) ([]byte, error) {
	var key []byte
	err := r.DB.Q(ctx).QueryRow(ctx, `SELECT sealed_private_key FROM accounts WHERE id = $1`, id).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && len(key) == 0) {
		return nil, shared.ErrNotFound
	}
	return key, err
}

func (r accountRepo) Get(ctx context.Context, id shared.AccountID) (domain.Account, error) {
	var a domain.Account
	var agreedAt *time.Time
	err := r.DB.Q(ctx).QueryRow(ctx,
		`SELECT id, display_name, main_fighter, created_at, terms_version, terms_agreed_at FROM accounts WHERE id = $1`, id).
		Scan(&a.ID, &a.DisplayName, &a.MainFighter, &a.CreatedAt, &a.TermsVersion, &agreedAt)
	if agreedAt != nil {
		a.TermsAgreedAt = *agreedAt
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return a, shared.ErrNotFound
	}
	return a, err
}

func (r accountRepo) Update(ctx context.Context, a domain.Account) error {
	_, err := r.DB.Q(ctx).Exec(ctx,
		`UPDATE accounts SET display_name = $2, main_fighter = $3, terms_version = $4, terms_agreed_at = $5 WHERE id = $1`,
		a.ID, a.DisplayName, a.MainFighter, a.TermsVersion, nullTime(a.TermsAgreedAt))
	return err
}

type identityRepo Postgres

func (r identityRepo) FindAccountID(ctx context.Context, hash string) (shared.AccountID, error) {
	var id shared.AccountID
	err := r.DB.Q(ctx).QueryRow(ctx, `SELECT account_id FROM external_identities WHERE identifier_hash = $1`, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", shared.ErrNotFound
	}
	return id, err
}

func (r identityRepo) Create(ctx context.Context, hash, provider string, accountID shared.AccountID, now time.Time) error {
	_, err := r.DB.Q(ctx).Exec(ctx,
		`INSERT INTO external_identities (identifier_hash, provider, account_id, created_at) VALUES ($1, $2, $3, $4)`,
		hash, provider, accountID, now)
	return err
}

type sessionRepo Postgres

func (r sessionRepo) Create(ctx context.Context, s domain.Session, now time.Time) error {
	_, err := r.DB.Q(ctx).Exec(ctx,
		`INSERT INTO sessions (token_hash, provider, identifier_hash, account_id, created_at, expires_at)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)`,
		s.TokenHash, s.Provider, s.IdentifierHash, string(s.AccountID), now, s.ExpiresAt)
	return err
}

func (r sessionRepo) Get(ctx context.Context, tokenHash string, now time.Time) (domain.Session, error) {
	var s domain.Session
	var accountID *string
	err := r.DB.Q(ctx).QueryRow(ctx,
		`SELECT token_hash, provider, identifier_hash, account_id, expires_at FROM sessions
		 WHERE token_hash = $1 AND expires_at > $2`, tokenHash, now).
		Scan(&s.TokenHash, &s.Provider, &s.IdentifierHash, &accountID, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, shared.ErrNotFound
	}
	if accountID != nil {
		s.AccountID = shared.AccountID(*accountID)
	}
	return s, err
}

func (r sessionRepo) AttachAccount(ctx context.Context, tokenHash string, accountID shared.AccountID) error {
	_, err := r.DB.Q(ctx).Exec(ctx, `UPDATE sessions SET account_id = $2 WHERE token_hash = $1`, tokenHash, accountID)
	return err
}

func (r sessionRepo) Delete(ctx context.Context, tokenHash string) error {
	_, err := r.DB.Q(ctx).Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

type stateRepo Postgres

func (r stateRepo) Save(ctx context.Context, state, verifier string, now time.Time) error {
	q := r.DB.Q(ctx)
	if _, err := q.Exec(ctx, `DELETE FROM oauth_states WHERE created_at < $1`, now.Add(-oauthStateTTL)); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `INSERT INTO oauth_states (state, verifier, created_at) VALUES ($1, $2, $3)`, state, verifier, now)
	return err
}

func (r stateRepo) Take(ctx context.Context, state string, now time.Time) (string, error) {
	var verifier string
	err := r.DB.Q(ctx).QueryRow(ctx,
		`DELETE FROM oauth_states WHERE state = $1 AND created_at >= $2 RETURNING verifier`,
		state, now.Add(-oauthStateTTL)).Scan(&verifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", shared.ErrNotFound
	}
	return verifier, err
}

// nullTime はゼロ値の時刻を NULL として書き込む。
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
