package bootstrap

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	fedadapter "github.com/tacky0612/smash-hub/backend/internal/modules/federation/adapter"
	fedapp "github.com/tacky0612/smash-hub/backend/internal/modules/federation/application"
	feddomain "github.com/tacky0612/smash-hub/backend/internal/modules/federation/domain"
	ratingdomain "github.com/tacky0612/smash-hub/backend/internal/modules/rating/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// このファイルは、ログのスナップショットの出力と検証を扱う（docs/architecture/federation.md の「8.3 スナップショット」）。
// スナップショットは、インスタンスが保存しているログ（自インスタンスのログと、取り込んだ連合先のログ）の全体と、
// そのインスタンスが計算したレートを1つにまとめたもの。検証では、DB を使わずにログを取り込み直してレートを再計算し、
// スナップショットに含まれるレートと一致するかを確かめる。

// スナップショットの形式。
const (
	SnapshotFormat  = "smashhub-snapshot"
	SnapshotVersion = 1
)

// Snapshot はログのスナップショット。sig は header の正規化 JSON に対する、作成したインスタンスの署名。
// header の chains が各ログの末尾のハッシュを持つため、この署名は events の全体にも及ぶ。
type Snapshot struct {
	Header SnapshotHeader    `json:"header"`
	Sig    string            `json:"sig"`
	Events []feddomain.Event `json:"events"` // chains の順に、各ログの seq の昇順
}

// SnapshotHeader はスナップショットの見出し。
type SnapshotHeader struct {
	Format     string             `json:"format"`
	Version    int                `json:"version"`
	InstanceID string             `json:"instance_id"` // 作成したインスタンス
	PublicKey  string             `json:"public_key"`  // 作成したインスタンスの公開鍵（base64）
	CreatedAt  string             `json:"created_at"`
	Chains     []fedapp.ChainHead `json:"chains"`
	Season     SnapshotSeason     `json:"season"`
	Ratings    []SnapshotStanding `json:"ratings"` // 作成したインスタンスが計算した、シーズンの成績（アカウント ID の順）
}

// SnapshotSeason はレートを集計したシーズン。
type SnapshotSeason struct {
	ID    string `json:"id"`
	Start string `json:"start"`
	End   string `json:"end,omitempty"`
}

// SnapshotStanding はシーズンのプレイヤーの成績。
type SnapshotStanding struct {
	AccountID shared.AccountID `json:"account_id"`
	Rating    int              `json:"rating"`
	Matches   int              `json:"matches"`
	Wins      int              `json:"wins"`
	Losses    int              `json:"losses"`
}

func standingOf(s ratingdomain.Standing) SnapshotStanding {
	return SnapshotStanding{AccountID: s.AccountID, Rating: s.Rating, Matches: s.Matches, Wins: s.Wins, Losses: s.Losses}
}

func (h SnapshotHeader) signingBytes() ([]byte, error) { return shared.Canonical(h) }

// snapshotPageSize はスナップショットの出力で、DB から一度に読むイベントの数。
const snapshotPageSize = 1000

// WriteSnapshot はスナップショットを w に書き出す。DB の同じ時点の内容を読むため、出力中に追記されたイベントは含まない。
// イベントは DB から少しずつ読んで書き出す（ログが大きくてもメモリを使い切らない）。
func (a *App) WriteSnapshot(ctx context.Context, w io.Writer) error {
	return a.db.InReadOnlySnapshot(ctx, func(ctx context.Context) error {
		heads, err := a.fedStore.ChainHeads(ctx)
		if err != nil {
			return err
		}
		self := string(a.Federation.Self)
		for i, h := range heads {
			if h.InstanceID == self {
				heads[i].PublicKey = a.Federation.PublicKey
			}
			if heads[i].PublicKey == "" {
				return fmt.Errorf("public key of %s is unknown", h.InstanceID)
			}
		}
		season := a.Ratings.Season
		standings, err := a.ratingStore.All(ctx, season.ID)
		if err != nil {
			return err
		}
		header := SnapshotHeader{
			Format: SnapshotFormat, Version: SnapshotVersion,
			InstanceID: self, PublicKey: a.Federation.PublicKey, CreatedAt: shared.FormatTime(a.clock.Now()),
			Chains: heads, Season: SnapshotSeason{ID: season.ID, Start: shared.FormatTime(season.Start)},
			Ratings: make([]SnapshotStanding, len(standings)),
		}
		if !season.End.IsZero() {
			header.Season.End = shared.FormatTime(season.End)
		}
		for i, s := range standings {
			header.Ratings[i] = standingOf(s)
		}
		hb, err := header.signingBytes()
		if err != nil {
			return err
		}
		sig, err := json.Marshal(a.Federation.Sign(hb))
		if err != nil {
			return err
		}

		bw := bufio.NewWriter(w)
		fmt.Fprintf(bw, `{"header":%s,"sig":%s,"events":[`, hb, sig)
		first := true
		for _, h := range heads {
			for since := int64(0); since < h.Seq; {
				bodies, err := a.fedStore.Events(ctx, h.InstanceID, since, snapshotPageSize)
				if err != nil {
					return err
				}
				if len(bodies) == 0 {
					return fmt.Errorf("log of %s ends at seq %d (want %d)", h.InstanceID, since, h.Seq)
				}
				for _, b := range bodies {
					if !first {
						bw.WriteByte(',')
					}
					first = false
					bw.Write(b)
				}
				since += int64(len(bodies))
			}
		}
		bw.WriteString("]}\n")
		return bw.Flush()
	})
}

// SnapshotReport はスナップショットの検証結果。
type SnapshotReport struct {
	Header    SnapshotHeader
	Records   int                // 取り込んだ対戦記録の数（シーズン外も含む）
	Effective int                // シーズンのレート計算に使った記録の数（取り込みの締め切りと無効化を適用した後）
	Computed  []SnapshotStanding // ログから再計算した成績（アカウント ID の順）
	// Mismatches は、スナップショットに含まれる成績と再計算した成績の食い違い。空なら一致している
	Mismatches []string
}

// ErrSnapshotInvalid はスナップショットの形式・署名・ログの検証の失敗。
var ErrSnapshotInvalid = errors.New("snapshot is invalid")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrSnapshotInvalid, fmt.Sprintf(format, args...))
}

// VerifySnapshot はスナップショットを検証し、ログからレートを再計算する。DB は使わない。
// ログの取り込みには、連合先からの取り込み（pull 同期）と同じ処理を使う。そのため、署名・連番・ハッシュチェーン・
// プレイヤー署名の検証と、取り込みの締め切り・無効化の扱いは、インスタンスで動いているものと同じになる。
//
// keys には、信頼できる経路（各インスタンスの /federation/instance 等）で確かめた公開鍵をインスタンス ID ごとに渡せる。
// 渡した鍵とスナップショットの鍵が違えば失敗にする。渡さなかったインスタンスは、スナップショットの鍵を信じる。
func VerifySnapshot(ctx context.Context, r io.Reader, keys map[string]string) (SnapshotReport, error) {
	var snap Snapshot
	dec := json.NewDecoder(r)
	if err := dec.Decode(&snap); err != nil {
		return SnapshotReport{}, invalid("cannot decode: %v", err)
	}
	h := snap.Header
	report := SnapshotReport{Header: h}
	if h.Format != SnapshotFormat || h.Version != SnapshotVersion {
		return report, invalid("unsupported format %q version %d", h.Format, h.Version)
	}
	pinned := func(id, key string) error {
		if want, ok := keys[id]; ok && want != key {
			return invalid("public key of %s does not match the given key", id)
		}
		return nil
	}
	if err := pinned(h.InstanceID, h.PublicKey); err != nil {
		return report, err
	}
	hb, err := h.signingBytes()
	if err != nil {
		return report, err
	}
	if !fedadapter.VerifyKey(h.PublicKey, hb, snap.Sig) {
		return report, invalid("signature of the snapshot is invalid")
	}

	// ログをオリジンごとに分ける
	logs := snapshotLogs{keys: map[string]string{}, events: map[string][]feddomain.Event{}}
	var origins []string
	for _, c := range h.Chains {
		if _, dup := logs.keys[c.InstanceID]; dup {
			return report, invalid("duplicate chain %s", c.InstanceID)
		}
		if c.InstanceID == h.InstanceID && c.PublicKey != h.PublicKey {
			return report, invalid("public key of %s differs between header and chain", c.InstanceID)
		}
		if err := pinned(c.InstanceID, c.PublicKey); err != nil {
			return report, err
		}
		logs.keys[c.InstanceID] = c.PublicKey
		origins = append(origins, c.InstanceID)
	}
	for _, e := range snap.Events {
		if _, ok := logs.keys[e.InstanceID]; !ok {
			return report, invalid("event from %s is not in chains", e.InstanceID)
		}
		logs.events[e.InstanceID] = append(logs.events[e.InstanceID], e)
	}

	// 連合先のログとして取り込み直す
	created, err := time.Parse(time.RFC3339, h.CreatedAt)
	if err != nil {
		return report, invalid("created_at: %v", err)
	}
	store := fedadapter.NewMemory()
	fed := &fedapp.Service{
		Tx: noTx{}, Store: store, Client: logs, Clock: fixedClock{created},
		Self: "snapshot-verifier", VerifyKey: fedadapter.VerifyKey, VerifyDID: crypto.VerifyDID, Peers: origins,
	}
	for _, c := range h.Chains {
		for {
			n, err := fed.SyncPeer(ctx, c.InstanceID)
			if err != nil {
				return report, invalid("log of %s: %v", c.InstanceID, err)
			}
			if n == 0 {
				break
			}
		}
		st, err := store.Peer(ctx, c.InstanceID)
		if err != nil {
			return report, err
		}
		if st.LastSeq != c.Seq || st.LastHash != c.Hash {
			return report, invalid("log of %s ends at seq %d, but the header says seq %d", c.InstanceID, st.LastSeq, c.Seq)
		}
	}

	// レートを再計算して突き合わせる
	season, err := h.Season.parse()
	if err != nil {
		return report, err
	}
	all, err := store.Records(ctx, time.Time{}, time.Time{})
	if err != nil {
		return report, err
	}
	report.Records = len(all)
	results, err := confirmedResults{fed}.ConfirmedResults(ctx, season)
	if err != nil {
		return report, err
	}
	report.Effective = len(results)
	for _, s := range ratingdomain.Compute(results) {
		report.Computed = append(report.Computed, standingOf(s))
	}
	sort.Slice(report.Computed, func(i, j int) bool { return report.Computed[i].AccountID < report.Computed[j].AccountID })
	report.Mismatches = diffStandings(h.Ratings, report.Computed)
	return report, nil
}

func (s SnapshotSeason) parse() (ratingdomain.Season, error) {
	season := ratingdomain.Season{ID: s.ID}
	var err error
	if season.Start, err = time.Parse(time.RFC3339, s.Start); err != nil {
		return season, invalid("season start: %v", err)
	}
	if s.End != "" {
		if season.End, err = time.Parse(time.RFC3339, s.End); err != nil {
			return season, invalid("season end: %v", err)
		}
	}
	return season, nil
}

// diffStandings はスナップショットの成績と再計算した成績の食い違いを返す。
func diffStandings(want, got []SnapshotStanding) []string {
	byID := map[shared.AccountID]SnapshotStanding{}
	for _, s := range got {
		byID[s.AccountID] = s
	}
	var out []string
	for _, w := range want {
		g, ok := byID[w.AccountID]
		delete(byID, w.AccountID)
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("%s: snapshot %+v, recomputed (none)", w.AccountID, w))
		case g != w:
			out = append(out, fmt.Sprintf("%s: snapshot %+v, recomputed %+v", w.AccountID, w, g))
		}
	}
	for _, g := range got {
		if _, ok := byID[g.AccountID]; ok {
			out = append(out, fmt.Sprintf("%s: snapshot (none), recomputed %+v", g.AccountID, g))
		}
	}
	return out
}

// snapshotLogs はスナップショットのログを、連合先から取得したログとして渡す（fedapp.PeerClient）。
type snapshotLogs struct {
	keys   map[string]string
	events map[string][]feddomain.Event
}

func (l snapshotLogs) InstanceInfo(_ context.Context, peer string) (fedapp.InstanceInfo, error) {
	return fedapp.InstanceInfo{InstanceID: peer, PublicKey: l.keys[peer]}, nil
}

func (l snapshotLogs) Log(_ context.Context, peer string, since int64, limit int) ([]feddomain.Event, error) {
	evs := l.events[peer]
	// イベントは seq の昇順に並んでいる前提。並んでいなければ、取り込みのハッシュチェーンの検証で失敗する
	i := sort.Search(len(evs), func(i int) bool { return evs[i].Seq > since })
	return evs[i:min(i+limit, len(evs))], nil
}

// noTx はトランザクションを持たない TxManager（メモリ上の Store 用）。
type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// fixedClock は常に同じ時刻を返す Clock。
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }
