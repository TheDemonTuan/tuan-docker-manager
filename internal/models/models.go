package models

import (
	"time"
)

type Role string

const (
	RoleViewer   Role = "VIEWER"
	RoleOperator Role = "OPERATOR"
	RoleAdmin    Role = "ADMIN"
	RoleOwner    Role = "OWNER"
)

type User struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Role        Role       `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Host struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Hostname   string    `json:"hostname"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type StackStatus string

const (
	StackStatusRunning StackStatus = "running"
	StackStatusStopped StackStatus = "stopped"
	StackStatusPartial StackStatus = "partial"
	StackStatusUnknown StackStatus = "unknown"
)

type Stack struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Status            StackStatus     `json:"status"`
	Path              string          `json:"path"`
	WorkingDir        string          `json:"working_dir,omitempty"`
	ComposeFile       string          `json:"compose_file,omitempty"`
	ComposeContent    string          `json:"compose_content"`
	EnvContent        string          `json:"env_content"`
	DockerfileContent string          `json:"dockerfile_content,omitempty"`
	SecurityScore     int             `json:"security_score"`
	IsSystem          bool            `json:"is_system"`
	Containers        []ContainerInfo `json:"containers,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type StackRevision struct {
	ID         int64     `json:"id"`
	StackID    string    `json:"stack_id"`
	CreatedAt  time.Time `json:"created_at"`
	UserID     string    `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	Content    string    `json:"content"`
	EnvContent string    `json:"env_content"`
	SHA256     string    `json:"sha256"`
	Message    string    `json:"message"`
}

type PortMapping struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
	Exposure    string `json:"exposure"` // PUBLIC, LOCALHOST, INTERNAL
}

type ContainerInfo struct {
	ID          string            `json:"id"`
	Names       []string          `json:"names"`
	Image       string            `json:"image"`
	ImageID     string            `json:"image_id"`
	Command     string            `json:"command"`
	Created     int64             `json:"created"`
	State       string            `json:"state"`
	Status      string            `json:"status"`
	Ports       []PortMapping     `json:"ports"`
	Labels      map[string]string `json:"labels"`
	StackName   string            `json:"stack_name,omitempty"`
	ServiceName string            `json:"service_name,omitempty"`
	ComposeFile string            `json:"compose_file,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	Health      string            `json:"health,omitempty"`
	SizeRw      *int64            `json:"size_rw,omitempty"`
	SizeRootFS  *int64            `json:"size_root_fs,omitempty"`
}

type StorageSnapshot struct {
	MeasuredAt time.Time                   `json:"measured_at"`
	Containers map[string]ContainerStorage `json:"containers"`
	Volumes    map[string]VolumeStorage    `json:"volumes"`
	Stacks     map[string]StackStorage     `json:"stacks"`
	Status     string                      `json:"status"`
	Scope      string                      `json:"scope"`
}

type ContainerStorage struct {
	WritableBytes *int64   `json:"writable_bytes,omitempty"`
	RootFSBytes   *int64   `json:"root_fs_bytes,omitempty"`
	VolumeNames   []string `json:"volume_names,omitempty"`
}

type VolumeStorage struct {
	Bytes      *int64   `json:"bytes,omitempty"`
	RefCount   int      `json:"ref_count"`
	StackNames []string `json:"stack_names,omitempty"`
}

type StackStorage struct {
	WritableBytes        int64    `json:"writable_bytes"`
	ExclusiveVolumeBytes int64    `json:"exclusive_volume_bytes"`
	SharedVolumeBytes    int64    `json:"shared_volume_bytes"`
	VolumeNames          []string `json:"volume_names,omitempty"`
	Incomplete           bool     `json:"incomplete"`
}

type ContainerDetail struct {
	ContainerInfo
	StartedAt     string         `json:"started_at"`
	FinishedAt    string         `json:"finished_at"`
	RestartCount  int            `json:"restart_count"`
	RestartPolicy string         `json:"restart_policy"`
	IPAddress     string         `json:"ip_address"`
	Networks      []string       `json:"networks"`
	Mounts        []MountDetail  `json:"mounts"`
	Env           []string       `json:"env,omitempty"`
	Args          []string       `json:"args,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
	HostConfig    map[string]any `json:"host_config,omitempty"`
	StateDetail   map[string]any `json:"state_detail,omitempty"`
}

type MountDetail struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}

type ContainerStats struct {
	ContainerID   string    `json:"container_id"`
	Timestamp     time.Time `json:"timestamp"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryUsed    uint64    `json:"memory_used"`
	MemoryLimit   uint64    `json:"memory_limit"`
	MemoryPercent float64   `json:"memory_percent"`
	PIDs          uint32    `json:"pids"`
	NetRxBytes    uint64    `json:"net_rx_bytes"`
	NetTxBytes    uint64    `json:"net_tx_bytes"`
	BlockRead     uint64    `json:"block_read"`
	BlockWrite    uint64    `json:"block_write"`
}

type ImageInfo struct {
	ID          string            `json:"id"`
	RepoTags    []string          `json:"repo_tags"`
	RepoDigests []string          `json:"repo_digests"`
	Created     int64             `json:"created"`
	Size        int64             `json:"size"`
	SharedSize  int64             `json:"shared_size"`
	Containers  int64             `json:"containers"`
	Labels      map[string]string `json:"labels"`
}

type VolumeInfo struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	CreatedAt  string            `json:"created_at"`
	Status     map[string]any    `json:"status,omitempty"`
	Labels     map[string]string `json:"labels"`
	Scope      string            `json:"scope"`
	Size       int64             `json:"size,omitempty"`
	RefCount   int               `json:"ref_count,omitempty"`
}

type NetworkInfo struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Scope      string            `json:"scope"`
	EnableIPv6 bool              `json:"enable_ipv6"`
	Internal   bool              `json:"internal"`
	Attachable bool              `json:"attachable"`
	IPAM       map[string]any    `json:"ipam,omitempty"`
	Containers map[string]any    `json:"containers,omitempty"`
	Labels     map[string]string `json:"labels"`
}

type HostMetrics struct {
	Timestamp      time.Time `json:"timestamp"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryUsed     uint64    `json:"memory_used"`
	MemoryTotal    uint64    `json:"memory_total"`
	MemoryPercent  float64   `json:"memory_percent"`
	SwapUsed       uint64    `json:"swap_used"`
	SwapTotal      uint64    `json:"swap_total"`
	DiskUsed       uint64    `json:"disk_used"`
	DiskTotal      uint64    `json:"disk_total"`
	DiskPercent    float64   `json:"disk_percent"`
	Load1          float64   `json:"load_1"`
	Load5          float64   `json:"load_5"`
	Load15         float64   `json:"load_15"`
	UptimeSeconds  uint64    `json:"uptime_seconds"`
	NetRxBytesRate float64   `json:"net_rx_bytes_rate"`
	NetTxBytesRate float64   `json:"net_tx_bytes_rate"`
	DiskReadRate   float64   `json:"disk_read_rate"`
	DiskWriteRate  float64   `json:"disk_write_rate"`
}

type GPUMetrics struct {
	Available     bool      `json:"available"`
	DriverVersion string    `json:"driver_version,omitempty"`
	GPUs          []GPUInfo `json:"gpus"`
}

type GPUInfo struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Utilization float64 `json:"utilization"` // percentage
	VRAMUsed    uint64  `json:"vram_used"`   // bytes
	VRAMTotal   uint64  `json:"vram_total"`  // bytes
	Temperature float64 `json:"temperature"` // Celsius
	PowerDraw   float64 `json:"power_draw"`  // Watts
	PowerLimit  float64 `json:"power_limit"` // Watts
	FanSpeed    float64 `json:"fan_speed"`   // percentage
}

type DockerEvent struct {
	Type      string    `json:"type"`
	Action    string    `json:"action"`
	ActorID   string    `json:"actor_id"`
	ActorName string    `json:"actor_name"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

type AuditLog struct {
	ID        int64     `json:"id"`
	UserEmail string    `json:"user_email"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	IP        string    `json:"ip"`
	Metadata  string    `json:"metadata"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
}

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type Job struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	StackID     string     `json:"stack_id,omitempty"`
	Status      JobStatus  `json:"status"`
	Error       string     `json:"error,omitempty"`
	Logs        string     `json:"logs"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedBy   string     `json:"created_by"`
}

type AlertRule struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	MetricType      string    `json:"metric_type"` // cpu, memory, disk, gpu_temp, container_unhealthy, container_down
	Condition       string    `json:"condition"`   // gt, lt, eq
	Threshold       float64   `json:"threshold"`
	DurationSeconds int       `json:"duration_seconds"`
	Enabled         bool      `json:"enabled"`
	Severity        string    `json:"severity"` // info, warning, critical
	CreatedAt       time.Time `json:"created_at"`
}

type AlertEvent struct {
	ID         int64      `json:"id"`
	RuleID     int64      `json:"rule_id"`
	RuleName   string     `json:"rule_name"`
	Severity   string     `json:"severity"`
	Status     string     `json:"status"` // firing, resolved
	Message    string     `json:"message"`
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

type SecretMetadata struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BackupRecord struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	SizeBytes int64     `json:"size_bytes"`
	Encrypted bool      `json:"encrypted"`
	CreatedAt time.Time `json:"created_at"`
}

type PruneResult struct {
	ImagesDeleted  []string `json:"images_deleted"`
	SpaceReclaimed uint64   `json:"space_reclaimed"`
}

type PruneVolumesResult struct {
	VolumesDeleted []string `json:"volumes_deleted"`
	SpaceReclaimed uint64   `json:"space_reclaimed"`
}
