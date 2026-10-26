package crypto

import (
	"bytes"
	"testing"
)

func TestDIDKey_RoundTrip(t *testing.T) {
	pub, priv := GenerateKey()
	did := DIDKey(pub)
	if did[:9] != "did:key:z" {
		t.Fatalf("did = %s", did)
	}
	got, err := PublicKeyFromDID(did)
	if err != nil || !bytes.Equal(got, pub) {
		t.Fatalf("公開鍵を取り出せない: %v", err)
	}
	sig := Sign(priv, []byte("hello"))
	if !VerifyDID(did, []byte("hello"), sig) {
		t.Error("署名を検証できない")
	}
	if VerifyDID(did, []byte("tampered"), sig) {
		t.Error("改ざんした内容の署名を受け付けた")
	}
}

func TestSealer(t *testing.T) {
	s, err := NewSealer([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	sealed := s.Seal([]byte("secret"))
	if bytes.Contains(sealed, []byte("secret")) {
		t.Fatal("平文が含まれている")
	}
	plain, err := s.Open(sealed)
	if err != nil || string(plain) != "secret" {
		t.Fatalf("復号できない: %v", err)
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := s.Open(sealed); err == nil {
		t.Error("改ざんした暗号文を復号できた")
	}
}
