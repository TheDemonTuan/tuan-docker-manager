export type Role = 'VIEWER' | 'OPERATOR' | 'ADMIN' | 'OWNER'

export interface User {
  id: string
  email: string
  role: Role
  created_at: string
  last_login_at?: string
}

export interface AuthResponse {
  user: User
  permissions: string[]
}

export interface PortMapping {
  ip?: string
  private_port: number
  public_port?: number
  type: string
  exposure: 'PUBLIC' | 'LOCALHOST' | 'INTERNAL'
}

export interface ContainerInfo {
  id: string
  names: string[]
  image: string
  image_id: string
  command: string
  created: number
  state: string
  status: string
  ports: PortMapping[]
  labels: Record<string, string>
  stack_name?: string
  service_name?: string
  health?: string
}

export interface ContainerDetail extends ContainerInfo {
  started_at: string
  finished_at: string
  restart_count: number
  restart_policy: string
  ip_address: string
  networks: string[]
  mounts: Array<{
    type: string
    source: string
    destination: string
    mode: string
    rw: boolean
  }>
  env?: string[]
}

export interface ContainerStats {
  container_id: string
  timestamp: string
  cpu_percent: number
  memory_used: number
  memory_limit: number
  memory_percent: number
  pids: number
  net_rx_bytes: number
  net_tx_bytes: number
  block_read: number
  block_write: number
}

export interface Stack {
  id: string
  name: string
  status: 'running' | 'stopped' | 'partial' | 'unknown'
  path: string
  compose_content: string
  env_content: string
  security_score: number
  is_system: boolean
  containers?: ContainerInfo[]
  created_at: string
  updated_at: string
}

export interface StackRevision {
  id: number
  stack_id: string
  created_at: string
  user_id: string
  user_email: string
  content: string
  env_content: string
  sha256: string
  message: string
}

export interface Finding {
  service: string
  rule: string
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFO'
  message: string
  recommendation: string
}

export interface PortExposure {
  service: string
  host_ip?: string
  host_port: number
  container_port: number
  protocol: string
  exposure: 'PUBLIC' | 'LOCALHOST' | 'INTERNAL'
}

export interface SecurityReport {
  score: number
  status: 'SAFE' | 'WARNING' | 'DANGEROUS'
  findings: Finding[]
  exposures: PortExposure[]
  passed_checks: string[]
}

export interface HostMetrics {
  timestamp: string
  cpu_percent: number
  memory_used: number
  memory_total: number
  memory_percent: number
  swap_used: number
  swap_total: number
  disk_used: number
  disk_total: number
  disk_percent: number
  load_1: number
  load_5: number
  load_15: number
  uptime_seconds: number
  net_rx_bytes_rate: number
  net_tx_bytes_rate: number
}

export interface GPUInfo {
  id: number
  name: string
  utilization: number
  vram_used: number
  vram_total: number
  temperature: number
  power_draw: number
  power_limit: number
  fan_speed: number
}

export interface GPUMetrics {
  available: boolean
  driver_version?: string
  gpus: GPUInfo[]
}

export interface ImageInfo {
  id: string
  repo_tags: string[]
  created: number
  size: number
  containers: number
}

export interface VolumeInfo {
  name: string
  driver: string
  mountpoint: string
  created_at: string
  labels: Record<string, string>
}

export interface NetworkInfo {
  id: string
  name: string
  driver: string
  scope: string
  internal: boolean
}

export interface Job {
  id: string
  type: string
  stack_id?: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  error?: string
  logs: string
  started_at: string
  completed_at?: string
  created_by: string
}

export interface DockerEvent {
  type: string
  action: string
  actor_id: string
  actor_name: string
  timestamp: string
  message: string
}

export interface AuditLog {
  id: number
  user_email: string
  action: string
  resource: string
  ip: string
  metadata: string
  result: string
  created_at: string
}

export interface AlertRule {
  id: number
  name: string
  metric_type: string
  condition: string
  threshold: number
  duration_seconds: number
  enabled: boolean
  severity: string
}

export interface AlertEvent {
  id: number
  rule_id: number
  rule_name: string
  severity: string
  status: string
  message: string
  started_at: string
  resolved_at?: string
}

export interface BackupRecord {
  id: string
  filename: string
  size_bytes: number
  encrypted: boolean
  created_at: string
}

export interface DashboardData {
  host: HostMetrics
  gpu: GPUMetrics
  stacks: Stack[]
  containers: ContainerInfo[]
  summary: {
    total_stacks: number
    running_stacks: number
    total_containers: number
    running_containers: number
    stopped_containers: number
    average_security_score: number
  }
}
