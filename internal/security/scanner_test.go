package security

import (
	"testing"
)

func TestScanComposeSecurity_HighRisk(t *testing.T) {
	dangerousYAML := `
services:
  evil:
    image: alpine:latest
    privileged: true
    network_mode: host
    pid: host
    cap_add:
      - SYS_ADMIN
      - ALL
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /:/host:rw
    ports:
      - "8080:80"
`

	scanner := NewScanner([]string{"/srv/docker-panel/stacks"})
	report, err := scanner.Scan(dangerousYAML, "/srv/docker-panel/stacks/evil")
	if err != nil {
		t.Fatalf("unexpected error scanning dangerous YAML: %v", err)
	}

	if report.Score >= 50 {
		t.Errorf("expected low security score for dangerous compose, got %d", report.Score)
	}

	var foundPrivileged, foundDockerSock, foundRootMount, foundHostPID, foundHostNet bool
	for _, f := range report.Findings {
		switch f.Rule {
		case "NO_PRIVILEGED_MODE":
			foundPrivileged = true
		case "NO_DOCKER_SOCKET_MOUNT":
			foundDockerSock = true
		case "NO_ROOT_HOST_MOUNT":
			foundRootMount = true
		case "NO_HOST_PID":
			foundHostPID = true
		case "NO_HOST_NETWORK":
			foundHostNet = true
		}
	}

	if !foundPrivileged {
		t.Errorf("expected NO_PRIVILEGED_MODE finding")
	}
	if !foundDockerSock {
		t.Errorf("expected NO_DOCKER_SOCKET_MOUNT finding")
	}
	if !foundRootMount {
		t.Errorf("expected NO_ROOT_HOST_MOUNT finding")
	}
	if !foundHostPID {
		t.Errorf("expected NO_HOST_PID finding")
	}
	if !foundHostNet {
		t.Errorf("expected NO_HOST_NETWORK finding")
	}

	// Verify public port detection
	if len(report.Exposures) != 1 {
		t.Fatalf("expected 1 port exposure, got %d", len(report.Exposures))
	}
	if report.Exposures[0].Exposure != "PUBLIC" {
		t.Errorf("expected PUBLIC exposure, got %s", report.Exposures[0].Exposure)
	}
}

func TestScanComposeSecurity_Safe(t *testing.T) {
	safeYAML := `
services:
  web:
    image: nginx:1.27.0-alpine
    read_only: true
    user: "1000:1000"
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    ports:
      - "127.0.0.1:8080:80"
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
`

	scanner := NewScanner([]string{"/srv/docker-panel/stacks"})
	report, err := scanner.Scan(safeYAML, "/srv/docker-panel/stacks/web")
	if err != nil {
		t.Fatalf("unexpected error scanning safe YAML: %v", err)
	}

	if report.Score < 80 {
		t.Errorf("expected high security score for safe compose, got %d", report.Score)
	}

	// Verify port exposure is LOCALHOST
	if len(report.Exposures) != 1 {
		t.Fatalf("expected 1 port exposure, got %d", len(report.Exposures))
	}
	if report.Exposures[0].Exposure != "LOCALHOST" {
		t.Errorf("expected LOCALHOST exposure, got %s", report.Exposures[0].Exposure)
	}
}

func TestScanComposeSecurity_InvalidYAML(t *testing.T) {
	invalidYAML := `
services:
  broken: [unterminated
`
	scanner := NewScanner([]string{"/srv/docker-panel/stacks"})
	_, err := scanner.Scan(invalidYAML, "/srv/docker-panel/stacks/broken")
	if err == nil {
		t.Errorf("expected error for invalid YAML syntax, got nil")
	}
}
