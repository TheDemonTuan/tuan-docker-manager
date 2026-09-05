package rbac

import (
	"strings"

	"docker-panel/internal/models"
)

const (
	PermViewDashboard  = "view:dashboard"
	PermViewContainers = "view:containers"
	PermViewLogs       = "view:logs"
	PermViewTerminal   = "view:terminal"
	PermViewStats      = "view:stats"
	PermViewCompose    = "view:compose"
	PermViewEvents     = "view:events"
	PermViewMetrics    = "view:metrics"
	PermViewAudit      = "view:audit"

	PermContainerAction = "action:container" // start, stop, restart
	PermContainerKill   = "action:container_kill"
	PermComposeUp       = "action:compose_up"
	PermComposeRestart  = "action:compose_restart"
	PermComposePull     = "action:compose_pull"

	PermComposeEdit    = "manage:compose_edit"
	PermComposeCreate  = "manage:compose_create"
	PermComposeBuild   = "manage:compose_build"
	PermManageImages   = "manage:images"
	PermManageNetworks = "manage:networks"
	PermManageVolumes  = "manage:volumes"

	PermVolumeDelete   = "admin:volume_delete"
	PermComposeDownV   = "admin:compose_down_v"
	PermSystemPrune    = "admin:system_prune"
	PermManageSecrets  = "admin:secrets"
	PermManageSettings = "admin:settings"
	PermManageUsers    = "admin:users"
	PermManageBackups  = "admin:backups"
)

var rolePermissions = map[models.Role]map[string]bool{
	models.RoleViewer: {
		PermViewDashboard:  true,
		PermViewContainers: true,
		PermViewLogs:       true,
		PermViewStats:      true,
		PermViewCompose:    true,
		PermViewEvents:     true,
		PermViewMetrics:    true,
		PermViewAudit:      true,
	},
	models.RoleOperator: {
		// Inherits Viewer
		PermViewDashboard:  true,
		PermViewContainers: true,
		PermViewLogs:       true,
		PermViewTerminal:   true,
		PermViewStats:      true,
		PermViewCompose:    true,
		PermViewEvents:     true,
		PermViewMetrics:    true,
		PermViewAudit:      true,
		// Operator actions
		PermContainerAction: true,
		PermComposeUp:       true,
		PermComposeRestart:  true,
		PermComposePull:     true,
	},
	models.RoleAdmin: {
		// Inherits Operator
		PermViewDashboard:   true,
		PermViewContainers:  true,
		PermViewLogs:        true,
		PermViewTerminal:    true,
		PermViewStats:       true,
		PermViewCompose:     true,
		PermViewEvents:      true,
		PermViewMetrics:     true,
		PermViewAudit:       true,
		PermContainerAction: true,
		PermContainerKill:   true,
		PermComposeUp:       true,
		PermComposeRestart:  true,
		PermComposePull:     true,
		// Admin actions
		PermComposeEdit:    true,
		PermComposeCreate:  true,
		PermComposeBuild:   true,
		PermManageImages:   true,
		PermManageNetworks: true,
		PermManageVolumes:  true,
	},
	models.RoleOwner: {
		// All permissions
		PermViewDashboard:   true,
		PermViewContainers:  true,
		PermViewLogs:        true,
		PermViewTerminal:    true,
		PermViewStats:       true,
		PermViewCompose:     true,
		PermViewEvents:      true,
		PermViewMetrics:     true,
		PermViewAudit:       true,
		PermContainerAction: true,
		PermContainerKill:   true,
		PermComposeUp:       true,
		PermComposeRestart:  true,
		PermComposePull:     true,
		PermComposeEdit:     true,
		PermComposeCreate:   true,
		PermComposeBuild:    true,
		PermManageImages:    true,
		PermManageNetworks:  true,
		PermManageVolumes:   true,
		PermVolumeDelete:    true,
		PermComposeDownV:    true,
		PermSystemPrune:     true,
		PermManageSecrets:   true,
		PermManageSettings:  true,
		PermManageUsers:     true,
		PermManageBackups:   true,
	},
}

func Can(role models.Role, perm string) bool {
	if perms, ok := rolePermissions[role]; ok {
		return perms[perm]
	}
	return false
}

func ParseRole(roleStr string) models.Role {
	switch strings.ToUpper(strings.TrimSpace(roleStr)) {
	case "OWNER":
		return models.RoleOwner
	case "ADMIN":
		return models.RoleAdmin
	case "OPERATOR":
		return models.RoleOperator
	default:
		return models.RoleViewer
	}
}
