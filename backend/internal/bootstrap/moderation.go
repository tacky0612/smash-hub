package bootstrap

import (
	"context"
	"errors"

	accountapp "github.com/tacky0612/smash-hub/backend/internal/modules/account/application"
	fedadapter "github.com/tacky0612/smash-hub/backend/internal/modules/federation/adapter"
	fedapp "github.com/tacky0612/smash-hub/backend/internal/modules/federation/application"
	matchapp "github.com/tacky0612/smash-hub/backend/internal/modules/match/application"
	mmapp "github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/application"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// このファイルは、モデレーション（moderation）と各コンテキストの port をつなぐ。

// admins は設定で指定した管理者（modapp.Admins）。
type admins struct {
	ids      map[shared.AccountID]bool
	devNames []string // 開発用ログインの名前（DEV_LOGIN_ENABLED のときだけ設定できる）
	accounts *accountapp.Service
}

func newAdmins(ids, devNames []string, accounts *accountapp.Service) admins {
	a := admins{ids: map[shared.AccountID]bool{}, devNames: devNames, accounts: accounts}
	for _, id := range ids {
		a.ids[shared.AccountID(id)] = true
	}
	return a
}

func (a admins) IsAdmin(ctx context.Context, id shared.AccountID) (bool, error) {
	if a.ids[id] {
		return true, nil
	}
	for _, name := range a.devNames {
		dev, err := a.accounts.DevAccountID(ctx, name)
		if errors.Is(err, shared.ErrNotFound) {
			continue
		}
		if err != nil {
			return false, err
		}
		if dev == id {
			return true, nil
		}
	}
	return false, nil
}

// eligibility は、利用規約への同意（account）と利用停止（moderation）の両方を確かめる（mmapp.Eligibility）。
type eligibility []mmapp.Eligibility

func (e eligibility) CanPlay(ctx context.Context, id shared.AccountID) error {
	for _, c := range e {
		if err := c.CanPlay(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// knownAccounts は自インスタンスのアカウントと、取り込み済みの他インスタンスのアカウントを対象にする（modapp.Accounts）。
type knownAccounts struct {
	accounts   *accountapp.Service
	federation fedadapter.Postgres
}

func (k knownAccounts) Exists(ctx context.Context, id shared.AccountID) (bool, error) {
	if _, err := k.accounts.GetAccount(ctx, id); err == nil {
		return true, nil
	} else if !errors.Is(err, shared.ErrNotFound) {
		return false, err
	}
	if _, err := k.federation.RemoteAccount(ctx, id); err == nil {
		return true, nil
	} else if !errors.Is(err, shared.ErrNotFound) {
		return false, err
	}
	return false, nil
}

// voidReader は表示用に、無効化された対戦を返す（matchapp.VoidReader）。
type voidReader struct {
	store fedadapter.Postgres
}

func (v voidReader) VoidedMatches(ctx context.Context, refs []matchapp.MatchRef) (map[matchapp.MatchRef]bool, error) {
	keys := make([]fedapp.MatchKey, len(refs))
	for i, r := range refs {
		keys[i] = fedapp.MatchKey{Host: r.Host, MatchID: r.ID}
	}
	voided, err := v.store.VoidedMatches(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := make(map[matchapp.MatchRef]bool, len(voided))
	for k := range voided {
		out[matchapp.MatchRef{Host: k.Host, ID: k.MatchID}] = true
	}
	return out, nil
}
