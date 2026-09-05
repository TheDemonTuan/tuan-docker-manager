package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"docker-panel/internal/models"
)

// User & Session Repository
func (db *DB) GetOrCreateUserByEmail(email string, defaultRole models.Role) (*models.User, error) {
	var u models.User
	var lastLogin sql.NullTime
	err := db.QueryRow("SELECT id, email, role, created_at, last_login_at FROM users WHERE email = ?", email).
		Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt, &lastLogin)

	if err == nil {
		if lastLogin.Valid {
			u.LastLoginAt = &lastLogin.Time
		}
		// Update last login
		now := time.Now()
		_, _ = db.Exec("UPDATE users SET last_login_at = ? WHERE id = ?", now, u.ID)
		u.LastLoginAt = &now
		return &u, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create user
	now := time.Now()
	id := fmt.Sprintf("usr_%d", now.UnixNano())
	_, err = db.Exec("INSERT INTO users (id, email, role, created_at, last_login_at) VALUES (?, ?, ?, ?, ?)",
		id, email, string(defaultRole), now, now)
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:          id,
		Email:       email,
		Role:        defaultRole,
		CreatedAt:   now,
		LastLoginAt: &now,
	}, nil
}

func (db *DB) GetUserByID(id string) (*models.User, error) {
	var u models.User
	var lastLogin sql.NullTime
	err := db.QueryRow("SELECT id, email, role, created_at, last_login_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt, &lastLogin)
	if err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLoginAt = &lastLogin.Time
	}
	return &u, nil
}

func (db *DB) CreateSession(userID, token string, duration time.Duration) (*models.Session, error) {
	now := time.Now()
	expiresAt := now.Add(duration)
	id := fmt.Sprintf("sess_%d", now.UnixNano())

	_, err := db.Exec("INSERT INTO sessions (id, user_id, token, expires_at, created_at) VALUES (?, ?, ?, ?, ?)",
		id, userID, token, expiresAt, now)
	if err != nil {
		return nil, err
	}

	return &models.Session{
		ID:        id,
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

func (db *DB) GetUserBySessionToken(token string) (*models.User, error) {
	var u models.User
	var lastLogin sql.NullTime
	var expiresAt time.Time

	err := db.QueryRow(`
		SELECT u.id, u.email, u.role, u.created_at, u.last_login_at, s.expires_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ?`, token).
		Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt, &lastLogin, &expiresAt)

	if err != nil {
		return nil, err
	}

	if time.Now().After(expiresAt) {
		_, _ = db.Exec("DELETE FROM sessions WHERE token = ?", token)
		return nil, sql.ErrNoRows
	}

	if lastLogin.Valid {
		u.LastLoginAt = &lastLogin.Time
	}
	return &u, nil
}

// Stack Repository
func (db *DB) GetAllStacks() ([]*models.Stack, error) {
	rows, err := db.Query("SELECT id, name, status, path, compose_content, env_content, security_score, is_system, created_at, updated_at FROM stacks ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.Stack
	for rows.Next() {
		var s models.Stack
		if err := rows.Scan(&s.ID, &s.Name, &s.Status, &s.Path, &s.ComposeContent, &s.EnvContent, &s.SecurityScore, &s.IsSystem, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &s)
	}
	return list, nil
}

func (db *DB) GetStackByID(id string) (*models.Stack, error) {
	var s models.Stack
	err := db.QueryRow("SELECT id, name, status, path, compose_content, env_content, security_score, is_system, created_at, updated_at FROM stacks WHERE id = ?", id).
		Scan(&s.ID, &s.Name, &s.Status, &s.Path, &s.ComposeContent, &s.EnvContent, &s.SecurityScore, &s.IsSystem, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) GetStackByName(name string) (*models.Stack, error) {
	var s models.Stack
	err := db.QueryRow("SELECT id, name, status, path, compose_content, env_content, security_score, is_system, created_at, updated_at FROM stacks WHERE name = ?", name).
		Scan(&s.ID, &s.Name, &s.Status, &s.Path, &s.ComposeContent, &s.EnvContent, &s.SecurityScore, &s.IsSystem, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) UpsertStack(s *models.Stack) error {
	now := time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.UpdatedAt = now

	_, err := db.Exec(`
		INSERT INTO stacks (id, name, status, path, compose_content, env_content, security_score, is_system, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			status=excluded.status,
			path=excluded.path,
			compose_content=excluded.compose_content,
			env_content=excluded.env_content,
			security_score=excluded.security_score,
			is_system=excluded.is_system,
			updated_at=excluded.updated_at`,
		s.ID, s.Name, string(s.Status), s.Path, s.ComposeContent, s.EnvContent, s.SecurityScore, s.IsSystem, s.CreatedAt, s.UpdatedAt)
	return err
}

func (db *DB) DeleteStack(id string) error {
	_, err := db.Exec("DELETE FROM stacks WHERE id = ?", id)
	return err
}

// Stack Revisions
func (db *DB) CreateRevision(stackID, userID, userEmail, content, envContent, message string) (*models.StackRevision, error) {
	hash := sha256.Sum256([]byte(content + "\n" + envContent))
	shaStr := hex.EncodeToString(hash[:])
	now := time.Now()

	res, err := db.Exec(`
		INSERT INTO stack_revisions (stack_id, user_id, user_email, content, env_content, sha256, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		stackID, userID, userEmail, content, envContent, shaStr, message, now)
	if err != nil {
		return nil, err
	}

	revID, _ := res.LastInsertId()
	return &models.StackRevision{
		ID:         revID,
		StackID:    stackID,
		UserID:     userID,
		UserEmail:  userEmail,
		Content:    content,
		EnvContent: envContent,
		SHA256:     shaStr,
		Message:    message,
		CreatedAt:  now,
	}, nil
}

func (db *DB) GetRevisionsForStack(stackID string) ([]models.StackRevision, error) {
	rows, err := db.Query(`
		SELECT id, stack_id, user_id, user_email, content, env_content, sha256, message, created_at
		FROM stack_revisions
		WHERE stack_id = ?
		ORDER BY id DESC`, stackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revs []models.StackRevision
	for rows.Next() {
		var r models.StackRevision
		if err := rows.Scan(&r.ID, &r.StackID, &r.UserID, &r.UserEmail, &r.Content, &r.EnvContent, &r.SHA256, &r.Message, &r.CreatedAt); err != nil {
			return nil, err
		}
		revs = append(revs, r)
	}
	return revs, nil
}

func (db *DB) GetRevisionByID(id int64) (*models.StackRevision, error) {
	var r models.StackRevision
	err := db.QueryRow(`
		SELECT id, stack_id, user_id, user_email, content, env_content, sha256, message, created_at
		FROM stack_revisions
		WHERE id = ?`, id).
		Scan(&r.ID, &r.StackID, &r.UserID, &r.UserEmail, &r.Content, &r.EnvContent, &r.SHA256, &r.Message, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Jobs Repository
func (db *DB) CreateJob(job *models.Job) error {
	now := time.Now()
	if job.StartedAt.IsZero() {
		job.StartedAt = now
	}
	_, err := db.Exec(`
		INSERT INTO jobs (id, type, stack_id, status, error, logs, started_at, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Type, job.StackID, string(job.Status), job.Error, job.Logs, job.StartedAt, job.CreatedBy)
	return err
}

func (db *DB) UpdateJob(job *models.Job) error {
	_, err := db.Exec(`
		UPDATE jobs
		SET status = ?, error = ?, logs = ?, completed_at = ?
		WHERE id = ?`,
		string(job.Status), job.Error, job.Logs, job.CompletedAt, job.ID)
	return err
}

func (db *DB) GetJobByID(id string) (*models.Job, error) {
	var j models.Job
	var completedAt sql.NullTime
	var stackID sql.NullString

	err := db.QueryRow(`
		SELECT id, type, stack_id, status, error, logs, started_at, completed_at, created_by
		FROM jobs WHERE id = ?`, id).
		Scan(&j.ID, &j.Type, &stackID, &j.Status, &j.Error, &j.Logs, &j.StartedAt, &completedAt, &j.CreatedBy)
	if err != nil {
		return nil, err
	}

	if stackID.Valid {
		j.StackID = stackID.String
	}
	if completedAt.Valid {
		j.CompletedAt = &completedAt.Time
	}
	return &j, nil
}

func (db *DB) ListJobs(limit int) ([]models.Job, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(`
		SELECT id, type, stack_id, status, error, logs, started_at, completed_at, created_by
		FROM jobs ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		var stackID sql.NullString
		var completedAt sql.NullTime
		if err := rows.Scan(&j.ID, &j.Type, &stackID, &j.Status, &j.Error, &j.Logs, &j.StartedAt, &completedAt, &j.CreatedBy); err != nil {
			return nil, err
		}
		if stackID.Valid {
			j.StackID = stackID.String
		}
		if completedAt.Valid {
			j.CompletedAt = &completedAt.Time
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// Metrics Repository
func (db *DB) SaveHostMetrics(hostID string, m *models.HostMetrics) error {
	_, err := db.Exec(`
		INSERT INTO host_metrics (host_id, timestamp, cpu_percent, memory_used, memory_total, disk_used, disk_total, load1, load5, load15, net_rx, net_tx)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		hostID, m.Timestamp, m.CPUPercent, m.MemoryUsed, m.MemoryTotal, m.DiskUsed, m.DiskTotal, m.Load1, m.Load5, m.Load15, m.NetRxBytesRate, m.NetTxBytesRate)
	return err
}

func (db *DB) GetHostMetricsHistory(hostID string, since time.Time) ([]models.HostMetrics, error) {
	rows, err := db.Query(`
		SELECT timestamp, cpu_percent, memory_used, memory_total, disk_used, disk_total, load1, load5, load15, net_rx, net_tx
		FROM host_metrics
		WHERE host_id = ? AND timestamp >= ?
		ORDER BY timestamp ASC`, hostID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.HostMetrics
	for rows.Next() {
		var m models.HostMetrics
		if err := rows.Scan(&m.Timestamp, &m.CPUPercent, &m.MemoryUsed, &m.MemoryTotal, &m.DiskUsed, &m.DiskTotal, &m.Load1, &m.Load5, &m.Load15, &m.NetRxBytesRate, &m.NetTxBytesRate); err != nil {
			return nil, err
		}
		if m.MemoryTotal > 0 {
			m.MemoryPercent = (float64(m.MemoryUsed) / float64(m.MemoryTotal)) * 100.0
		}
		if m.DiskTotal > 0 {
			m.DiskPercent = (float64(m.DiskUsed) / float64(m.DiskTotal)) * 100.0
		}
		list = append(list, m)
	}
	return list, nil
}

// Audit Log Repository
func (db *DB) CreateAuditLog(userEmail, action, resource, ip, metadata, result string) error {
	_, err := db.Exec(`
		INSERT INTO audit_logs (user_email, action, resource, ip, metadata, result, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userEmail, action, resource, ip, metadata, result, time.Now())
	return err
}

func (db *DB) GetAuditLogs(limit int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Query(`
		SELECT id, user_email, action, resource, ip, metadata, result, created_at
		FROM audit_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.AuditLog
	for rows.Next() {
		var a models.AuditLog
		if err := rows.Scan(&a.ID, &a.UserEmail, &a.Action, &a.Resource, &a.IP, &a.Metadata, &a.Result, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, nil
}

// Settings
func (db *DB) GetSetting(key string) (string, error) {
	var val string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	return val, err
}

func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, time.Now())
	return err
}
