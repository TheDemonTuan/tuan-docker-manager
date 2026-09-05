package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"docker-panel/internal/auth"
	"docker-panel/internal/rbac"
)

func (s *Server) handleEventsSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	eventCh := s.eventBus.Subscribe()
	defer s.eventBus.Unsubscribe(eventCh)

	// Send initial ping
	fmt.Fprintf(w, ": ping\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-eventCh:
			if !ok {
				return
			}
			b, err := json.Marshal(ev)
			if err == nil {
				fmt.Fprintf(w, "data: %s\n\n", string(b))
				flusher.Flush()
			}
		}
	}
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermViewAudit) {
		writeError(w, http.StatusForbidden, "insufficient permissions to view audit logs")
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	logs, err := s.db.GetAuditLogs(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}
