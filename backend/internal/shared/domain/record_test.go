package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func statement(host InstanceID, matchID MatchID) MatchStatement {
	return MatchStatement{
		MatchID: matchID, HostInstance: host, PlayedAt: "2026-10-26T10:30:00.000Z", Ruleset: RulesetStandard,
		Players: []PlayerRef{{AccountID: "did:key:zA"}, {AccountID: "did:key:zB"}},
		Picks:   []Pick{{Game: 1, Characters: []FighterID{"mario", "fox"}}},
		Winner:  0,
	}
}

// 記録 ID は、両プレイヤーが署名する対戦結果の内容（正規化 JSON）の SHA-256 を小文字の16進で表したもの。
func TestRecordID_IsSHA256OfStatement(t *testing.T) {
	s := statement("https://a.example", "m1")
	sum := sha256.Sum256(s.Bytes())
	if got, want := s.RecordID(), hex.EncodeToString(sum[:]); got != want {
		t.Errorf("RecordID = %s, want %s", got, want)
	}
}

// 別のホストが同じ match_id を使っても、記録 ID は衝突しない。
func TestRecordID_DiffersByHost(t *testing.T) {
	if statement("https://a.example", "m1").RecordID() == statement("https://b.example", "m1").RecordID() {
		t.Error("ホストが違うのに記録 ID が同じ")
	}
}

// 署名は記録 ID に含めない（同じ対戦結果なら署名の表現によらず同じ ID）。
func TestRecordID_IgnoresSignatures(t *testing.T) {
	s := statement("https://a.example", "m1")
	r1 := MatchRecord{MatchStatement: s, Signatures: []PlayerSignature{{AccountID: "did:key:zA", Sig: "x"}}}
	r2 := MatchRecord{MatchStatement: s, Signatures: []PlayerSignature{{AccountID: "did:key:zA", Sig: "y"}}}
	if r1.RecordID() != r2.RecordID() {
		t.Error("署名の違いで記録 ID が変わった")
	}
}
