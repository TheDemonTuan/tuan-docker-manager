package database

import (
	"docker-panel/internal/models"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	tempDir, err := os.MkdirTemp("", "db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to initialize db: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

func TestDatabase_StackRevisions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert stack first to satisfy foreign key
	err := db.UpsertStack(&models.Stack{
		ID:        "stack-123",
		Name:      "production-app",
		Status:    models.StackStatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to insert stack: %v", err)
	}

	rev1, err := db.CreateRevision(
		"stack-123",
		"user-1",
		"admin@example.com",
		"version: '3.8'\nservices:\n  web:\n    image: nginx:latest\n",
		"PORT=80",
		"Initial stack deployment",
	)
	if err != nil {
		t.Fatalf("failed to create rev1: %v", err)
	}

	if rev1.ID == 0 {
		t.Errorf("expected non-zero ID")
	}

	rev2, err := db.CreateRevision(
		"stack-123",
		"user-1",
		"admin@example.com",
		"version: '3.8'\nservices:\n  web:\n    image: nginx:1.27-alpine\n",
		"PORT=8080",
		"Upgrade to alpine",
	)
	if err != nil {
		t.Fatalf("failed to create rev2: %v", err)
	}

	list, err := db.GetRevisionsForStack("stack-123")
	if err != nil {
		t.Fatalf("failed to list revisions: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(list))
	}

	singleRev, err := db.GetRevisionByID(rev2.ID)
	if err != nil {
		t.Fatalf("failed to get single revision: %v", err)
	}

	if singleRev.Message != "Upgrade to alpine" {
		t.Errorf("expected message Upgrade to alpine, got %s", singleRev.Message)
	}
}

func TestDatabase_AuditLogs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.CreateAuditLog(
		"operator@example.com",
		"CONTAINER_RESTART",
		"container/web-server-1",
		"192.168.1.50",
		`{"reason":"maintenance"}`,
		"SUCCESS",
	)
	if err != nil {
		t.Fatalf("failed to log audit entry: %v", err)
	}

	entries, err := db.GetAuditLogs(10)
	if err != nil {
		t.Fatalf("failed to query audit logs: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].UserEmail != "operator@example.com" {
		t.Errorf("expected operator email, got %s", entries[0].UserEmail)
	}
	if entries[0].Action != "CONTAINER_RESTART" {
		t.Errorf("expected CONTAINER_RESTART, got %s", entries[0].Action)
	}
}

func TestDatabase_Jobs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	job := &models.Job{
		ID:        "job-12345",
		Type:      "compose_up",
		StackID:   "my-stack",
		Status:    models.JobStatusPending,
		CreatedBy: "admin@example.com",
		StartedAt: time.Now(),
	}

	if err := db.CreateJob(job); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	fetched, err := db.GetJobByID("job-12345")
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if fetched.Status != models.JobStatusPending {
		t.Errorf("expected pending, got %s", fetched.Status)
	}

	job.Status = models.JobStatusCompleted
	job.Logs = "All containers started smoothly"
	if err := db.UpdateJob(job); err != nil {
		t.Fatalf("failed to update job status: %v", err)
	}

	updated, err := db.GetJobByID("job-12345")
	if err != nil {
		t.Fatalf("failed to get updated job: %v", err)
	}
	if updated.Status != models.JobStatusCompleted {
		t.Errorf("expected completed, got %s", updated.Status)
	}
	if updated.Logs != "All containers started smoothly" {
		t.Errorf("expected log output, got %q", updated.Logs)
	}
}
