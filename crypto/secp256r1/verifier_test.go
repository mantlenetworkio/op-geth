package secp256r1

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerify(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	digest := sha256.Sum256([]byte("test data"))

	r, s, err := ecdsa.Sign(rand.Reader, privKey, digest[:])
	if err != nil {
		panic(err)
	}

	res := Verify(digest[:], r, s, privKey.PublicKey.X, privKey.PublicKey.Y)
	assert.Equal(t, res, true)
}

func TestGenerateSecp256r1Input(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	digest := sha256.Sum256([]byte("test data"))

	r, s, err := ecdsa.Sign(rand.Reader, privKey, digest[:])
	if err != nil {
		panic(err)
	}

	input := make([]byte, 160)
	copy(input[:32], digest[:])
	copy(input[32:64], r.Bytes())
	copy(input[64:96], s.Bytes())
	copy(input[96:128], privKey.PublicKey.X.Bytes())
	copy(input[128:160], privKey.PublicKey.Y.Bytes())
	fmt.Printf("digest: %x\n", input[:32])
	fmt.Printf("sign: %x\n", input[32:96])
	fmt.Printf("pub key: %x\n", input[96:160])
	fmt.Printf("input: %x\n", input)
}
