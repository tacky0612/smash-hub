package domain

import "time"

// RulesetStandard は標準ルール（docs/domain/match-rules.md）の ID とバージョン。
const RulesetStandard = "standard-1on1@1"

// InstanceID はインスタンスの識別子（ベース URL）。
type InstanceID string

// PlayerRef は対戦記録の参加者。表示名など変更されうる情報は持たない。
type PlayerRef struct {
	AccountID AccountID `json:"account_id"`
}

// Pick は1ゲームで使ったファイター。Characters は Players と同じ順番。
type Pick struct {
	Game       int         `json:"game"`
	Characters []FighterID `json:"characters"`
}

// MatchStatement は両プレイヤーが署名する対戦結果の内容（MatchRecorded の payload から署名を除いたもの）。
type MatchStatement struct {
	MatchID      MatchID     `json:"match_id"`
	HostInstance InstanceID  `json:"host_instance"`
	PlayedAt     string      `json:"played_at"` // RFC 3339（UTC、ミリ秒まで）
	Ruleset      string      `json:"ruleset"`
	Players      []PlayerRef `json:"players"` // プレイヤー1、プレイヤー2 の順
	Picks        []Pick      `json:"picks"`
	Winner       int         `json:"winner"` // Players の添字
}

// FormatTime は署名対象に含める時刻の形式。
func FormatTime(t time.Time) string {
	return t.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z")
}

// Bytes は署名対象のバイト列を返す。
func (s MatchStatement) Bytes() []byte {
	b, err := Canonical(s)
	if err != nil {
		panic(err) // 構造体の JSON 化は失敗しない
	}
	return b
}

// PlayerSignature はプレイヤーの署名。
type PlayerSignature struct {
	AccountID AccountID `json:"account_id"`
	Sig       string    `json:"sig"`
}

// MatchRecord は確定した対戦記録（連合で共有される MatchRecorded の payload）。
type MatchRecord struct {
	MatchStatement
	Signatures []PlayerSignature `json:"signatures"`
}
