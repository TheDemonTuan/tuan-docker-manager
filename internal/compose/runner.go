package compose

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var validStackNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type Runner struct {
	stacksRoot string
}

func NewRunner(stacksRoot string) *Runner {
	if stacksRoot == "" {
		stacksRoot = "/srv/docker-panel/stacks"
	}
	return &Runner{
		stacksRoot: filepath.Clean(stacksRoot),
	}
}

func (r *Runner) ValidateStackName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("stack name cannot be empty")
	}
	if !validStackNamePattern.MatchString(name) {
		return errors.New("stack name may only contain alphanumeric characters, hyphens, and underscores")
	}
	return nil
}

func (r *Runner) GetStackDir(name string) (string, error) {
	if err := r.ValidateStackName(name); err != nil {
		return "", err
	}
	dir := filepath.Join(r.stacksRoot, name)
	clean := filepath.Clean(dir)

	// Guard against directory traversal
	rel, err := filepath.Rel(r.stacksRoot, clean)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return "", errors.New("invalid stack directory path: traversal detected")
	}
	return clean, nil
}

func (r *Runner) ValidateComposeYAML(content string) error {
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		return fmt.Errorf("invalid YAML syntax: %w", err)
	}
	if services, ok := parsed["services"]; !ok || services == nil {
		return errors.New("compose file must define 'services' section")
	}
	return nil
}

func (r *Runner) SaveStackFiles(name string, composeContent string, envContent string) error {
	dir, err := r.GetStackDir(name)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create stack directory: %w", err)
	}

	// Atomic write compose.yaml
	composePath := filepath.Join(dir, "compose.yaml")
	tmpCompose := composePath + ".tmp"
	if err := os.WriteFile(tmpCompose, []byte(composeContent), 0644); err != nil {
		return fmt.Errorf("failed to write compose temp file: %w", err)
	}
	if err := os.Rename(tmpCompose, composePath); err != nil {
		_ = os.Remove(tmpCompose)
		return fmt.Errorf("failed to commit compose file: %w", err)
	}

	// Write .env if provided
	if envContent != "" {
		envPath := filepath.Join(dir, ".env")
		tmpEnv := envPath + ".tmp"
		if err := os.WriteFile(tmpEnv, []byte(envContent), 0600); err != nil {
			return fmt.Errorf("failed to write .env temp file: %w", err)
		}
		if err := os.Rename(tmpEnv, envPath); err != nil {
			_ = os.Remove(tmpEnv)
			return fmt.Errorf("failed to commit .env file: %w", err)
		}
	}

	return nil
}

func (r *Runner) ReadStackFiles(name string) (composeContent string, envContent string, err error) {
	dir, err := r.GetStackDir(name)
	if err != nil {
		return "", "", err
	}

	composeFile := filepath.Join(dir, "compose.yaml")
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		composeFile = filepath.Join(dir, "docker-compose.yaml")
		if _, err := os.Stat(composeFile); os.IsNotExist(err) {
			composeFile = filepath.Join(dir, "docker-compose.yml")
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				return "", "", fmt.Errorf("no compose file found for stack '%s'", name)
			}
		}
	}

	cBytes, err := os.ReadFile(composeFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to read compose file: %w", err)
	}

	envFile := filepath.Join(dir, ".env")
	eBytes, _ := os.ReadFile(envFile)

	return string(cBytes), string(eBytes), nil
}

func (r *Runner) Execute(ctx context.Context, name string, action string, removeVolumes bool) (string, error) {
	dir, err := r.GetStackDir(name)
	if err != nil {
		return "", err
	}

	var args []string
	switch action {
	case "up":
		args = []string{"compose", "-p", name, "up", "-d", "--remove-orphans"}
	case "down":
		args = []string{"compose", "-p", name, "down"}
		if removeVolumes {
			args = append(args, "-v")
		}
	case "restart":
		args = []string{"compose", "-p", name, "restart"}
	case "pull":
		args = []string{"compose", "-p", name, "pull"}
	case "build":
		args = []string{"compose", "-p", name, "build"}
	default:
		return "", fmt.Errorf("unknown compose action: %s", action)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	output := strings.TrimSpace(outBuf.String() + "\n" + errBuf.String())

	if runErr != nil {
		return output, fmt.Errorf("docker compose %s failed: %w (output: %s)", action, runErr, output)
	}

	return output, nil
}
