package connectors

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

const (
	credentialKeySize   = 32
	credentialNonceSize = 12
	defaultKeyID        = "env:v1"
)

type CredentialVault struct {
	keyID string
	aead  cipher.AEAD
}

type EncryptedSecret struct {
	Ciphertext []byte
	Nonce      []byte
	KeyID      string
}

func NewCredentialVaultFromKeyString(raw string) (*CredentialVault, error) {
	key, err := decodeCredentialKey(raw)
	if err != nil {
		return nil, err
	}
	return NewCredentialVault(defaultKeyID, key)
}

func NewCredentialVault(keyID string, key []byte) (*CredentialVault, error) {
	if keyID == "" {
		return nil, errors.New("connector credential key id is required")
	}
	if len(key) != credentialKeySize {
		return nil, errors.New("connector credential key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &CredentialVault{keyID: keyID, aead: aead}, nil
}

func (v *CredentialVault) Encrypt(plaintext string, associatedData []byte) (EncryptedSecret, error) {
	if v == nil {
		return EncryptedSecret{}, errors.New("connector credential vault is not configured")
	}
	nonce := make([]byte, credentialNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptedSecret{}, err
	}
	ciphertext := v.aead.Seal(nil, nonce, []byte(plaintext), associatedData)
	return EncryptedSecret{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		KeyID:      v.keyID,
	}, nil
}

func (v *CredentialVault) Decrypt(secret EncryptedSecret, associatedData []byte) (string, error) {
	if v == nil {
		return "", errors.New("connector credential vault is not configured")
	}
	if secret.KeyID != "" && secret.KeyID != v.keyID {
		return "", errors.New("connector credential key id mismatch")
	}
	plaintext, err := v.aead.Open(nil, secret.Nonce, secret.Ciphertext, associatedData)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
