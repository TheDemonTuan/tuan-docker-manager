package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"docker-panel/internal/agent"
	"docker-panel/internal/alerts"
	"docker-panel/internal/audit"
	"docker-panel/internal/auth"
	"docker-panel/internal/backup"
	"docker-panel/internal/compose"
	"docker-panel/internal/database"
	"docker-panel/internal/events"
	"docker-panel/internal/jobs"
	"docker-panel/internal/models"
	"docker-panel/internal/secrets"
	"docker-panel/internal/security"
)

func setupTestServer(t *testing.T) (*Server, func()) {
	tempDir, err := os.MkdirTemp("", "api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "panel.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	keyPath := filepath.Join(tempDir, "master.key")
	secretsMgr, err := secrets.NewManager(keyPath)
	if err != nil {
		t.Fatalf("failed to init secrets: %v", err)
	}

	// Mock Agent backend
	agentMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/ping":
			_ = json.NewEncoder(w).Encode(agent.PingResponse{
				Status:        "ok",
				Version:       "0.1.0-mvp",
				DockerVersion: "27.2.0",
				Hostname:      "test-host",
			})
		case "/actions/containers/list":
			_ = json.NewEncoder(w).Encode(agent.ListContainersResponse{Containers: []models.ContainerInfo{}})
		case "/actions/images/list":
			_ = json.NewEncoder(w).Encode([]models.ImageInfo{})
		case "/actions/volumes/list":
			_ = json.NewEncoder(w).Encode([]models.VolumeInfo{})
		case "/actions/networks/list":
			_ = json.NewEncoder(w).Encode([]models.NetworkInfo{})
		case "/actions/compose/action":
			_ = json.NewEncoder(w).Encode(agent.ComposeActionResponse{
				Success: true,
				Logs:    "Compose action executed",
			})
		case "/actions/compose/discover":
			_ = json.NewEncoder(w).Encode(agent.DiscoverStacksResponse{
				Stacks: []compose.DiscoveredStack{},
			})
		case "/metrics/host":
			_ = json.NewEncoder(w).Encode(models.HostMetrics{
				CPUPercent:    15.5,
				MemoryUsed:    4294967296,
				MemoryTotal:   17179869184,
				MemoryPercent: 25.0,
			})
		case "/metrics/gpu":
			_ = json.NewEncoder(w).Encode(models.GPUMetrics{Available: false})
		default:
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}
	}))

	agentClient := agent.NewClient("tcp://" + agentMock.Listener.Addr().String())
	jobMgr := jobs.NewManager(db, agentClient)
	scanner := security.NewScanner([]string{"/srv/docker-panel/stacks"})
	authenticator := auth.NewAuthenticator(db, []string{"admin@example.com"}, "admin@example.com", false)
	auditLogger := audit.NewLogger(db)
	alertEngine := alerts.NewEngine(db)
	stacksDir := filepath.Join(tempDir, "stacks")
	backupMgr := backup.NewManager(dbPath, stacksDir, filepath.Join(tempDir, "backups"), secretsMgr)
	eventBus := events.NewBus()
	dummyFS := fstest.MapFS{
		"index.html": {Data: []byte("<html><body>Mock Panel UI</body></html>")},
	}

	server := NewServer(
		db,
		agentClient,
		jobMgr,
		scanner,
		authenticator,
		auditLogger,
		alertEngine,
		backupMgr,
		eventBus,
		dummyFS,
		stacksDir,
	)

	cleanup := func() {
		agentMock.Close()
		db.Close()
		os.RemoveAll(tempDir)
	}

	return server, cleanup
}

func TestAPI_Health(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected health endpoint status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAPI_AuthMe(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	handler := server.Routes()

	// 1. Unauthorized request
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rrUnauth := httptest.NewRecorder()
	handler.ServeHTTP(rrUnauth, reqUnauth)
	if rrUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rrUnauth.Code)
	}

	// 2. Authorized request via Cloudflare header
	reqAuth := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	reqAuth.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	rrAuth := httptest.NewRecorder()
	handler.ServeHTTP(rrAuth, reqAuth)
	if rrAuth.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d: %s", rrAuth.Code, rrAuth.Body.String())
	}

	var authResp struct {
		User        models.User `json:"user"`
		Permissions []string    `json:"permissions"`
	}
	if err := json.Unmarshal(rrAuth.Body.Bytes(), &authResp); err != nil {
		t.Fatalf("failed to decode user response: %v", err)
	}
	if authResp.User.Email != "admin@example.com" || authResp.User.Role != models.RoleOwner {
		t.Errorf("unexpected user: %+v", authResp.User)
	}
}

func TestAPI_DashboardSummary(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	handler := server.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	req.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var dash DashboardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &dash); err != nil {
		t.Fatalf("failed to decode dashboard response: %v", err)
	}

	if dash.Host == nil || dash.Host.CPUPercent != 15.5 {
		t.Errorf("unexpected host metrics: %+v", dash.Host)
	}
}

func TestAPI_CreateStack(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	handler := server.Routes()

	body := []byte(`{
		"name": "blog",
		"compose_content": "services:\n  web:\n    image: ghost:alpine\n    ports:\n      - 127.0.0.1:2368:2368\n",
		"env_content": "NODE_ENV=production"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	req.Host = "example.com"
	req.Header.Set("Origin", "http://example.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rr.Code, rr.Body.String())
	}

	var stack models.Stack
	if err := json.Unmarshal(rr.Body.Bytes(), &stack); err != nil {
		t.Fatalf("failed to decode created stack: %v", err)
	}

	if stack.Name != "blog" || stack.SecurityScore < 50 {
		t.Errorf("unexpected created stack: %+v", stack)
	}
}

func TestAPI_DeleteStack(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	handler := server.Routes()

	// 1. Create a stack first in DB
	stk := &models.Stack{
		ID:             "stk_to_delete",
		Name:           "temp-stack",
		Status:         models.StackStatusStopped,
		Path:           "/srv/docker-panel/stacks/temp-stack",
		ComposeContent: "services:\n  app:\n    image: alpine\n",
		IsSystem:       false,
	}
	if err := server.db.UpsertStack(stk); err != nil {
		t.Fatalf("failed to insert test stack: %v", err)
	}

	// 2. Delete request with critical confirm
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stacks/stk_to_delete", nil)
	req.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	req.Header.Set("X-Critical-Confirm", "1")
	req.Host = "example.com"
	req.Header.Set("Origin", "http://example.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Verify stack is deleted from DB
	deleted, _ := server.db.GetStackByID("stk_to_delete")
	if deleted != nil {
		t.Errorf("expected stack to be deleted from database, got %+v", deleted)
	}
}
