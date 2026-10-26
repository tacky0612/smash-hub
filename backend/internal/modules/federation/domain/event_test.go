package domain

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

type keyPair struct {
	did  string
	sign Signer
}

func newKey() keyPair {
	pub, priv := crypto.GenerateKey()
	return keyPair{did: crypto.DIDKey(pub), sign: func(m []byte) string { return crypto.Sign(priv, m) }}
}

const host = "https://a.example"

func signedRecord(alice, bob keyPair) shared.MatchRecord {
	st := shared.MatchStatement{
		MatchID: "m1", HostInstance: host, PlayedAt: "2026-11-01T20:00:00.000Z", Ruleset: shared.RulesetStandard,
		Players: []shared.PlayerRef{{AccountID: shared.AccountID(alice.did)}, {AccountID: shared.AccountID(bob.did)}},
		Picks:   []shared.Pick{{Game: 1, Characters: []shared.FighterID{"mario", "fox"}}},
		Winner:  0,
	}
	return shared.MatchRecord{MatchStatement: st, Signatures: []shared.PlayerSignature{
		{AccountID: shared.AccountID(alice.did), Sig: alice.sign(st.Bytes())},
		{AccountID: shared.AccountID(bob.did), Sig: bob.sign(st.Bytes())},
	}}
}

func TestMatchRecorded_RoundTripAndTamperDetection(t *testing.T) {
	inst, alice, bob := newKey(), newKey(), newKey()
	e, err := NewEvent(TypeMatchRecorded, host, 1, "", "2026-11-01T20:10:00.000Z", signedRecord(alice, bob), inst.sign)
	if err != nil {
		t.Fatal(err)
	}
	instPub, _ := crypto.PublicKeyFromDID(inst.did)
	verifyInst := func(m []byte, s string) bool { return crypto.Verify(instPub, m, s) }

	// 配信されたバイト列から復元しても検証できる
	var received Event
	if err := json.Unmarshal(e.Body(), &received); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChain(received, host, 0, "", verifyInst); err != nil {
		t.Fatalf("正しいイベントを拒否した: %v", err)
	}
	if _, err := DecodeMatchRecorded(received, crypto.VerifyDID); err != nil {
		t.Fatalf("正しい記録を拒否した: %v", err)
	}

	// 勝者を書き換えると、インスタンス署名もプレイヤー署名も合わなくなる
	tampered := received
	var rec shared.MatchRecord
	_ = json.Unmarshal(received.Payload, &rec)
	rec.Winner = 1
	tampered.Payload, _ = shared.Canonical(rec)
	if err := VerifyChain(tampered, host, 0, "", verifyInst); !errors.Is(err, ErrBadInstanceSignature) {
		t.Errorf("改ざんしたイベントのインスタンス署名を受け付けた: %v", err)
	}
	if _, err := DecodeMatchRecorded(tampered, crypto.VerifyDID); err == nil {
		t.Error("改ざんした記録のプレイヤー署名を受け付けた")
	}

	// 連番の飛び・prev の不一致を検出する
	if err := VerifyChain(received, host, 1, "", verifyInst); !errors.Is(err, ErrBrokenChain) {
		t.Errorf("連番の不一致を検出しない: %v", err)
	}
	if err := VerifyChain(received, "https://other.example", 0, "", verifyInst); !errors.Is(err, ErrUnexpectedOrigin) {
		t.Errorf("発行元の不一致を検出しない: %v", err)
	}
}

func TestMatchRecorded_RejectsForeignHostAndMissingSignature(t *testing.T) {
	inst, alice, bob := newKey(), newKey(), newKey()
	rec := signedRecord(alice, bob)
	rec.Signatures = rec.Signatures[:1]
	e, _ := NewEvent(TypeMatchRecorded, host, 1, "", "2026-11-01T20:10:00.000Z", rec, inst.sign)
	if _, err := DecodeMatchRecorded(e, crypto.VerifyDID); err == nil {
		t.Error("署名が1人分の記録を受け付けた")
	}
	e, _ = NewEvent(TypeMatchRecorded, "https://other.example", 1, "", "2026-11-01T20:10:00.000Z", signedRecord(alice, bob), inst.sign)
	if _, err := DecodeMatchRecorded(e, crypto.VerifyDID); err == nil {
		t.Error("ホスト以外が発行した記録を受け付けた")
	}
}

func TestAccountPublished_RequiresAccountSignature(t *testing.T) {
	inst, alice, mallory := newKey(), newKey(), newKey()
	a := AccountPublished{AccountID: shared.AccountID(alice.did), DisplayName: "alice", HomeInstance: host}
	a.AccountSig = alice.sign(a.SigningBytes())
	e, _ := NewEvent(TypeAccountPublished, host, 1, "", "2026-11-01T20:10:00.000Z", a, inst.sign)
	if _, err := DecodeAccountPublished(e, crypto.VerifyDID); err != nil {
		t.Fatalf("正しいイベントを拒否した: %v", err)
	}
	// 他人（mallory）の鍵で alice を名乗るイベントは拒否する
	a.AccountSig = mallory.sign(a.SigningBytes())
	e, _ = NewEvent(TypeAccountPublished, host, 1, "", "2026-11-01T20:10:00.000Z", a, inst.sign)
	if _, err := DecodeAccountPublished(e, crypto.VerifyDID); err == nil {
		t.Error("他人の鍵で署名したアカウントを受け付けた")
	}
}
