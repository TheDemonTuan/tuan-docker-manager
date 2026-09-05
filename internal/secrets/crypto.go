package secrets

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/chacha20poly1305"
)

type Manager struct {
	key []byte
}

func NewManager(keyPath string) (*Manager, error) {
	if keyPath == "" {
		keyPath = "./secrets/master.key"
	}

	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load/create master key: %w", err)
	}

	return &Manager{key: key}, nil
}

func loadOrCreateKey(keyPath string) ([]byte, error) {
	if data, err := os.ReadFile(keyPath); err == nil {
		if len(data) == chacha20poly1305.KeySize {
			return data, nil
		}
	}

	// Create new master key
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}

	// Write file with strict 0600 permissions
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, err
	}

	return key, nil
}

func (m *Manager) Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error) {
	aead, err := chacha20poly1305.NewX(m.key)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func (m *Manager) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, errors.New("empty ciphertext")
	}

	aead, err := chacha20poly1305.NewX(m.key)
	if err != nil {
		return nil, err
	}

	if len(nonce) != aead.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

func (m *Manager) Key() []byte {
	return m.key
}
