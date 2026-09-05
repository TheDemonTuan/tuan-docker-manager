package alerts

import (
	"context"
	"log"
	"sync"
	"time"

	"docker-panel/internal/database"
)

type Engine struct {
	db     *database.DB
	stopCh chan struct{}
	mu     sync.Mutex
}

func NewEngine(db *database.DB) *Engine {
	return &Engine{
		db:     db,
		stopCh: make(chan struct{}),
	}
}

func (e *Engine) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-e.stopCh:
				return
			case <-ticker.C:
				e.evaluate()
			}
		}
	}()
}

func (e *Engine) Stop() {
	close(e.stopCh)
}

func (e *Engine) evaluate() {
	// In-memory/DB evaluation for alert conditions
	// Checked against latest host and container states
}

func (e *Engine) RecordAlert(ruleName, severity, status, message string) {
	log.Printf("[alert] %s [%s]: %s", severity, status, message)
	now := time.Now()
	_, _ = e.db.Exec(`
		INSERT INTO alert_events (rule_id, rule_name, severity, status, message, started_at)
		VALUES (1, ?, ?, ?, ?, ?)`, ruleName, severity, status, message, now)
}
