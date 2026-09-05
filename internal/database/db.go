package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(dbPath string) (*DB, error) {
	if dbPath == "" {
		dbPath = "./panel.db"
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db dir: %w", err)
	}

	// SQLite connection string with WAL and busy timeout
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", dbPath)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	sqlDB.SetMaxOpenConns(1) // SQLite single writer concurrency safety
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	db := &DB{DB: sqlDB}
	if err := db.initSchema(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

func (db *DB) initSchema() error {
	if _, err := db.Exec(SchemaSQL); err != nil {
		return err
	}

	// Initialize default host if empty
	var hostCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM hosts").Scan(&hostCount)
	if hostCount == 0 {
		hostname, _ := os.Hostname()
		now := time.Now()
		_, _ = db.Exec(`INSERT INTO hosts (id, name, hostname, status, created_at, last_seen_at) 
			VALUES (?, ?, ?, ?, ?, ?)`, "vps-01", "Primary VPS", hostname, "healthy", now, now)
	}

	// Initialize default alert rules if empty
	var ruleCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM alert_rules").Scan(&ruleCount)
	if ruleCount == 0 {
		now := time.Now()
		rules := []struct {
			name     string
			metric   string
			cond     string
			thresh   float64
			duration int
			sev      string
		}{
			{"High CPU Usage", "cpu", "gt", 90.0, 300, "critical"},
			{"High Memory Usage", "memory", "gt", 90.0, 300, "critical"},
			{"High Disk Usage", "disk", "gt", 85.0, 60, "warning"},
			{"GPU High Temperature", "gpu_temp", "gt", 85.0, 60, "critical"},
			{"Container Unhealthy", "container_unhealthy", "eq", 1.0, 0, "warning"},
			{"Container Stopped Unexpectedly", "container_down", "eq", 1.0, 0, "critical"},
		}
		for _, r := range rules {
			_, _ = db.Exec(`INSERT INTO alert_rules (name, metric_type, condition, threshold, duration_seconds, enabled, severity, created_at)
				VALUES (?, ?, ?, ?, ?, 1, ?, ?)`, r.name, r.metric, r.cond, r.thresh, r.duration, r.sev, now)
		}
	}

	return nil
}
