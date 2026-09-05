package api

import (
	"net/http"

	"docker-panel/internal/auth"
	"docker-panel/internal/rbac"
)

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	permissions := make([]string, 0)
	allPerms := []string{
		rbac.PermViewDashboard, rbac.PermViewContainers, rbac.PermViewLogs,
		rbac.PermViewTerminal, rbac.PermViewStats, rbac.PermViewCompose,
		rbac.PermViewEvents, rbac.PermViewMetrics, rbac.PermViewAudit,
		rbac.PermContainerAction, rbac.PermContainerKill, rbac.PermComposeUp,
		rbac.PermComposeRestart, rbac.PermComposePull, rbac.PermComposeEdit,
		rbac.PermComposeCreate, rbac.PermComposeBuild, rbac.PermManageImages,
		rbac.PermManageNetworks, rbac.PermManageVolumes, rbac.PermVolumeDelete,
		rbac.PermComposeDownV, rbac.PermSystemPrune, rbac.PermManageSecrets,
		rbac.PermManageSettings, rbac.PermManageUsers, rbac.PermManageBackups,
	}

	for _, p := range allPerms {
		if rbac.Can(user.Role, p) {
			permissions = append(permissions, p)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":        user,
		"permissions": permissions,
	})
}
