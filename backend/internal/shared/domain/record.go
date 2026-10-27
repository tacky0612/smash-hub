package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"time"
)

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

// MaxGames は1回の対戦の最大ゲーム数（2本先取）。
const MaxGames = 3

// fighterIDPattern はファイター ID の書式（英小文字と _。docs/domain/fighters.md）。
var fighterIDPattern = regexp.MustCompile(`^[a-z_]+$`)

// ValidPicks は対戦記録の picks の形が正しいかを返す（連合から取り込む記録の検証に使う）。
// 1〜MaxGames ゲームで、game は1からの連番、characters は2人分のファイター ID の書式の値。
// random（おまかせ）は、ゲームごとのファイターがそろわなかった記録（1戦目だけの記録）にだけ認める
// （docs/domain/fighters.md）。
// ファイター ID が自インスタンスのファイター一覧にあるかは確かめない。新しいファイターの ID を追加した
// バージョンのインスタンスの記録も取り込めるようにするため（未知の ID は表示できないだけ）。
func ValidPicks(picks []Pick) bool {
	if len(picks) < 1 || len(picks) > MaxGames {
		return false
	}
	for i, p := range picks {
		if p.Game != i+1 || len(p.Characters) != 2 {
			return false
		}
		for _, c := range p.Characters {
			if !fighterIDPattern.MatchString(string(c)) || (c == FighterRandom && len(picks) > 1) {
				return false
			}
		}
	}
	return true
}

// SamePicks は2つの picks が同じ内容かを返す。
func SamePicks(a, b []Pick) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Game != b[i].Game || len(a[i].Characters) != len(b[i].Characters) {
			return false
		}
		for j := range a[i].Characters {
			if a[i].Characters[j] != b[i].Characters[j] {
				return false
			}
		}
	}
	return true
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

// RecordID は記録 ID（対戦結果の内容の正規化 JSON の SHA-256 を小文字の16進で表したもの）。
// 連合全体で記録を一意に識別し、同時刻の記録を並べる順番にも使う（docs/architecture/federation.md 12.1）。
func (s MatchStatement) RecordID() string {
	sum := sha256.Sum256(s.Bytes())
	return hex.EncodeToString(sum[:])
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
