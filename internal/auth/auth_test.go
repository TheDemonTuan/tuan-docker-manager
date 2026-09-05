package auth

import (
	"docker-panel/internal/database"
	"docker-panel/internal/models"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) (*database.DB, func()) {
	tempDir, err := os.MkdirTemp("", "auth-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to initialize db: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

func TestAuth_CloudflareAccessHeader(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	authenticator := NewAuthenticator(
		db,
		[]string{"alice@company.com", "bob@company.com"},
		"bob@company.com",
		false, // devMode disabled
	)

	// Authorized email
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Cf-Access-Authenticated-User-Email", "alice@company.com")

	user, err := authenticator.Authenticate(req)
	if err != nil {
		t.Fatalf("expected authentication to succeed: %v", err)
	}
	if user.Email != "alice@company.com" {
		t.Errorf("expected alice@company.com, got %s", user.Email)
	}

	// Admin email gets Owner role
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	reqAdmin.Header.Set("Cf-Access-Authenticated-User-Email", "bob@company.com")

	adminUser, err := authenticator.Authenticate(reqAdmin)
	if err != nil {
		t.Fatalf("expected admin authentication to succeed: %v", err)
	}
	if adminUser.Role != models.RoleOwner {
		t.Errorf("expected owner role for admin email, got %s", adminUser.Role)
	}

	// Unauthorized email not in allowlist
	reqForbidden := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	reqForbidden.Header.Set("Cf-Access-Authenticated-User-Email", "hacker@evil.com")

	_, err = authenticator.Authenticate(reqForbidden)
	if err == nil {
		t.Errorf("expected failure for non-allowlisted email")
	}
}

func TestAuth_DevMode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	authenticator := NewAuthenticator(
		db,
		nil,
		"local-dev@example.com",
		true, // devMode enabled
	)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	user, err := authenticator.Authenticate(req)
	if err != nil {
		t.Fatalf("expected dev auth to succeed: %v", err)
	}
	if user.Email != "local-dev@example.com" {
		t.Errorf("expected local-dev@example.com, got %s", user.Email)
	}
	if user.Role != models.RoleOwner {
		t.Errorf("expected owner role in dev mode, got %s", user.Role)
	}
}

func TestAuth_OriginValidation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	auth := NewAuthenticator(db, nil, "admin@example.com", true)

	// Valid origin matching host
	reqValid := httptest.NewRequest(http.MethodPost, "http://panel.example.com/api/stacks/up", nil)
	reqValid.Host = "panel.example.com"
	reqValid.Header.Set("Origin", "http://panel.example.com")
	if !auth.validateOrigin(reqValid) {
		t.Errorf("expected valid origin to pass")
	}

	// Cross-site attack origin
	reqCSRF := httptest.NewRequest(http.MethodPost, "http://panel.example.com/api/stacks/up", nil)
	reqCSRF.Host = "panel.example.com"
	reqCSRF.Header.Set("Origin", "http://evil-attacker.com")
	if auth.validateOrigin(reqCSRF) {
		t.Errorf("expected evil origin to fail validation")
	}
}
