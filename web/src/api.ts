import {
  AuthResponse,
  DashboardData,
  Stack,
  StackRevision,
  SecurityReport,
  ContainerInfo,
  ContainerDetail,
  ContainerStats,
  ImageInfo,
  VolumeInfo,
  NetworkInfo,
  HostMetrics,
  GPUMetrics,
  Job,
  AuditLog,
  AlertRule,
  AlertEvent,
  BackupRecord,
} from './types'

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers || {})
  if (!headers.has('Content-Type') && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetch(url, {
    ...options,
    headers,
  })

  if (!response.ok) {
    let errorMsg = `HTTP Error ${response.status}`
    try {
      const errJson = await response.json()
      if (errJson.error) {
        errorMsg = errJson.error
      }
    } catch {
      // ignore
    }
    throw new Error(errorMsg)
  }

  // Handle 204 or empty response
  if (response.status === 204) {
    return {} as T
  }

  return response.json()
}

export const api = {
  // Auth
  getMe: () => request<AuthResponse>('/api/v1/auth/me'),

  // Dashboard
  getDashboard: () => request<DashboardData>('/api/v1/dashboard'),

  // Stacks
  listStacks: () => request<Stack[]>('/api/v1/stacks'),
  createStack: (data: { name: string; compose_content: string; env_content?: string }) =>
    request<Stack>('/api/v1/stacks', { method: 'POST', body: JSON.stringify(data) }),
  getStack: (id: string) => request<Stack>(`/api/v1/stacks/${id}`),
  deleteStack: (id: string, confirmed = true) =>
    request<{ success: boolean }>(`/api/v1/stacks/${id}`, {
      method: 'DELETE',
      headers: confirmed ? { 'X-Critical-Confirm': '1' } : {},
    }),
  getStackCompose: (id: string) =>
    request<{
      compose_content: string
      env_content: string
      dockerfile_content?: string
      compose_file?: string
      path?: string
    }>(`/api/v1/stacks/${id}/compose`),
  updateStackCompose: (id: string, data: { compose_content: string; env_content?: string; message?: string }) =>
    request<{ stack: Stack; revision: StackRevision; security: SecurityReport }>(`/api/v1/stacks/${id}/compose`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  stackAction: (id: string, action: 'up' | 'down' | 'restart' | 'pull' | 'build', removeVolumes = false) => {
    const url = `/api/v1/stacks/${id}/${action}${removeVolumes ? '?volumes=true' : ''}`
    const headers: Record<string, string> = {}
    if (removeVolumes) {
      headers['X-Critical-Confirm'] = '1'
    }
    return request<Job>(url, { method: 'POST', headers })
  },
  getStackSecurity: (id: string) => request<SecurityReport>(`/api/v1/stacks/${id}/security`),
  getStackRevisions: (id: string) => request<StackRevision[]>(`/api/v1/stacks/${id}/revisions`),
  restoreRevision: (id: string, revId: number) =>
    request<{ stack: Stack; revision: StackRevision }>(`/api/v1/stacks/${id}/revisions/${revId}/restore`, {
      method: 'POST',
    }),
  getStackPreview: (id: string) => request<any>(`/api/v1/stacks/${id}/preview`),

  // Containers
  listContainers: (all = true, stack?: string) => {
    let url = `/api/v1/containers?all=${all}`
    if (stack) url += `&stack=${encodeURIComponent(stack)}`
    return request<ContainerInfo[]>(url)
  },
  getContainer: (id: string) => request<ContainerDetail>(`/api/v1/containers/${id}`),
  containerAction: (id: string, action: 'start' | 'stop' | 'restart' | 'kill') =>
    request<{ success: boolean }>(`/api/v1/containers/${id}/${action}`, { method: 'POST' }),
  getContainerStats: (id: string) => request<ContainerStats>(`/api/v1/containers/${id}/stats`),
  getAllContainerStats: () => request<Record<string, ContainerStats>>('/api/v1/containers/stats-all'),
  getContainerLogsSSE: (id: string, tail = 100, follow = true) => {
    return new EventSource(`/api/v1/containers/${id}/logs?tail=${tail}&follow=${follow}`)
  },
  getContainerTerminalWS: (id: string) => {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return new WebSocket(`${proto}//${window.location.host}/api/v1/containers/${id}/terminal`)
  },

  // Images
  listImages: () => request<ImageInfo[]>('/api/v1/images'),
  pullImage: (image: string) =>
    request<{ success: boolean }>('/api/v1/images/pull', {
      method: 'POST',
      body: JSON.stringify({ image }),
    }),
  deleteImage: (id: string, force = false) =>
    request<{ success: boolean }>(`/api/v1/images/${encodeURIComponent(id)}${force ? '?force=true' : ''}`, {
      method: 'DELETE',
    }),
  pruneImages: () => request<any>('/api/v1/images/prune', { method: 'POST' }),

  // Volumes
  listVolumes: () => request<VolumeInfo[]>('/api/v1/volumes'),
  deleteVolume: (name: string, confirmed = true) =>
    request<{ success: boolean }>(`/api/v1/volumes/${encodeURIComponent(name)}`, {
      method: 'DELETE',
      headers: confirmed ? { 'X-Critical-Confirm': '1' } : {},
    }),
  pruneVolumes: (confirmed = true) =>
    request<any>('/api/v1/volumes/prune', {
      method: 'POST',
      headers: confirmed ? { 'X-Critical-Confirm': '1' } : {},
    }),

  // Networks
  listNetworks: () => request<NetworkInfo[]>('/api/v1/networks'),
  deleteNetwork: (id: string) => request<{ success: boolean }>(`/api/v1/networks/${id}`, { method: 'DELETE' }),

  // Metrics
  getHostMetrics: () => request<HostMetrics>('/api/v1/metrics/host'),
  getHostMetricsHistory: (range: '1h' | '6h' | '24h' | '7d') =>
    request<HostMetrics[]>(`/api/v1/metrics/host?range=${range}`),
  getGPUMetrics: () => request<GPUMetrics>('/api/v1/metrics/gpu'),

  // Audit
  getAuditLogs: (limit = 100) => request<AuditLog[]>(`/api/v1/audit?limit=${limit}`),

  // Jobs
  listJobs: () => request<Job[]>('/api/v1/jobs'),
  getJob: (id: string) => request<Job>(`/api/v1/jobs/${id}`),

  // Alerts
  listAlertRules: () => request<AlertRule[]>('/api/v1/alerts/rules'),
  listAlertEvents: () => request<AlertEvent[]>('/api/v1/alerts/events'),

  // Backups
  listBackups: () => request<BackupRecord[]>('/api/v1/backups'),
  createBackup: () => request<BackupRecord>('/api/v1/backups', { method: 'POST' }),

  // Settings
  getSettings: () => request<Record<string, string>>('/api/v1/settings'),
  updateSettings: (data: Record<string, string>) =>
    request<{ success: boolean }>('/api/v1/settings', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
}
