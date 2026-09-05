package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"docker-panel/internal/models"
	"docker-panel/internal/secrets"
)

func TestBackupManager_CreateAndListWithStacksProvider(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "panel.db")
	if err := os.WriteFile(dbPath, []byte("sqlite-db-mock-content"), 0600); err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}

	keyPath := filepath.Join(tempDir, "master.key")
	secretsMgr, err := secrets.NewManager(keyPath)
	if err != nil {
		t.Fatalf("failed to init secrets manager: %v", err)
	}

	backupDir := filepath.Join(tempDir, "backups")

	// Mock stacks provider (e.g. db.GetAllStacks)
	stacksProvider := func() ([]*models.Stack, error) {
		return []*models.Stack{
			{
				Name:           "web-app",
				ComposeContent: "services:\n  web:\n    image: nginx\n",
				EnvContent:     "PORT=80\nDEBUG=0\n",
			},
		}, nil
	}

	mgr := NewManager(dbPath, "", backupDir, secretsMgr, stacksProvider)

	record, err := mgr.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}
	if record.Filename == "" || record.SizeBytes == 0 {
		t.Errorf("invalid backup record: %+v", record)
	}

	// Verify backup can be decrypted and extracted
	backupFile := filepath.Join(backupDir, record.Filename)
	encData, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}

	// First 24 bytes (XChaCha20-Poly1305 nonce)
	nonce := encData[:24]
	ciphertext := encData[24:]

	plaintext, err := secretsMgr.Decrypt(ciphertext, nonce)
	if err != nil {
		t.Fatalf("failed to decrypt backup: %v", err)
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(plaintext))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	foundFiles := make(map[string]string)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar reading failed: %v", err)
		}
		content, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("failed to read file from tar: %v", err)
		}
		foundFiles[header.Name] = string(content)
	}

	if dbContent, ok := foundFiles["panel.db"]; !ok || dbContent != "sqlite-db-mock-content" {
		t.Errorf("panel.db missing or content mismatch in backup: %v", foundFiles)
	}
	if composeContent, ok := foundFiles["stacks/web-app/compose.yaml"]; !ok || composeContent != "services:\n  web:\n    image: nginx\n" {
		t.Errorf("stacks/web-app/compose.yaml missing or mismatch: %v", foundFiles)
	}
	if envContent, ok := foundFiles["stacks/web-app/.env"]; !ok || envContent != "PORT=80\nDEBUG=0\n" {
		t.Errorf("stacks/web-app/.env missing or mismatch: %v", foundFiles)
	}

	// Verify ListBackups
	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(backups) != 1 || backups[0].Filename != record.Filename {
		t.Errorf("unexpected ListBackups result: %+v", backups)
	}
}
