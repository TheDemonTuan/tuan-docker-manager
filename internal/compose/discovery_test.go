package compose

import (
	"docker-panel/internal/models"
	"os"
	"path/filepath"
	"testing"
)

func TestCompose_Discovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "stacks-discovery-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create valid stack 1
	stack1Dir := filepath.Join(tempDir, "web-app")
	if err := os.MkdirAll(stack1Dir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stack1Dir, "docker-compose.yml"), []byte("services:\n  web:\n    image: nginx\n"), 0644); err != nil {
		t.Fatalf("failed to write compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stack1Dir, ".env"), []byte("PORT=8080\n"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Create valid stack 2 with compose.yaml
	stack2Dir := filepath.Join(tempDir, "database")
	if err := os.MkdirAll(stack2Dir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stack2Dir, "compose.yaml"), []byte("services:\n  db:\n    image: postgres\n"), 0644); err != nil {
		t.Fatalf("failed to write compose: %v", err)
	}

	// Create empty folder (should be ignored)
	emptyDir := filepath.Join(tempDir, "empty-folder")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}

	stacks, err := DiscoverStacks(tempDir)
	if err != nil {
		t.Fatalf("unexpected error discovering stacks: %v", err)
	}

	if len(stacks) != 2 {
		t.Fatalf("expected 2 discovered stacks, got %d", len(stacks))
	}

	foundMap := make(map[string]DiscoveredStack)
	for _, s := range stacks {
		foundMap[s.Name] = s
	}

	s1, ok := foundMap["web-app"]
	if !ok {
		t.Errorf("web-app not found")
	} else if s1.EnvContent != "PORT=8080\n" {
		t.Errorf("expected env content PORT=8080, got %q", s1.EnvContent)
	}

	_, ok2 := foundMap["database"]
	if !ok2 {
		t.Errorf("database not found")
	}
}

func TestCompose_MatchContainersToStacks(t *testing.T) {
	stacks := []*models.Stack{
		{Name: "web-app"},
		{Name: "database"},
		{Name: "analytics"},
	}

	containers := []models.ContainerInfo{
		{ID: "c1", Names: []string{"/web-app_web_1"}, StackName: "web-app", State: "running"},
		{ID: "c2", Names: []string{"/database_db_1"}, StackName: "database", State: "running"},
		{ID: "c3", Names: []string{"/database_cache_1"}, StackName: "database", State: "exited"},
	}

	MatchContainersToStacks(stacks, containers)

	if stacks[0].Status != models.StackStatusRunning {
		t.Errorf("web-app expected running, got %s", stacks[0].Status)
	}
	if len(stacks[0].Containers) != 1 {
		t.Errorf("web-app expected 1 container, got %d", len(stacks[0].Containers))
	}

	if stacks[1].Status != models.StackStatusPartial {
		t.Errorf("database expected partial, got %s", stacks[1].Status)
	}
	if len(stacks[1].Containers) != 2 {
		t.Errorf("database expected 2 containers, got %d", len(stacks[1].Containers))
	}

	if stacks[2].Status != models.StackStatusStopped {
		t.Errorf("analytics expected stopped, got %s", stacks[2].Status)
	}
}
