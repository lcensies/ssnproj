package rsa

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateKeyPair(t *testing.T) {
	priv, pub := GenerateKeyPair(2048)
	// fmt.Println(string(PrivateKeyToBytes(priv)))
	// fmt.Println(string(PublicKeyToBytes(pub)))

	assert.Equal(t, len(PrivateKeyToBytes(priv)) > 0, true)
	assert.Equal(t, len(PublicKeyToBytes(pub)) > 0, true)
}

func TestBytesToPrivateKey(t *testing.T) {
	priv, _ := GenerateKeyPair(2048)
	privBytes := PrivateKeyToBytes(priv)
	privKey := BytesToPrivateKey(privBytes)
	assert.Equal(t, privKey.PublicKey, priv.PublicKey)
}

func TestBytesToPublicKey(t *testing.T) {
	_, pub := GenerateKeyPair(2048)
	pubBytes := PublicKeyToBytes(pub)
	pubKey := BytesToPublicKey(pubBytes)
	assert.Equal(t, pubKey, pub)
}
