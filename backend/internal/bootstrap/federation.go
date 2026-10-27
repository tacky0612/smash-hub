package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	accountapp "github.com/tacky0612/smash-hub/backend/internal/modules/account/application"
	accountdomain "github.com/tacky0612/smash-hub/backend/internal/modules/account/domain"
	fedadapter "github.com/tacky0612/smash-hub/backend/internal/modules/federation/adapter"
	fedapp "github.com/tacky0612/smash-hub/backend/internal/modules/federation/application"
	feddomain "github.com/tacky0612/smash-hub/backend/internal/modules/federation/domain"
	matchadapter "github.com/tacky0612/smash-hub/backend/internal/modules/match/adapter"
	matchapp "github.com/tacky0612/smash-hub/backend/internal/modules/match/application"
	matchdomain "github.com/tacky0612/smash-hub/backend/internal/modules/match/domain"
	mmapp "github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/application"
	mmdomain "github.com/tacky0612/smash-hub/backend/internal/modules/matchmaking/domain"
	ratingapp "github.com/tacky0612/smash-hub/backend/internal/modules/rating/application"
	ratingdomain "github.com/tacky0612/smash-hub/backend/internal/modules/rating/domain"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// このファイルは、連合（federation）と各コンテキストの port をつなぐ。
// 各コンテキストは互いの domain を参照しないため、型の変換はここで行う。

// accountEvents はアカウントの公開情報の変更を連合のログに追記する（accountapp.AccountEvents）。
type accountEvents struct {
	federation *fedapp.Service
}

func (e accountEvents) AccountChanged(ctx context.Context, a accountdomain.Account, sign func([]byte) (string, error)) error {
	p := feddomain.AccountPublished{AccountID: a.ID, DisplayName: a.DisplayName, HomeInstance: string(e.federation.Self)}
	sig, err := sign(p.SigningBytes())
	if err != nil {
		return err
	}
	p.AccountSig = sig
	return e.federation.PublishAccount(ctx, p)
}

// confirmedResults は連合で共有された対戦記録を rating の入力に変換する（ratingapp.ResultSource）。
type confirmedResults struct {
	federation *fedapp.Service
}

func (c confirmedResults) ConfirmedResults(ctx context.Context, season ratingdomain.Season) ([]ratingdomain.Result, error) {
	recs, err := c.federation.ConfirmedResults(ctx, season.Start, season.End)
	if err != nil {
		return nil, err
	}
	results := make([]ratingdomain.Result, 0, len(recs))
	for _, r := range recs {
		played, err := time.Parse(time.RFC3339, r.PlayedAt)
		if err != nil {
			continue
		}
		results = append(results, ratingdomain.Result{
			RecordID: r.RecordID(), PlayedAt: played,
			Winner: r.Players[r.Winner].AccountID, Loser: r.Players[1-r.Winner].AccountID,
		})
	}
	return results, nil
}

// recordReader は戦績の表示用に、連合で共有された対戦記録を返す（matchapp.RecordReader）。
type recordReader struct {
	store fedadapter.Postgres
}

func (r recordReader) RecordsFor(ctx context.Context, account shared.AccountID, limit int) ([]shared.MatchRecord, error) {
	recs, err := r.store.RecordsFor(ctx, account, limit)
	if err != nil {
		return nil, err
	}
	out := make([]shared.MatchRecord, len(recs))
	for i, rec := range recs {
		out[i] = rec.MatchRecord
	}
	return out, nil
}

// --- インスタンスをまたぐ対戦: 参加者のホームインスタンス側 ---

// remoteHosts は他インスタンスがホストする対戦の状態の取得と報告の中継（matchapp.RemoteHosts）。
type remoteHosts struct {
	client *fedadapter.Client
}

func (h remoteHosts) Live(ctx context.Context, host shared.InstanceID, id shared.MatchID) (matchdomain.Match, error) {
	var live fedapp.LiveMatch
	if err := h.client.Do(ctx, http.MethodGet, string(host), "/federation/matches/"+string(id), nil, &live); err != nil {
		return matchdomain.Match{}, peerErr(err)
	}
	return fromLive(live), nil
}

func (h remoteHosts) Report(ctx context.Context, host shared.InstanceID, id shared.MatchID, by shared.AccountID, sub matchdomain.Submission) (matchdomain.Match, error) {
	var live fedapp.LiveMatch
	req := fedapp.RemoteReport{MatchID: id, AccountID: by, Result: string(sub.Report), Sig: sub.Sig, Picks: sub.Picks, PicksSig: sub.PicksSig}
	if err := h.client.Do(ctx, http.MethodPost, string(host), "/federation/match-reports", req, &live); err != nil {
		return matchdomain.Match{}, peerErr(err)
	}
	return fromLive(live), nil
}

// remoteRequests は対戦募集の配信と、配信された募集への申し込み（mmapp.RemoteRequests）。
type remoteRequests struct {
	federation *fedapp.Service
	client     *fedadapter.Client
}

func (r *remoteRequests) Candidates(ctx context.Context) ([]mmdomain.Request, error) {
	remotes, err := r.federation.ActiveRemoteRequests(ctx)
	if err != nil {
		return nil, err
	}
	now := r.federation.Clock.Now()
	out := make([]mmdomain.Request, 0, len(remotes))
	for _, rr := range remotes {
		out = append(out, mmdomain.Request{
			ID: rr.ID, AccountID: rr.AccountID, Fighter: rr.Fighter, RateRange: rr.RateRange, Rating: rr.Rating,
			Status: mmdomain.StatusWaiting, CreatedAt: rr.CreatedAt, LastSeenAt: now, Origin: shared.InstanceID(rr.Origin),
		})
	}
	return out, nil
}

func (r *remoteRequests) Claim(ctx context.Context, target, mine mmdomain.Request) (shared.MatchID, error) {
	var res struct {
		MatchID shared.MatchID `json:"match_id"`
	}
	c := fedapp.Claim{RequestID: target.ID, AccountID: mine.AccountID, Fighter: mine.Fighter, RateRange: mine.RateRange, CreatedAt: mine.CreatedAt}
	if err := r.client.Do(ctx, http.MethodPost, string(target.Origin), "/federation/match-claims", c, &res); err != nil {
		return "", err
	}
	if res.MatchID == "" {
		return "", errors.New("host returned no match id")
	}
	return res.MatchID, nil
}

func (r *remoteRequests) Announce(req mmdomain.Request) {
	body := fedapp.RemoteRequest{ID: req.ID, AccountID: req.AccountID, Fighter: req.Fighter, RateRange: req.RateRange, Rating: req.Rating, CreatedAt: req.CreatedAt}
	r.broadcast(http.MethodPost, "/federation/match-requests", body)
}

func (r *remoteRequests) Withdraw(id string) {
	r.broadcast(http.MethodDelete, "/federation/match-requests/"+id, nil)
}

// broadcast は全連合先へ非同期に送る。配信は補助的なもので、失敗しても待機は続けられる。
func (r *remoteRequests) broadcast(method, path string, body any) {
	for _, peer := range r.federation.Peers {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := r.client.Do(ctx, method, peer, path, body, nil); err != nil {
				slog.Debug("broadcast failed", "peer", peer, "path", path, "err", err)
			}
		}()
	}
}

// --- インスタンスをまたぐ対戦: ホスト側 ---

// inboundMatchmaking は申し込みを matchmaking に渡す（fedapp.InboundMatchmaking）。
type inboundMatchmaking struct {
	matchmaking *mmapp.Service
}

func (m inboundMatchmaking) AcceptClaim(ctx context.Context, from string, c fedapp.Claim) (shared.MatchID, error) {
	id, err := m.matchmaking.AcceptClaim(ctx, shared.InstanceID(from), c.RequestID, c.AccountID, c.Fighter, c.RateRange, c.CreatedAt)
	if errors.Is(err, mmapp.ErrRequestUnavailable) {
		return "", &conflict{"この募集は終了しています"}
	}
	return id, err
}

// inboundMatches は対戦の状態の提供と中継された報告を match に渡す（fedapp.InboundMatches）。
type inboundMatches struct {
	matches *matchapp.Service
}

func (m inboundMatches) Live(ctx context.Context, from string, id shared.MatchID) (fedapp.LiveMatch, error) {
	match, err := m.matches.LiveForPeer(ctx, shared.InstanceID(from), id)
	if err != nil {
		return fedapp.LiveMatch{}, err
	}
	return toLive(match), nil
}

func (m inboundMatches) Report(ctx context.Context, from string, r fedapp.RemoteReport) (fedapp.LiveMatch, error) {
	sub := matchdomain.Submission{Report: matchdomain.Report(r.Result), Sig: r.Sig, Picks: r.Picks, PicksSig: r.PicksSig}
	match, err := m.matches.RemoteReport(ctx, shared.InstanceID(from), r.MatchID, r.AccountID, sub)
	if err != nil {
		return fedapp.LiveMatch{}, err
	}
	return toLive(match), nil
}

func toLive(m matchdomain.Match) fedapp.LiveMatch {
	return fedapp.LiveMatch{
		MatchID: m.ID, Host: string(m.Host), Player1: m.Player1, Player2: m.Player2, Player2Home: string(m.Player2Home),
		Fighter1: m.Fighter1, Fighter2: m.Fighter2, RoomID: m.RoomID, Status: string(m.Status),
		Report1: string(m.Report1), Report2: string(m.Report2), Winner: m.Winner, PlayedAt: m.PlayedAt, ExpiresAt: m.ExpiresAt,
		Picks1: m.Picks1, Picks2: m.Picks2,
	}
}

func fromLive(l fedapp.LiveMatch) matchdomain.Match {
	return matchdomain.Match{
		ID: l.MatchID, Host: shared.InstanceID(l.Host), Player1: l.Player1, Player2: l.Player2, Player2Home: shared.InstanceID(l.Player2Home),
		Fighter1: l.Fighter1, Fighter2: l.Fighter2, RoomID: l.RoomID, Status: matchdomain.Status(l.Status),
		Report1: matchdomain.Report(l.Report1), Report2: matchdomain.Report(l.Report2), Winner: l.Winner, PlayedAt: l.PlayedAt, ExpiresAt: l.ExpiresAt,
		Picks1: l.Picks1, Picks2: l.Picks2,
	}
}

// conflict は shared.ErrConflict として扱われるエラー（メッセージ付き）。
type conflict struct{ msg string }

func (c *conflict) Error() string        { return c.msg }
func (c *conflict) Is(target error) bool { return target == shared.ErrConflict }

// peerErr はホストの応答をこのインスタンスの利用者向けのエラーに変換する。
func peerErr(err error) error {
	switch {
	case fedadapter.IsPeerStatus(err, http.StatusNotFound):
		return shared.ErrNotFound
	case fedadapter.IsPeerStatus(err, http.StatusForbidden):
		return shared.ErrForbidden
	case fedadapter.IsPeerStatus(err, http.StatusBadRequest):
		return shared.Invalid("ホストのインスタンスが報告を受け付けませんでした")
	}
	return err
}

// --- 表示用 ---

// directory は表示用のプレイヤー情報を、自インスタンスのアカウント・他インスタンスのアカウント・レートから組み立てる。
type directory struct {
	accounts   *accountapp.Service
	ratings    *ratingapp.Service
	federation fedadapter.Postgres
}

func (d directory) DisplayName(ctx context.Context, id shared.AccountID) (string, error) {
	a, err := d.accounts.GetAccount(ctx, id)
	if err == nil {
		return a.DisplayName, nil
	}
	if !errors.Is(err, shared.ErrNotFound) {
		return "", err
	}
	ra, err := d.federation.RemoteAccount(ctx, id)
	if errors.Is(err, shared.ErrNotFound) {
		return "（他インスタンスの利用者）", nil // 公開情報をまだ取り込んでいない
	}
	return ra.DisplayName, err
}

func (d directory) Player(ctx context.Context, id shared.AccountID) (matchadapter.Player, error) {
	name, err := d.DisplayName(ctx, id)
	if err != nil {
		return matchadapter.Player{}, err
	}
	rating, err := d.ratings.CurrentRating(ctx, id)
	if err != nil {
		return matchadapter.Player{}, err
	}
	return matchadapter.Player{ID: id, DisplayName: name, Rating: rating}, nil
}
