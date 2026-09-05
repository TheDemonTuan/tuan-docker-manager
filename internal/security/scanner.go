package security

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityInfo     Severity = "INFO"
)

type Finding struct {
	Service        string   `json:"service"`
	Rule           string   `json:"rule"`
	Severity       Severity `json:"severity"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation"`
}

type PortExposure struct {
	Service       string `json:"service"`
	HostIP        string `json:"host_ip,omitempty"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	Exposure      string `json:"exposure"` // PUBLIC, LOCALHOST, INTERNAL
}

type SecurityReport struct {
	Score        int            `json:"score"`
	Status       string         `json:"status"` // SAFE, WARNING, DANGEROUS
	Findings     []Finding      `json:"findings"`
	Exposures    []PortExposure `json:"exposures"`
	PassedChecks []string       `json:"passed_checks"`
}

type ComposeScanner struct {
	AllowedHostPaths []string
}

func NewScanner(allowedPaths []string) *ComposeScanner {
	cleanPaths := make([]string, 0, len(allowedPaths))
	for _, p := range allowedPaths {
		if p != "" {
			cleanPaths = append(cleanPaths, filepath.Clean(p))
		}
	}
	return &ComposeScanner{
		AllowedHostPaths: cleanPaths,
	}
}

type composeService struct {
	Image       string         `yaml:"image"`
	Privileged  bool           `yaml:"privileged"`
	Pid         string         `yaml:"pid"`
	Ipc         string         `yaml:"ipc"`
	NetworkMode string         `yaml:"network_mode"`
	ReadOnly    bool           `yaml:"read_only"`
	CapAdd      []string       `yaml:"cap_add"`
	CapDrop     []string       `yaml:"cap_drop"`
	Devices     []string       `yaml:"devices"`
	Ports       []any          `yaml:"ports"`
	Volumes     []any          `yaml:"volumes"`
	Deploy      *composeDeploy `yaml:"deploy"`
	MemLimit    string         `yaml:"mem_limit"`
	Cpus        any            `yaml:"cpus"`
}

type composeDeploy struct {
	Resources *composeResources `yaml:"resources"`
}

type composeResources struct {
	Limits *composeLimits `yaml:"limits"`
}

type composeLimits struct {
	Cpus   any    `yaml:"cpus"`
	Memory string `yaml:"memory"`
}

type composeFile struct {
	Version  string                    `yaml:"version"`
	Services map[string]composeService `yaml:"services"`
	Volumes  map[string]any            `yaml:"volumes"`
	Networks map[string]any            `yaml:"networks"`
}

func (s *ComposeScanner) Scan(yamlContent string, stackRoot string) (*SecurityReport, error) {
	var cf composeFile
	if err := yaml.Unmarshal([]byte(yamlContent), &cf); err != nil {
		return nil, fmt.Errorf("invalid YAML syntax: %w", err)
	}

	report := &SecurityReport{
		Score:        100,
		Findings:     make([]Finding, 0),
		Exposures:    make([]PortExposure, 0),
		PassedChecks: make([]string, 0),
	}

	if len(cf.Services) == 0 {
		report.Status = "SAFE"
		return report, nil
	}

	hasPrivileged := false
	hasDockerSock := false
	hasHostFS := false
	hasDangerousCaps := false
	hasHostNet := false
	hasPublicPort := false

	effectiveAllowed := make([]string, 0, len(s.AllowedHostPaths)+1)
	effectiveAllowed = append(effectiveAllowed, s.AllowedHostPaths...)
	if stackRoot != "" {
		effectiveAllowed = append(effectiveAllowed, filepath.Clean(stackRoot))
	}

	for svcName, svc := range cf.Services {
		// 1. Privileged mode
		if svc.Privileged {
			hasPrivileged = true
			report.Findings = append(report.Findings, Finding{
				Service:        svcName,
				Rule:           "NO_PRIVILEGED_MODE",
				Severity:       SeverityCritical,
				Message:        "Container runs with privileged: true, giving full root capabilities on host.",
				Recommendation: "Disable privileged mode and grant only minimal necessary capabilities.",
			})
			report.Score -= 25
		}

		// 2. Volumes: docker.sock, root FS, disallowed paths
		for _, v := range svc.Volumes {
			volStr, ok := v.(string)
			if !ok {
				continue
			}
			parts := strings.Split(volStr, ":")
			if len(parts) >= 2 {
				hostSource := filepath.Clean(parts[0])
				// check docker.sock
				if strings.Contains(hostSource, "docker.sock") {
					hasDockerSock = true
					report.Findings = append(report.Findings, Finding{
						Service:        svcName,
						Rule:           "NO_DOCKER_SOCKET_MOUNT",
						Severity:       SeverityCritical,
						Message:        "Container mounts /var/run/docker.sock, granting host-level container escape vector.",
						Recommendation: "Remove docker.sock mount; use dedicated isolated agent or API.",
					})
					report.Score -= 25
				}

				// check root filesystem mount
				if hostSource == "/" || hostSource == `\` || hostSource == "/etc" || hostSource == "/root" {
					hasHostFS = true
					report.Findings = append(report.Findings, Finding{
						Service:        svcName,
						Rule:           "NO_ROOT_HOST_MOUNT",
						Severity:       SeverityCritical,
						Message:        fmt.Sprintf("Container mounts sensitive host path: %s", hostSource),
						Recommendation: "Mount only isolated application data directories.",
					})
					report.Score -= 25
				}

				// check allowed paths
				if strings.HasPrefix(hostSource, "/") || strings.Contains(hostSource, `:\`) {
					isAllowed := false
					for _, allowed := range effectiveAllowed {
						rel, err := filepath.Rel(allowed, hostSource)
						if err == nil && !strings.HasPrefix(rel, "..") {
							isAllowed = true
							break
						}
					}
					if !isAllowed && !hasDockerSock && !hasHostFS {
						report.Findings = append(report.Findings, Finding{
							Service:        svcName,
							Rule:           "ALLOWED_HOST_PATHS",
							Severity:       SeverityHigh,
							Message:        fmt.Sprintf("Host mount path '%s' is outside allowed directories.", hostSource),
							Recommendation: "Use named Docker volumes or configure allowed host paths in panel settings.",
						})
						report.Score -= 15
					}
				}
			}
		}

		// 3. Capabilities
		for _, cap := range svc.CapAdd {
			uCap := strings.ToUpper(cap)
			if uCap == "ALL" || uCap == "SYS_ADMIN" || uCap == "SYS_PTRACE" || uCap == "NET_ADMIN" {
				hasDangerousCaps = true
				report.Findings = append(report.Findings, Finding{
					Service:        svcName,
					Rule:           "NO_DANGEROUS_CAPABILITIES",
					Severity:       SeverityCritical,
					Message:        fmt.Sprintf("Container adds dangerous Linux capability: %s", cap),
					Recommendation: "Drop ALL capabilities and only add specific unprivileged capabilities.",
				})
				report.Score -= 20
			}
		}

		// 4. Host namespaces
		if strings.EqualFold(svc.Pid, "host") {
			report.Findings = append(report.Findings, Finding{
				Service:        svcName,
				Rule:           "NO_HOST_PID",
				Severity:       SeverityHigh,
				Message:        "Container shares the host PID namespace.",
				Recommendation: "Remove 'pid: host' to isolate container processes.",
			})
			report.Score -= 15
		}
		if strings.EqualFold(svc.Ipc, "host") {
			report.Findings = append(report.Findings, Finding{
				Service:        svcName,
				Rule:           "NO_HOST_IPC",
				Severity:       SeverityHigh,
				Message:        "Container shares the host IPC namespace.",
				Recommendation: "Remove 'ipc: host' to prevent shared memory attacks.",
			})
			report.Score -= 15
		}
		if strings.EqualFold(svc.NetworkMode, "host") {
			hasHostNet = true
			report.Findings = append(report.Findings, Finding{
				Service:        svcName,
				Rule:           "NO_HOST_NETWORK",
				Severity:       SeverityHigh,
				Message:        "Container uses host networking, bypassing container port isolation.",
				Recommendation: "Use custom bridge network or Cloudflare tunnel.",
			})
			report.Score -= 15
		}

		// 5. Host devices
		if len(svc.Devices) > 0 {
			report.Findings = append(report.Findings, Finding{
				Service:        svcName,
				Rule:           "HOST_DEVICE_EXPOSURE",
				Severity:       SeverityHigh,
				Message:        fmt.Sprintf("Container maps %d host hardware device(s).", len(svc.Devices)),
				Recommendation: "Verify mapped devices are strictly required.",
			})
			report.Score -= 10
		}

		// 6. Ports Exposure analysis
		for _, p := range svc.Ports {
			exp := parsePortExposure(svcName, p)
			if exp != nil {
				report.Exposures = append(report.Exposures, *exp)
				if exp.Exposure == "PUBLIC" {
					hasPublicPort = true
					report.Findings = append(report.Findings, Finding{
						Service:        svcName,
						Rule:           "NO_PUBLIC_PORTS",
						Severity:       SeverityMedium,
						Message:        fmt.Sprintf("Port %d is published on 0.0.0.0 (publicly accessible).", exp.HostPort),
						Recommendation: "Bind port to 127.0.0.1 (e.g. 127.0.0.1:port:port) or use Cloudflare Tunnel.",
					})
					report.Score -= 10
				}
			}
		}

		// 7. Image latest tag
		if svc.Image != "" {
			if strings.HasSuffix(svc.Image, ":latest") || !strings.Contains(svc.Image, ":") {
				report.Findings = append(report.Findings, Finding{
					Service:        svcName,
					Rule:           "PINNED_IMAGE_VERSION",
					Severity:       SeverityMedium,
					Message:        fmt.Sprintf("Image '%s' uses latest or unversioned tag.", svc.Image),
					Recommendation: "Pin image to specific version tag or immutable SHA digest.",
				})
				report.Score -= 5
			}
		}

		// 8. Resource limits
		hasLimits := false
		if svc.MemLimit != "" || svc.Cpus != nil {
			hasLimits = true
		}
		if svc.Deploy != nil && svc.Deploy.Resources != nil && svc.Deploy.Resources.Limits != nil {
			if svc.Deploy.Resources.Limits.Memory != "" || svc.Deploy.Resources.Limits.Cpus != nil {
				hasLimits = true
			}
		}
		if !hasLimits {
			report.Findings = append(report.Findings, Finding{
				Service:        svcName,
				Rule:           "RESOURCE_LIMITS_SET",
				Severity:       SeverityMedium,
				Message:        "No CPU or memory resource limits configured.",
				Recommendation: "Define memory and CPU limits to prevent container OOM or host starvation.",
			})
			report.Score -= 5
		}

		// 9. Read only root FS
		if !svc.ReadOnly {
			report.Findings = append(report.Findings, Finding{
				Service:        svcName,
				Rule:           "READ_ONLY_ROOT_FS",
				Severity:       SeverityLow,
				Message:        "Root filesystem is writable.",
				Recommendation: "Set read_only: true and mount tmpfs for temporary files.",
			})
			report.Score -= 2
		}
	}

	// Record passed checks
	if !hasPrivileged {
		report.PassedChecks = append(report.PassedChecks, "No privileged containers")
	}
	if !hasDockerSock {
		report.PassedChecks = append(report.PassedChecks, "No Docker socket mounts")
	}
	if !hasHostFS {
		report.PassedChecks = append(report.PassedChecks, "No root host filesystem mounts")
	}
	if !hasDangerousCaps {
		report.PassedChecks = append(report.PassedChecks, "No dangerous Linux capabilities")
	}
	if !hasHostNet {
		report.PassedChecks = append(report.PassedChecks, "Isolated network namespaces")
	}
	if !hasPublicPort {
		report.PassedChecks = append(report.PassedChecks, "Zero publicly exposed ports")
	}

	if report.Score < 0 {
		report.Score = 0
	}

	if report.Score >= 80 {
		report.Status = "SAFE"
	} else if report.Score >= 50 {
		report.Status = "WARNING"
	} else {
		report.Status = "DANGEROUS"
	}

	return report, nil
}

func parsePortExposure(service string, portEntry any) *PortExposure {
	switch v := portEntry.(type) {
	case string:
		parts := strings.Split(v, ":")
		if len(parts) == 1 {
			cp, _ := strconv.Atoi(parts[0])
			return &PortExposure{
				Service:       service,
				ContainerPort: cp,
				Exposure:      "INTERNAL",
				Protocol:      "tcp",
			}
		} else if len(parts) == 2 {
			hp, _ := strconv.Atoi(parts[0])
			cp, _ := strconv.Atoi(parts[1])
			return &PortExposure{
				Service:       service,
				HostPort:      hp,
				ContainerPort: cp,
				Exposure:      "PUBLIC",
				Protocol:      "tcp",
			}
		} else if len(parts) == 3 {
			ip := parts[0]
			hp, _ := strconv.Atoi(parts[1])
			cp, _ := strconv.Atoi(parts[2])
			exposure := "PUBLIC"
			if ip == "127.0.0.1" || ip == "localhost" || ip == "::1" {
				exposure = "LOCALHOST"
			}
			return &PortExposure{
				Service:       service,
				HostIP:        ip,
				HostPort:      hp,
				ContainerPort: cp,
				Exposure:      exposure,
				Protocol:      "tcp",
			}
		}
	case int:
		return &PortExposure{
			Service:       service,
			ContainerPort: v,
			Exposure:      "INTERNAL",
			Protocol:      "tcp",
		}
	}
	return nil
}
