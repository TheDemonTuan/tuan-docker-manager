package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"docker-panel/internal/models"
	"docker-panel/internal/secrets"
)

type Manager struct {
	dbPath     string
	stacksDir  string
	backupDir  string
	secretsMgr *secrets.Manager
}

func NewManager(dbPath, stacksDir, backupDir string, secretsMgr *secrets.Manager) *Manager {
	if backupDir == "" {
		backupDir = "/srv/docker-panel/backups"
	}
	_ = os.MkdirAll(backupDir, 0700)

	return &Manager{
		dbPath:     dbPath,
		stacksDir:  stacksDir,
		backupDir:  backupDir,
		secretsMgr: secretsMgr,
	}
}

func (m *Manager) CreateBackup() (*models.BackupRecord, error) {
	now := time.Now()
	timestamp := now.Format("20060102_150405")
	filename := fmt.Sprintf("backup_%s.tar.gz.enc", timestamp)
	targetPath := filepath.Join(m.backupDir, filename)

	var tarBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&tarBuf)
	tarWriter := tar.NewWriter(gzWriter)

	// 1. Add SQLite DB file
	if dbData, err := os.ReadFile(m.dbPath); err == nil {
		hdr := &tar.Header{
			Name: "panel.db",
			Mode: 0600,
			Size: int64(len(dbData)),
		}
		if err := tarWriter.WriteHeader(hdr); err == nil {
			_, _ = tarWriter.Write(dbData)
		}
	}

	// 2. Add Stacks Compose and .env files
	if m.stacksDir != "" {
		_ = filepath.Walk(m.stacksDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			base := filepath.Base(path)
			if strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") || base == ".env" {
				rel, relErr := filepath.Rel(m.stacksDir, path)
				if relErr == nil {
					data, readErr := os.ReadFile(path)
					if readErr == nil {
						hdr := &tar.Header{
							Name: filepath.Join("stacks", rel),
							Mode: 0644,
							Size: int64(len(data)),
						}
						if err := tarWriter.WriteHeader(hdr); err == nil {
							_, _ = tarWriter.Write(data)
						}
					}
				}
			}
			return nil
		})
	}

	_ = tarWriter.Close()
	_ = gzWriter.Close()

	// Encrypt archive with master key
	ciphertext, nonce, err := m.secretsMgr.Encrypt(tarBuf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("backup encryption failed: %w", err)
	}

	// Write encrypted archive
	var finalPayload bytes.Buffer
	finalPayload.Write(nonce)
	finalPayload.Write(ciphertext)

	if err := os.WriteFile(targetPath, finalPayload.Bytes(), 0600); err != nil {
		return nil, fmt.Errorf("failed to save encrypted backup: %w", err)
	}

	return &models.BackupRecord{
		ID:        fmt.Sprintf("bk_%d", now.UnixNano()),
		Filename:  filename,
		SizeBytes: int64(finalPayload.Len()),
		Encrypted: true,
		CreatedAt: now,
	}, nil
}

func (m *Manager) ListBackups() ([]models.BackupRecord, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var records []models.BackupRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".enc") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		records = append(records, models.BackupRecord{
			ID:        e.Name(),
			Filename:  e.Name(),
			SizeBytes: info.Size(),
			Encrypted: true,
			CreatedAt: info.ModTime(),
		})
	}
	return records, nil
}
