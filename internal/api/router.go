package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"docker-panel/internal/agent"
	"docker-panel/internal/alerts"
	"docker-panel/internal/audit"
	"docker-panel/internal/auth"
	"docker-panel/internal/backup"
	"docker-panel/internal/database"
	"docker-panel/internal/events"
	"docker-panel/internal/jobs"
	"docker-panel/internal/security"
)

type Server struct {
	db          *database.DB
	agentClient *agent.Client
	jobManager  *jobs.Manager
	scanner     *security.ComposeScanner
	auth        *auth.Authenticator
	audit       *audit.Logger
	alerts      *alerts.Engine
	backups     *backup.Manager
	eventBus    *events.Bus
	staticFS    fs.FS
	stacksRoot  string
}

func NewServer(
	db *database.DB,
	agentClient *agent.Client,
	jobManager *jobs.Manager,
	scanner *security.ComposeScanner,
	authenticator *auth.Authenticator,
	auditLogger *audit.Logger,
	alertEngine *alerts.Engine,
	backupManager *backup.Manager,
	eventBus *events.Bus,
	staticFS fs.FS,
	stacksRoot string,
) *Server {
	return &Server{
		db:          db,
		agentClient: agentClient,
		jobManager:  jobManager,
		scanner:     scanner,
		auth:        authenticator,
		audit:       auditLogger,
		alerts:      alertEngine,
		backups:     backupManager,
		eventBus:    eventBus,
		staticFS:    staticFS,
		stacksRoot:  stacksRoot,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)

	// API Subrouter
	apiMux := http.NewServeMux()

	// Auth
	apiMux.HandleFunc("GET /api/v1/auth/me", s.handleAuthMe)

	// Dashboard
	apiMux.HandleFunc("GET /api/v1/dashboard", s.handleDashboard)

	// Stacks
	apiMux.HandleFunc("GET /api/v1/stacks", s.handleListStacks)
	apiMux.HandleFunc("POST /api/v1/stacks", s.handleCreateStack)
	apiMux.HandleFunc("GET /api/v1/stacks/{id}", s.handleGetStack)
	apiMux.HandleFunc("DELETE /api/v1/stacks/{id}", s.handleDeleteStack)
	apiMux.HandleFunc("GET /api/v1/stacks/{id}/compose", s.handleGetStackCompose)
	apiMux.HandleFunc("PUT /api/v1/stacks/{id}/compose", s.handleUpdateStackCompose)
	apiMux.HandleFunc("POST /api/v1/stacks/{id}/up", s.handleStackUp)
	apiMux.HandleFunc("POST /api/v1/stacks/{id}/down", s.handleStackDown)
	apiMux.HandleFunc("POST /api/v1/stacks/{id}/restart", s.handleStackRestart)
	apiMux.HandleFunc("POST /api/v1/stacks/{id}/pull", s.handleStackPull)
	apiMux.HandleFunc("POST /api/v1/stacks/{id}/build", s.handleStackBuild)
	apiMux.HandleFunc("GET /api/v1/stacks/{id}/security", s.handleStackSecurity)
	apiMux.HandleFunc("GET /api/v1/stacks/{id}/revisions", s.handleStackRevisions)
	apiMux.HandleFunc("POST /api/v1/stacks/{id}/revisions/{revId}/restore", s.handleRestoreRevision)
	apiMux.HandleFunc("GET /api/v1/stacks/{id}/preview", s.handleStackPreview)

	// Containers
	apiMux.HandleFunc("GET /api/v1/containers", s.handleListContainers)
	apiMux.HandleFunc("GET /api/v1/containers/{id}", s.handleGetContainer)
	apiMux.HandleFunc("POST /api/v1/containers/{id}/start", s.handleContainerStart)
	apiMux.HandleFunc("POST /api/v1/containers/{id}/stop", s.handleContainerStop)
	apiMux.HandleFunc("POST /api/v1/containers/{id}/restart", s.handleContainerRestart)
	apiMux.HandleFunc("POST /api/v1/containers/{id}/kill", s.handleContainerKill)
	apiMux.HandleFunc("GET /api/v1/containers/{id}/stats", s.handleContainerStats)
	apiMux.HandleFunc("GET /api/v1/containers/{id}/logs", s.handleContainerLogs)
	apiMux.HandleFunc("GET /api/v1/containers/{id}/terminal", s.handleContainerTerminal)

	// Images
	apiMux.HandleFunc("GET /api/v1/images", s.handleListImages)
	apiMux.HandleFunc("POST /api/v1/images/pull", s.handlePullImage)
	apiMux.HandleFunc("DELETE /api/v1/images/{id}", s.handleDeleteImage)
	apiMux.HandleFunc("POST /api/v1/images/prune", s.handlePruneImages)

	// Volumes
	apiMux.HandleFunc("GET /api/v1/volumes", s.handleListVolumes)
	apiMux.HandleFunc("DELETE /api/v1/volumes/{name}", s.handleDeleteVolume)
	apiMux.HandleFunc("POST /api/v1/volumes/prune", s.handlePruneVolumes)

	// Networks
	apiMux.HandleFunc("GET /api/v1/networks", s.handleListNetworks)
	apiMux.HandleFunc("DELETE /api/v1/networks/{id}", s.handleDeleteNetwork)

	// Metrics
	apiMux.HandleFunc("GET /api/v1/metrics/host", s.handleHostMetrics)
	apiMux.HandleFunc("GET /api/v1/metrics/gpu", s.handleGPUMetrics)

	// Events & Audit
	apiMux.HandleFunc("GET /api/v1/events", s.handleEventsSSE)
	apiMux.HandleFunc("GET /api/v1/audit", s.handleAuditLogs)

	// Jobs
	apiMux.HandleFunc("GET /api/v1/jobs", s.handleListJobs)
	apiMux.HandleFunc("GET /api/v1/jobs/{id}", s.handleGetJob)

	// Alerts
	apiMux.HandleFunc("GET /api/v1/alerts/rules", s.handleListAlertRules)
	apiMux.HandleFunc("GET /api/v1/alerts/events", s.handleListAlertEvents)

	// Backups & Settings
	apiMux.HandleFunc("GET /api/v1/backups", s.handleListBackups)
	apiMux.HandleFunc("POST /api/v1/backups", s.handleCreateBackup)
	apiMux.HandleFunc("GET /api/v1/settings", s.handleGetSettings)
	apiMux.HandleFunc("PUT /api/v1/settings", s.handleUpdateSettings)

	// Mount API with auth middleware
	mux.Handle("/api/", s.auth.Middleware(apiMux))

	// Static UI file server
	if s.staticFS != nil {
		fileServer := http.FileServer(http.FS(s.staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}

			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			// If file exists in staticFS, serve it, otherwise serve index.html for SPA router
			if f, err := s.staticFS.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}

			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
	}

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	if _, err := s.agentClient.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "agent unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func checkCriticalConfirmation(r *http.Request) bool {
	if r.Header.Get("X-Critical-Confirm") == "1" || r.Header.Get("X-Critical-Confirm") == "true" {
		return true
	}
	if r.URL.Query().Get("confirm") == "1" || r.URL.Query().Get("confirm") == "true" {
		return true
	}
	return false
}
