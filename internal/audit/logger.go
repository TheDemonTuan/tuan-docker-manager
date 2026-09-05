package audit

import (
	"log"

	"docker-panel/internal/database"
)

type Logger struct {
	db *database.DB
}

func NewLogger(db *database.DB) *Logger {
	return &Logger{db: db}
}

func (l *Logger) Log(userEmail, action, resource, ip, metadata, result string) {
	go func() {
		if err := l.db.CreateAuditLog(userEmail, action, resource, ip, metadata, result); err != nil {
			log.Printf("[audit] Failed to save audit log: %v", err)
		}
	}()
}
