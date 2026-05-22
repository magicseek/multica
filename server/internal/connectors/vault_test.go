package connectors

import (
	"bytes"
	"testing"
)

func TestCredentialVaultEncryptDecrypt(t *testing.T) {
	key := bytes.Repeat([]byte{7}, credentialKeySize)
	vault, err := NewCredentialVault("test-key", key)
	if err != nil {
		t.Fatalf("NewCredentialVault: %v", err)
	}

	associatedData := []byte("workspace/provider/user")
	secret, err := vault.Encrypt("pat-secret", associatedData)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if string(secret.Ciphertext) == "pat-secret" {
		t.Fatalf("ciphertext should not contain plaintext")
	}
	if len(secret.Nonce) != credentialNonceSize {
		t.Fatalf("nonce length = %d, want %d", len(secret.Nonce), credentialNonceSize)
	}

	plaintext, err := vault.Decrypt(secret, associatedData)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if plaintext != "pat-secret" {
		t.Fatalf("plaintext = %q, want pat-secret", plaintext)
	}
}

func TestCredentialVaultRejectsWrongAssociatedData(t *testing.T) {
	key := bytes.Repeat([]byte{9}, credentialKeySize)
	vault, err := NewCredentialVault("test-key", key)
	if err != nil {
		t.Fatalf("NewCredentialVault: %v", err)
	}
	secret, err := vault.Encrypt("pat-secret", []byte("workspace/a"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := vault.Decrypt(secret, []byte("workspace/b")); err == nil {
		t.Fatalf("Decrypt should reject wrong associated data")
	}
}
