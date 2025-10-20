package aes

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	assert.NoError(t, err)
	assert.Equal(t, 32, len(key), "Key should be 32 bytes (256 bits)")
}

func TestEncryptDecrypt(t *testing.T) {
	// Generate a key
	key, err := GenerateKey()
	assert.NoError(t, err)

	// Test data
	plaintext := []byte("Hello, this is a secret message!")

	// Encrypt
	ciphertext, err := Encrypt(plaintext, key)
	assert.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext, "Ciphertext should be different from plaintext")

	// Decrypt
	decrypted, err := Decrypt(ciphertext, key)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(plaintext, decrypted), "Decrypted text should match original plaintext")
}

func TestEncryptWithDifferentKeys(t *testing.T) {
	// Generate two different keys
	key1, err := GenerateKey()
	assert.NoError(t, err)
	key2, err := GenerateKey()
	assert.NoError(t, err)

	plaintext := []byte("Secret data")

	// Encrypt with key1
	ciphertext, err := Encrypt(plaintext, key1)
	assert.NoError(t, err)

	// Try to decrypt with key2 (should fail)
	_, err = Decrypt(ciphertext, key2)
	assert.Error(t, err, "Decryption with wrong key should fail")
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	key, err := GenerateKey()
	assert.NoError(t, err)

	// Too short ciphertext
	shortCiphertext := []byte("short")
	_, err = Decrypt(shortCiphertext, key)
	assert.Error(t, err, "Should fail with short ciphertext")
}

func TestEncryptEmptyData(t *testing.T) {
	key, err := GenerateKey()
	assert.NoError(t, err)

	// Empty plaintext
	plaintext := []byte{}
	ciphertext, err := Encrypt(plaintext, key)
	assert.NoError(t, err)

	// Decrypt
	decrypted, err := Decrypt(ciphertext, key)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(decrypted), "Decrypted empty data should be empty")
}

func TestEncryptLargeData(t *testing.T) {
	key, err := GenerateKey()
	assert.NoError(t, err)

	// Large plaintext (1 MB)
	plaintext := make([]byte, 1024*1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// Encrypt
	ciphertext, err := Encrypt(plaintext, key)
	assert.NoError(t, err)

	// Decrypt
	decrypted, err := Decrypt(ciphertext, key)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(plaintext, decrypted), "Large data should encrypt/decrypt correctly")
}
