package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunner_StackNamesAndDirs(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewRunner(tempDir)

	if runner.StacksRoot() != tempDir {
		t.Errorf("expected StacksRoot %s, got %s", tempDir, runner.StacksRoot())
	}

	// Valid stack name
	dir, err := runner.GetStackDir("valid-stack_1")
	if err != nil {
		t.Fatalf("expected valid stack name to succeed, got %v", err)
	}
	expected := filepath.Join(tempDir, "valid-stack_1")
	if dir != expected {
		t.Errorf("expected dir %s, got %s", expected, dir)
	}

	// Invalid stack names
	invalidNames := []string{"", " ", "../evil", "foo/bar", "foo\\bar", "stack;rm"}
	for _, name := range invalidNames {
		if _, err := runner.GetStackDir(name); err == nil {
			t.Errorf("expected error for invalid stack name %q, got nil", name)
		}
	}
}

func TestRunner_SaveReadDelete(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewRunner(tempDir)

	composeYAML := "services:\n  web:\n    image: nginx\n"
	envData := "KEY=value\n"

	// 1. Save stack files
	if err := runner.SaveStackFiles("demo", composeYAML, envData); err != nil {
		t.Fatalf("SaveStackFiles failed: %v", err)
	}

	// 2. Read stack files
	readCompose, readEnv, err := runner.ReadStackFiles("demo")
	if err != nil {
		t.Fatalf("ReadStackFiles failed: %v", err)
	}
	if readCompose != composeYAML {
		t.Errorf("compose content mismatch")
	}
	if readEnv != envData {
		t.Errorf("env content mismatch")
	}

	// 3. Delete stack dir
	if err := runner.DeleteStackDir("demo"); err != nil {
		t.Fatalf("DeleteStackDir failed: %v", err)
	}

	// Directory should be gone
	dir, _ := runner.GetStackDir("demo")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected stack directory to be deleted")
	}
}

func TestRunner_ValidateYAML(t *testing.T) {
	runner := NewRunner("")

	// Valid compose YAML
	valid := "services:\n  app:\n    image: alpine\n"
	if err := runner.ValidateComposeYAML(valid); err != nil {
		t.Errorf("expected valid YAML, got error: %v", err)
	}

	// Missing services
	missingServices := "version: '3.8'\nnetworks:\n  default:\n"
	if err := runner.ValidateComposeYAML(missingServices); err == nil {
		t.Errorf("expected error for compose without services")
	}

	// Syntax error
	invalidSyntax := "services: [broken: {{"
	if err := runner.ValidateComposeYAML(invalidSyntax); err == nil {
		t.Errorf("expected error for invalid syntax")
	}
}
