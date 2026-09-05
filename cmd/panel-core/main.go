package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"docker-panel/internal/agent"
	"docker-panel/internal/alerts"
	"docker-panel/internal/api"
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

//go:embed webdist/*
var embeddedWebFS embed.FS

var Version = "0.1.0-mvp"

func main() {
	listenAddr := flag.String("listen", getEnv("LISTEN_ADDR", ":8080"), "Panel core listen address")
	agentSocket := flag.String("agent-socket", getEnv("AGENT_SOCKET_URL", "/run/panel-agent/agent.sock"), "Agent socket URL (unix://path or tcp://addr)")
	dbPath := flag.String("db", getEnv("DB_PATH", "./panel.db"), "Path to SQLite database")
	keyPath := flag.String("key", getEnv("MASTER_KEY_PATH", "./secrets/master.key"), "Path to master encryption key")
	stacksRoot := flag.String("stacks-root", getEnv("STACKS_ROOT", "/srv/docker-panel/stacks"), "Compose stacks root directory")
	backupDir := flag.String("backup-dir", getEnv("BACKUP_DIR", "/srv/docker-panel/backups"), "Backup storage directory")
	allowedEmails := flag.String("allowed-emails", getEnv("ALLOWED_EMAILS", ""), "Comma-separated list of allowed user emails")
	adminEmail := flag.String("admin-email", getEnv("ADMIN_EMAIL", "admin@example.com"), "Initial admin email")
	devMode := flag.Bool("dev", getEnv("DEV_MODE", "true") == "true", "Enable development mode")
	flag.Parse()

	log.Printf("[panel-core] Starting version %s...", Version)
	log.Printf("[panel-core] Listening on %s (DevMode: %v)", *listenAddr, *devMode)
	log.Printf("[panel-core] Agent socket: %s", *agentSocket)
	log.Printf("[panel-core] Stacks directory: %s", *stacksRoot)

	// 1. Initialize Database
	db, err := database.Open(*dbPath)
	if err != nil {
		log.Fatalf("[panel-core] Failed to open database: %v", err)
	}
	defer db.Close()

	// 2. Initialize Secrets Manager
	secretsMgr, err := secrets.NewManager(*keyPath)
	if err != nil {
		log.Fatalf("[panel-core] Failed to initialize secrets manager: %v", err)
	}

	// 3. Allowed Host Paths
	allowedPathsList := []string{*stacksRoot}
	customPaths, _ := db.GetSetting("allowed_host_paths")
	if customPaths != "" {
		for _, p := range strings.Split(customPaths, "\n") {
			p = strings.TrimSpace(p)
			if p != "" {
				allowedPathsList = append(allowedPathsList, p)
			}
		}
	}
	scanner := security.NewScanner(allowedPathsList)

	// 4. Initialize Agent Client
	agentClient := agent.NewClient(*agentSocket)

	// 5. Initialize Subsystems
	var emails []string
	if *allowedEmails != "" {
		emails = strings.Split(*allowedEmails, ",")
	}
	authenticator := auth.NewAuthenticator(db, emails, *adminEmail, *devMode)
	auditLogger := audit.NewLogger(db)
	alertEngine := alerts.NewEngine(db)
	backupMgr := backup.NewManager(*dbPath, *stacksRoot, *backupDir, secretsMgr)
	jobMgr := jobs.NewManager(db, agentClient)
	eventBus := events.NewBus()

	// 6. Discover existing stacks on startup
	discoverExistingStacks(db, *stacksRoot, scanner)

	// 7. Background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	alertEngine.Start(ctx)
	startMetricsCollector(ctx, db, agentClient)
	startEventsSubscriber(ctx, agentClient, eventBus, db)

	// 8. Static Web Assets
	var staticSubFS fs.FS
	if sub, err := fs.Sub(embeddedWebFS, "webdist"); err == nil {
		staticSubFS = sub
	}

	// 9. API Server
	server := api.NewServer(
		db, agentClient, jobMgr, scanner, authenticator,
		auditLogger, alertEngine, backupMgr, eventBus,
		staticSubFS, *stacksRoot,
	)

	httpServer := &http.Server{
		Addr:         *listenAddr,
		Handler:      server.Routes(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // Streaming logs / events / websocket
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[panel-core] Server error: %v", err)
		}
	}()

	<-stop
	log.Printf("[panel-core] Shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	log.Printf("[panel-core] Stopped.")
}

func discoverExistingStacks(db *database.DB, stacksRoot string, scanner *security.ComposeScanner) {
	discovered, err := compose.DiscoverStacks(stacksRoot)
	if err != nil {
		log.Printf("[panel-core] Stacks discovery warning: %v", err)
		return
	}

	for _, ds := range discovered {
		existing, _ := db.GetStackByName(ds.Name)
		if existing == nil {
			report, _ := scanner.Scan(ds.ComposeContent, ds.Path)
			score := 100
			if report != nil {
				score = report.Score
			}

			stk := &models.Stack{
				ID:             "stk_" + ds.Name,
				Name:           ds.Name,
				Status:         models.StackStatusStopped,
				Path:           ds.Path,
				ComposeContent: ds.ComposeContent,
				EnvContent:     ds.EnvContent,
				SecurityScore:  score,
				IsSystem:       ds.Name == "panel" || ds.Name == "docker-panel",
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}
			_ = db.UpsertStack(stk)
			log.Printf("[panel-core] Imported discovered stack: %s (score: %d)", ds.Name, score)
		}
	}
}

func startMetricsCollector(ctx context.Context, db *database.DB, client *agent.Client) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m, err := client.GetHostMetrics(ctx)
				if err == nil && m != nil {
					_ = db.SaveHostMetrics("vps-01", m)
				}
			}
		}
	}()
}

func startEventsSubscriber(ctx context.Context, client *agent.Client, bus *events.Bus, db *database.DB) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				evCh := make(chan models.DockerEvent, 20)
				subCtx, subCancel := context.WithCancel(ctx)

				go func() {
					for ev := range evCh {
						bus.Publish(ev)
						_, _ = db.Exec(`
							INSERT INTO docker_events (host_id, timestamp, event_type, action, actor_id, actor_name, message)
							VALUES ('vps-01', ?, ?, ?, ?, ?, ?)`,
							ev.Timestamp, ev.Type, ev.Action, ev.ActorID, ev.ActorName, ev.Message)
					}
				}()

				_ = client.StreamEvents(subCtx, evCh)
				subCancel()
				time.Sleep(3 * time.Second) // backoff before reconnect
			}
		}
	}()
}

func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}
