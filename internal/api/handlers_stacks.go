package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"docker-panel/internal/agent"
	"docker-panel/internal/auth"
	"docker-panel/internal/compose"
	"docker-panel/internal/jobs"
	"docker-panel/internal/models"
	"docker-panel/internal/rbac"
	"gopkg.in/yaml.v3"
)

type CreateStackRequest struct {
	Name           string `json:"name"`
	ComposeContent string `json:"compose_content"`
	EnvContent     string `json:"env_content,omitempty"`
}

type UpdateComposeRequest struct {
	ComposeContent string `json:"compose_content"`
	EnvContent     string `json:"env_content,omitempty"`
	Message        string `json:"message,omitempty"`
}

func (s *Server) handleListStacks(w http.ResponseWriter, r *http.Request) {
	stacks, err := s.db.GetAllStacks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stacks == nil {
		stacks = make([]*models.Stack, 0)
	}

	// Correlate with running containers
	containers, _ := s.agentClient.ListContainers(r.Context(), agent.ListContainersRequest{All: true})
	compose.MatchContainersToStacks(stacks, containers)

	writeJSON(w, http.StatusOK, stacks)
}

func (s *Server) handleCreateStack(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermComposeCreate) {
		writeError(w, http.StatusForbidden, "permission denied: cannot create stacks")
		return
	}

	var req CreateStackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.ComposeContent == "" {
		writeError(w, http.StatusBadRequest, "stack name and compose_content are required")
		return
	}

	// Validate Compose YAML syntax
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(req.ComposeContent), &parsed); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid compose YAML: %v", err))
		return
	}

	// Security scan
	report, err := s.scanner.Scan(req.ComposeContent, filepath.Join(s.stacksRoot, req.Name))
	score := 100
	if err == nil && report != nil {
		score = report.Score
	}

	stackID := fmt.Sprintf("stk_%d", time.Now().UnixNano())
	stackPath := filepath.Join(s.stacksRoot, req.Name)

	// Save files via agent
	res, err := s.agentClient.ComposeAction(r.Context(), agent.ComposeActionRequest{
		StackName:      req.Name,
		StackPath:      stackPath,
		Action:         "validate",
		ComposeContent: req.ComposeContent,
		EnvContent:     req.EnvContent,
	})
	if err != nil || (res != nil && !res.Success) {
		errMsg := "failed to initialize stack"
		if res != nil && res.Error != "" {
			errMsg = res.Error
		}
		writeError(w, http.StatusInternalServerError, errMsg)
		return
	}

	stack := &models.Stack{
		ID:             stackID,
		Name:           req.Name,
		Status:         models.StackStatusStopped,
		Path:           stackPath,
		ComposeContent: req.ComposeContent,
		EnvContent:     req.EnvContent,
		SecurityScore:  score,
		IsSystem:       req.Name == "panel" || req.Name == "docker-panel",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.db.UpsertStack(stack); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Create initial revision
	_, _ = s.db.CreateRevision(stackID, user.ID, user.Email, req.ComposeContent, req.EnvContent, "Initial stack creation")
	s.audit.Log(user.Email, "stack:create", req.Name, r.RemoteAddr, "", "success")

	writeJSON(w, http.StatusCreated, stack)
}

func (s *Server) handleGetStack(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stack, err := s.db.GetStackByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	// Match containers
	containers, _ := s.agentClient.ListContainers(r.Context(), agent.ListContainersRequest{All: true, StackName: stack.Name})
	stack.Containers = containers

	writeJSON(w, http.StatusOK, stack)
}

func (s *Server) handleDeleteStack(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermComposeDownV) {
		writeError(w, http.StatusForbidden, "insufficient permissions to delete stack")
		return
	}

	if !checkCriticalConfirmation(r) {
		writeError(w, http.StatusBadRequest, "critical action confirmation required (X-Critical-Confirm: 1)")
		return
	}

	id := r.PathValue("id")
	stack, err := s.db.GetStackByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	// Panel self protection: cannot delete system stack!
	if stack.IsSystem {
		writeError(w, http.StatusForbidden, "cannot delete system stack")
		return
	}

	// First execute compose delete (down + remove directory) via agent
	_, _ = s.agentClient.ComposeAction(r.Context(), agent.ComposeActionRequest{
		StackName:     stack.Name,
		StackPath:     stack.Path,
		Action:        "delete",
		RemoveVolumes: true,
	})

	if err := s.db.DeleteStack(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.audit.Log(user.Email, "stack:delete", stack.Name, r.RemoteAddr, "", "success")
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleGetStackCompose(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stack, err := s.db.GetStackByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"compose_content": stack.ComposeContent,
		"env_content":     stack.EnvContent,
	})
}

func (s *Server) handleUpdateStackCompose(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermComposeEdit) {
		writeError(w, http.StatusForbidden, "insufficient permissions to edit compose")
		return
	}

	id := r.PathValue("id")
	stack, err := s.db.GetStackByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	// System stack protection
	if stack.IsSystem && user.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "only OWNER can edit system stack")
		return
	}

	var req UpdateComposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate Compose YAML syntax
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(req.ComposeContent), &parsed); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid compose YAML: %v", err))
		return
	}

	// Run security scanner
	report, err := s.scanner.Scan(req.ComposeContent, stack.Path)
	score := 100
	if err == nil && report != nil {
		score = report.Score
	}

	// Update files on disk via agent
	res, err := s.agentClient.ComposeAction(r.Context(), agent.ComposeActionRequest{
		StackName:      stack.Name,
		StackPath:      stack.Path,
		Action:         "validate",
		ComposeContent: req.ComposeContent,
		EnvContent:     req.EnvContent,
	})
	if err != nil || (res != nil && !res.Success) {
		errMsg := "failed to validate and save compose files"
		if res != nil && res.Error != "" {
			errMsg = res.Error
		}
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	stack.ComposeContent = req.ComposeContent
	stack.EnvContent = req.EnvContent
	stack.SecurityScore = score
	stack.UpdatedAt = time.Now()

	if err := s.db.UpsertStack(stack); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	msg := req.Message
	if msg == "" {
		msg = "Updated compose configuration"
	}

	// Create revision
	rev, _ := s.db.CreateRevision(stack.ID, user.ID, user.Email, req.ComposeContent, req.EnvContent, msg)
	s.audit.Log(user.Email, "stack:edit_compose", stack.Name, r.RemoteAddr, msg, "success")

	writeJSON(w, http.StatusOK, map[string]any{
		"stack":    stack,
		"revision": rev,
		"security": report,
	})
}

func (s *Server) handleStackUp(w http.ResponseWriter, r *http.Request) {
	s.handleComposeJob(w, r, "up", false)
}

func (s *Server) handleStackDown(w http.ResponseWriter, r *http.Request) {
	removeVolumes := r.URL.Query().Get("volumes") == "true"
	if removeVolumes && !checkCriticalConfirmation(r) {
		writeError(w, http.StatusBadRequest, "down with remove_volumes requires confirmation")
		return
	}
	s.handleComposeJob(w, r, "down", removeVolumes)
}

func (s *Server) handleStackRestart(w http.ResponseWriter, r *http.Request) {
	s.handleComposeJob(w, r, "restart", false)
}

func (s *Server) handleStackPull(w http.ResponseWriter, r *http.Request) {
	s.handleComposeJob(w, r, "pull", false)
}

func (s *Server) handleStackBuild(w http.ResponseWriter, r *http.Request) {
	s.handleComposeJob(w, r, "build", false)
}

func (s *Server) handleComposeJob(w http.ResponseWriter, r *http.Request, jobType string, removeVolumes bool) {
	user := auth.GetUser(r.Context())
	perm := rbac.PermComposeUp
	switch jobType {
	case "down":
		if removeVolumes {
			perm = rbac.PermComposeDownV
		} else {
			perm = rbac.PermComposeUp
		}
	case "restart":
		perm = rbac.PermComposeRestart
	case "pull":
		perm = rbac.PermComposePull
	case "build":
		perm = rbac.PermComposeBuild
	}

	if !rbac.Can(user.Role, perm) {
		writeError(w, http.StatusForbidden, fmt.Sprintf("permission denied for compose %s", jobType))
		return
	}

	id := r.PathValue("id")
	stack, err := s.db.GetStackByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	if stack.IsSystem && (jobType == "down" || jobType == "restart") && user.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "only OWNER can modify system stack")
		return
	}

	job, err := s.jobManager.SubmitComposeJob(r.Context(), jobType, stack, removeVolumes, user.Email)
	if err != nil {
		if err == jobs.ErrStackLocked {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.audit.Log(user.Email, "stack:"+jobType, stack.Name, r.RemoteAddr, "", "pending")
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleStackSecurity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stack, err := s.db.GetStackByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	report, err := s.scanner.Scan(stack.ComposeContent, stack.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleStackRevisions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	revs, err := s.db.GetRevisionsForStack(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if revs == nil {
		revs = make([]models.StackRevision, 0)
	}
	writeJSON(w, http.StatusOK, revs)
}

func (s *Server) handleRestoreRevision(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if !rbac.Can(user.Role, rbac.PermComposeEdit) {
		writeError(w, http.StatusForbidden, "insufficient permissions to restore revision")
		return
	}

	id := r.PathValue("id")
	revIDStr := r.PathValue("revId")
	revID, err := strconv.ParseInt(revIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid revision id")
		return
	}

	stack, err := s.db.GetStackByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	rev, err := s.db.GetRevisionByID(revID)
	if err != nil || rev.StackID != stack.ID {
		writeError(w, http.StatusNotFound, "revision not found")
		return
	}

	// Update stack with revision content
	stack.ComposeContent = rev.Content
	stack.EnvContent = rev.EnvContent
	stack.UpdatedAt = time.Now()

	// Update on disk via agent
	_, _ = s.agentClient.ComposeAction(r.Context(), agent.ComposeActionRequest{
		StackName:      stack.Name,
		StackPath:      stack.Path,
		Action:         "validate",
		ComposeContent: rev.Content,
		EnvContent:     rev.EnvContent,
	})

	_ = s.db.UpsertStack(stack)
	// Create new revision representing the restore
	newRev, _ := s.db.CreateRevision(stack.ID, user.ID, user.Email, rev.Content, rev.EnvContent, fmt.Sprintf("Restored from revision #%d", rev.ID))
	s.audit.Log(user.Email, "stack:restore_revision", stack.Name, r.RemoteAddr, fmt.Sprintf("rev #%d", rev.ID), "success")

	writeJSON(w, http.StatusOK, map[string]any{
		"stack":    stack,
		"revision": newRev,
	})
}

func (s *Server) handleStackPreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stack, err := s.db.GetStackByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}

	var cf struct {
		Services map[string]struct {
			Image   string `yaml:"image"`
			Ports   []any  `yaml:"ports"`
			Volumes []any  `yaml:"volumes"`
		} `yaml:"services"`
		Networks map[string]any `yaml:"networks"`
		Volumes  map[string]any `yaml:"volumes"`
	}

	_ = yaml.Unmarshal([]byte(stack.ComposeContent), &cf)

	images := make([]string, 0)
	services := make([]string, 0)
	for svcName, svc := range cf.Services {
		services = append(services, svcName)
		if svc.Image != "" {
			images = append(images, svc.Image)
		}
	}

	secReport, _ := s.scanner.Scan(stack.ComposeContent, stack.Path)

	writeJSON(w, http.StatusOK, map[string]any{
		"stack_name": stack.Name,
		"services":   services,
		"images":     images,
		"networks":   cf.Networks,
		"volumes":    cf.Volumes,
		"security":   secReport,
	})
}
