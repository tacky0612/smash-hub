package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// GenerateKey は Ed25519 の鍵ペアを生成する。
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return pub, priv
}

// PrivateKeyFromSeed は base64 で表した 32 バイトのシードから秘密鍵を復元する。
func PrivateKeyFromSeed(b64 string) (ed25519.PrivateKey, error) {
	seed, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("seed must be %d bytes", ed25519.SeedSize)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// ed25519 公開鍵の multicodec の接頭辞（0xed 0x01）。
var ed25519Multicodec = []byte{0xed, 0x01}

// DIDKey は公開鍵から did:key 形式のアカウント ID を作る（did:key:z + base58btc(multicodec + 公開鍵)）。
func DIDKey(pub ed25519.PublicKey) string {
	return "did:key:z" + base58Encode(append(append([]byte{}, ed25519Multicodec...), pub...))
}

// PublicKeyFromDID は did:key 形式の ID から公開鍵を取り出す。
func PublicKeyFromDID(did string) (ed25519.PublicKey, error) {
	rest, ok := strings.CutPrefix(did, "did:key:z")
	if !ok {
		return nil, errors.New("not a did:key")
	}
	b, err := base58Decode(rest)
	if err != nil {
		return nil, err
	}
	if len(b) != 2+ed25519.PublicKeySize || b[0] != ed25519Multicodec[0] || b[1] != ed25519Multicodec[1] {
		return nil, errors.New("not an ed25519 did:key")
	}
	return ed25519.PublicKey(b[2:]), nil
}

// VerifyDID は did:key の公開鍵で署名を検証する。
func VerifyDID(did string, msg []byte, sigB64 string) bool {
	pub, err := PublicKeyFromDID(did)
	if err != nil {
		return false
	}
	return Verify(pub, msg, sigB64)
}

// Sign は msg に署名し base64 で返す。
func Sign(priv ed25519.PrivateKey, msg []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, msg))
}

// Verify は base64 の署名を検証する。
func Verify(pub ed25519.PublicKey, msg []byte, sigB64 string) bool {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(pub, msg, sig)
}

// Sealer はアカウントの秘密鍵を DB に保存するための暗号化（AES-256-GCM）。
// 鍵暗号化鍵（KEK）は DB とは別に設定で渡す（docs/security/README.md）。
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer は秘密値から Sealer を作る。
func NewSealer(secret []byte) (*Sealer, error) {
	if len(secret) < 32 {
		return nil, errors.New("key encryption secret must be at least 32 bytes")
	}
	k := sha256.Sum256(secret)
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// Seal は平文を暗号化する（nonce を先頭に付ける）。
func (s *Sealer) Seal(plain []byte) []byte {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return s.aead.Seal(nonce, nonce, plain, nil)
}

// Open は Seal した値を復号する。
func (s *Sealer) Open(sealed []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("sealed value too short")
	}
	return s.aead.Open(nil, sealed[:n], sealed[n:], nil)
}

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func base58Encode(b []byte) string {
	n := new(big.Int).SetBytes(b)
	var out []byte
	mod, base := new(big.Int), big.NewInt(58)
	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		out = append(out, base58Alphabet[mod.Int64()])
	}
	for _, c := range b {
		if c != 0 {
			break
		}
		out = append(out, '1')
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

func base58Decode(s string) ([]byte, error) {
	n := new(big.Int)
	base := big.NewInt(58)
	for _, c := range s {
		i := strings.IndexRune(base58Alphabet, c)
		if i < 0 {
			return nil, errors.New("invalid base58")
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(i)))
	}
	out := n.Bytes()
	for _, c := range s {
		if c != '1' {
			break
		}
		out = append([]byte{0}, out...)
	}
	return out, nil
}
