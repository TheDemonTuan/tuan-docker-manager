package secrets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCrypto_EncryptDecrypt(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPath := filepath.Join(tempDir, "master.key")
	mgr, err := NewManager(keyPath)
	if err != nil {
		t.Fatalf("failed to initialize secrets manager: %v", err)
	}

	plaintext := []byte("DATABASE_PASSWORD=super-secret-production-password-12345!")
	ciphertext, nonce, err := mgr.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Errorf("ciphertext must not match plaintext")
	}

	decrypted, err := mgr.Decrypt(ciphertext, nonce)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("expected decrypted %q, got %q", plaintext, decrypted)
	}
}

func TestCrypto_TamperedCiphertext(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPath := filepath.Join(tempDir, "master.key")
	mgr, err := NewManager(keyPath)
	if err != nil {
		t.Fatalf("failed to init secrets manager: %v", err)
	}

	ciphertext, nonce, err := mgr.Encrypt([]byte("my-secret-data"))
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// Tamper with ciphertext
	raw := make([]byte, len(ciphertext))
	copy(raw, ciphertext)
	if len(raw) > 5 {
		raw[len(raw)-3] ^= 0xFF
	}

	_, err = mgr.Decrypt(raw, nonce)
	if err == nil {
		t.Errorf("expected decryption to fail for tampered ciphertext")
	}
}

func TestCrypto_ExistingKeyPreserved(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPath := filepath.Join(tempDir, "master.key")
	mgr1, err := NewManager(keyPath)
	if err != nil {
		t.Fatalf("failed to init mgr1: %v", err)
	}

	mgr2, err := NewManager(keyPath)
	if err != nil {
		t.Fatalf("failed to init mgr2: %v", err)
	}

	if !bytes.Equal(mgr1.Key(), mgr2.Key()) {
		t.Errorf("manager reopened with existing key should load identical key bytes")
	}
}
