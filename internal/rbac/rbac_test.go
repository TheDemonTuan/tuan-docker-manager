package rbac

import (
	"docker-panel/internal/models"
	"testing"
)

func TestRBAC_Permissions(t *testing.T) {
	tests := []struct {
		role models.Role
		perm string
		want bool
	}{
		// Viewer tests
		{models.RoleViewer, PermViewDashboard, true},
		{models.RoleViewer, PermViewContainers, true},
		{models.RoleViewer, PermContainerAction, false},
		{models.RoleViewer, PermComposeUp, false},
		{models.RoleViewer, PermComposeEdit, false},
		{models.RoleViewer, PermVolumeDelete, false},

		// Operator tests
		{models.RoleOperator, PermViewDashboard, true},
		{models.RoleOperator, PermContainerAction, true},
		{models.RoleOperator, PermComposeUp, true},
		{models.RoleOperator, PermComposeRestart, true},
		{models.RoleOperator, PermComposeEdit, false},
		{models.RoleOperator, PermVolumeDelete, false},

		// Admin tests
		{models.RoleAdmin, PermViewDashboard, true},
		{models.RoleAdmin, PermContainerAction, true},
		{models.RoleAdmin, PermComposeEdit, true},
		{models.RoleAdmin, PermManageVolumes, true},
		{models.RoleAdmin, PermManageUsers, false},
		{models.RoleAdmin, PermVolumeDelete, false},

		// Owner tests
		{models.RoleOwner, PermViewDashboard, true},
		{models.RoleOwner, PermComposeEdit, true},
		{models.RoleOwner, PermVolumeDelete, true},
		{models.RoleOwner, PermManageUsers, true},
	}

	for _, tc := range tests {
		got := Can(tc.role, tc.perm)
		if got != tc.want {
			t.Errorf("Can(%s, %s) = %v; want %v", tc.role, tc.perm, got, tc.want)
		}
	}
}

func TestRBAC_ParseRole(t *testing.T) {
	if ParseRole("viewer") != models.RoleViewer {
		t.Errorf("expected viewer")
	}
	if ParseRole("OPERATOR") != models.RoleOperator {
		t.Errorf("expected operator")
	}
	if ParseRole("Admin") != models.RoleAdmin {
		t.Errorf("expected admin")
	}
	if ParseRole("OWNER") != models.RoleOwner {
		t.Errorf("expected owner")
	}
	if ParseRole("unknown") != models.RoleViewer {
		t.Errorf("expected default viewer for unknown role")
	}
}
