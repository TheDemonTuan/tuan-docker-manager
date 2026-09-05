package api

import (
	"net/http"
	"time"

	"docker-panel/internal/models"
)

func (s *Server) handleHostMetrics(w http.ResponseWriter, r *http.Request) {
	rangeStr := r.URL.Query().Get("range")
	if rangeStr != "" {
		// History lookup
		duration := 1 * time.Hour
		switch rangeStr {
		case "6h":
			duration = 6 * time.Hour
		case "24h":
			duration = 24 * time.Hour
		case "7d":
			duration = 7 * 24 * time.Hour
		}
		since := time.Now().Add(-duration)
		history, err := s.db.GetHostMetricsHistory("vps-01", since)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, history)
		return
	}

	// Current live metrics
	m, err := s.agentClient.GetHostMetrics(r.Context())
	if err != nil {
		m = &models.HostMetrics{
			Timestamp:   time.Now(),
			CPUPercent:  0.0,
			MemoryTotal: 1024 * 1024 * 1024,
		}
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleGPUMetrics(w http.ResponseWriter, r *http.Request) {
	m, err := s.agentClient.GetGPUMetrics(r.Context())
	if err != nil {
		m = &models.GPUMetrics{
			Available: false,
			GPUs:      make([]models.GPUInfo, 0),
		}
	}
	writeJSON(w, http.StatusOK, m)
}
