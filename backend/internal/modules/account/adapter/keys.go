package adapter

import (
	"crypto/ed25519"

	"github.com/tacky0612/smash-hub/backend/internal/modules/account/application"
	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
	shared "github.com/tacky0612/smash-hub/backend/internal/shared/domain"
)

// Keys は application.KeyManager の実装。秘密鍵は鍵暗号化鍵（KEK）で暗号化して扱う。
type Keys struct {
	Sealer *crypto.Sealer
}

var _ application.KeyManager = Keys{}

// Generate は Ed25519 の鍵ペアを作り、did:key のアカウント ID と暗号化した秘密鍵を返す。
func (k Keys) Generate() (shared.AccountID, []byte, []byte) {
	pub, priv := crypto.GenerateKey()
	return shared.AccountID(crypto.DIDKey(pub)), pub, k.Sealer.Seal(priv.Seed())
}

// Sign は暗号化した秘密鍵を復号して msg に署名する。
func (k Keys) Sign(sealed, msg []byte) (string, error) {
	seed, err := k.Sealer.Open(sealed)
	if err != nil {
		return "", err
	}
	return crypto.Sign(ed25519.NewKeyFromSeed(seed), msg), nil
}
