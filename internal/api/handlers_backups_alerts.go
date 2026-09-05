package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"docker-panel/internal/auth"
	"docker-panel/internal/models"
	"docker-panel/internal/rbac"
)

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}
	jobs, err := s.db.ListJobs(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := s.db.GetJobByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, name, metric_type, condition, threshold, duration_seconds, enabled, severity, created_at FROM alert_rules ORDER BY id ASC")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var rules []models.AlertRule
	for rows.Next() {
		var ar models.AlertRule
		if err := rows.Scan(&ar.ID, &ar.Name, &ar.MetricType, &ar.Condition, &ar.Threshold, &ar.DurationSeconds, &ar.Enabled, &ar.Severity, &ar.CreatedAt); err != nil {
			continue
		}
		rules = append(rules, ar)
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handleListAlertEvents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, rule_id, rule_name, severity, status, message, started_at, resolved_at FROM alert_events ORDER BY id DESC LIMIT 50")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var events []models.AlertEvent
	for rows.Next() {
		var ae models.AlertEvent
		var resolvedAt *time.Time
		if err := rows.Scan(&ae.ID, &ae.RuleID, &ae.RuleName, &ae.Severity, &ae.Status, &ae.Message, &ae.StartedAt, &resolvedAt); err != nil {
			continue
		}
		ae.ResolvedAt = resolvedAt
		events = append(events, ae)
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermManageBackups) {
		writeError(w, http.StatusForbidden, "insufficient permissions to view backups")
		return
	}

	backups, err := s.backups.ListBackups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, backups)
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermManageBackups) {
		writeError(w, http.StatusForbidden, "insufficient permissions to trigger backup")
		return
	}

	record, err := s.backups.CreateBackup()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.audit.Log(user.Email, "backup:create", record.Filename, r.RemoteAddr, "", "success")
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermManageSettings) {
		writeError(w, http.StatusForbidden, "insufficient permissions to view settings")
		return
	}

	allowedPaths, _ := s.db.GetSetting("allowed_host_paths")
	if allowedPaths == "" {
		allowedPaths = "/srv/docker-panel"
	}

	retentionDays, _ := s.db.GetSetting("metrics_retention_days")
	if retentionDays == "" {
		retentionDays = "7"
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"allowed_host_paths":     allowedPaths,
		"metrics_retention_days": retentionDays,
	})
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermManageSettings) {
		writeError(w, http.StatusForbidden, "insufficient permissions to update settings")
		return
	}

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid settings payload")
		return
	}

	for k, v := range req {
		_ = s.db.SetSetting(k, v)
	}

	s.audit.Log(user.Email, "settings:update", "general", r.RemoteAddr, "", "success")
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
