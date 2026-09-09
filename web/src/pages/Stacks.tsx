import React, { useEffect, useState } from 'react'
import { api } from '../api'
import { Stack, StackRevision, SecurityReport, Job, ContainerStats, StorageSnapshot } from '../types'
import {
  IconLayers,
  IconPlus,
  IconPlay,
  IconSquare,
  IconRotateCw,
  IconTrash,
  IconFileText,
  IconShield,
  IconHistory,
  IconBox,
  IconDownload,
  IconCheckCircle,
  IconAlertCircle,
} from '../components/Icons'
import { Modal, CriticalConfirmModal } from '../components/Modal'

interface StacksProps {
  selectedStackId?: string | null
  onClearSelected?: () => void
}

export const Stacks: React.FC<StacksProps> = ({ selectedStackId, onClearSelected }) => {
  const [stacks, setStacks] = useState<Stack[]>([])
  const [activeStack, setActiveStack] = useState<Stack | null>(null)
  const [activeTab, setActiveTab] = useState<'overview' | 'compose' | 'dockerfile' | 'containers' | 'revisions' | 'security'>('overview')

  // Edit compose state
  const [composeContent, setComposeContent] = useState('')
  const [envContent, setEnvContent] = useState('')
  const [dockerfileContent, setDockerfileContent] = useState('')
  const [saveMessage, setSaveMessage] = useState('')
  const [saving, setSaving] = useState(false)
  const [saveFeedback, setSaveFeedback] = useState<string | null>(null)

  // Security report
  const [securityReport, setSecurityReport] = useState<SecurityReport | null>(null)

  // Revisions
  const [revisions, setRevisions] = useState<StackRevision[]>([])
  const [selectedRev, setSelectedRev] = useState<StackRevision | null>(null)

  // Jobs
  const [activeJob, setActiveJob] = useState<Job | null>(null)
  const [storage, setStorage] = useState<StorageSnapshot | null>(null)

  // Modals
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [newStackName, setNewStackName] = useState('')
  const [newCompose, setNewCompose] = useState('version: "3.8"\nservices:\n  web:\n    image: nginx:1.27-alpine\n    ports:\n      - "127.0.0.1:8080:80"\n    restart: unless-stopped\n')
  const [newEnv, setNewEnv] = useState('')

  const [allStats, setAllStats] = useState<Record<string, ContainerStats>>({})

  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }

  const loadStorage = async () => {
    try {
      setStorage(await api.getStorage())
    } catch {
      // Storage details are optional; stack management remains available.
    }
  }

  const loadAllStats = async () => {
    try {
      const res = await api.getAllContainerStats()
      if (res) setAllStats(res)
    } catch {
      // ignore
    }
  }

  const [isDeleteOpen, setIsDeleteOpen] = useState(false)
  const [isDownVOpen, setIsDownVOpen] = useState(false)
  const [isPreviewOpen, setIsPreviewOpen] = useState(false)
  const [previewData, setPreviewData] = useState<any>(null)

  const loadStacks = async () => {
    try {
      const list = await api.listStacks()
      setStacks(list)
      if (selectedStackId) {
        const found = list.find((s) => s.id === selectedStackId)
        if (found) {
          selectStack(found)
        }
      }
    } catch (err: any) {
      console.error(err)
    }
  }

  useEffect(() => {
    loadStacks()
    loadAllStats()
    loadStorage()
    const timer = setInterval(() => {
      loadStacks()
      loadAllStats()
      loadStorage()
    }, 4000)
    return () => clearInterval(timer)
  }, [selectedStackId])

  const selectStack = async (stk: Stack) => {
    setActiveStack(stk)
    setComposeContent(stk.compose_content || '')
    setEnvContent(stk.env_content || '')
    setDockerfileContent(stk.dockerfile_content || '')
    setSaveFeedback(null)

    try {
      const cmp = await api.getStackCompose(stk.id)
      if (cmp.compose_content) setComposeContent(cmp.compose_content)
      if (cmp.env_content) setEnvContent(cmp.env_content)
      if (cmp.dockerfile_content) setDockerfileContent(cmp.dockerfile_content)
    } catch {
      // ignore
    }

    // Fetch security
    try {
      const sec = await api.getStackSecurity(stk.id)
      setSecurityReport(sec)
    } catch {
      // ignore
    }

    // Fetch revisions
    try {
      const revs = await api.getStackRevisions(stk.id)
      setRevisions(revs)
    } catch {
      // ignore
    }
  }

  const handleSaveCompose = async () => {
    if (!activeStack) return
    setSaving(true)
    setSaveFeedback(null)
    try {
      const res = await api.updateStackCompose(activeStack.id, {
        compose_content: composeContent,
        env_content: envContent,
        message: saveMessage || 'Updated compose config',
      })
      setActiveStack(res.stack)
      setSecurityReport(res.security)
      setSaveMessage('')
      setSaveFeedback('Compose configuration saved and revision created successfully.')
      loadStacks()
      // Refresh revisions
      const revs = await api.getStackRevisions(activeStack.id)
      setRevisions(revs)
    } catch (err: any) {
      setSaveFeedback(`Error: ${err.message}`)
    } finally {
      setSaving(false)
    }
  }

  const handleRestoreRevision = async (revId: number) => {
    if (!activeStack) return
    try {
      const res = await api.restoreRevision(activeStack.id, revId)
      setActiveStack(res.stack)
      setComposeContent(res.stack.compose_content)
      setEnvContent(res.stack.env_content)
      setSelectedRev(null)
      loadStacks()
      const sec = await api.getStackSecurity(res.stack.id)
      setSecurityReport(sec)
    } catch (err: any) {
      alert(`Restore failed: ${err.message}`)
    }
  }

  const handleStackAction = async (action: 'up' | 'down' | 'restart' | 'pull' | 'build', removeVolumes = false) => {
    if (!activeStack) return
    try {
      const job = await api.stackAction(activeStack.id, action, removeVolumes)
      setActiveJob(job)
      pollJob(job.id)
    } catch (err: any) {
      alert(`Action failed: ${err.message}`)
    }
  }

  const pollJob = (jobId: string) => {
    const timer = setInterval(async () => {
      try {
        const j = await api.getJob(jobId)
        setActiveJob(j)
        if (j.status === 'completed' || j.status === 'failed') {
          clearInterval(timer)
          loadStacks()
          if (activeStack) {
            const updated = await api.getStack(activeStack.id)
            setActiveStack(updated)
          }
        }
      } catch {
        clearInterval(timer)
      }
    }, 1500)
  }

  const handleCreateStack = async () => {
    if (!newStackName.trim()) return
    try {
      const created = await api.createStack({
        name: newStackName.trim(),
        compose_content: newCompose,
        env_content: newEnv,
      })
      setIsCreateOpen(false)
      setNewStackName('')
      loadStacks()
      selectStack(created)
    } catch (err: any) {
      alert(`Create failed: ${err.message}`)
    }
  }

  const handleDeleteStack = async () => {
    if (!activeStack) return
    try {
      await api.deleteStack(activeStack.id, true)
      setIsDeleteOpen(false)
      setActiveStack(null)
      if (onClearSelected) onClearSelected()
      loadStacks()
    } catch (err: any) {
      alert(`Delete failed: ${err.message}`)
    }
  }

  const handleOpenPreview = async () => {
    if (!activeStack) return
    try {
      const prev = await api.getStackPreview(activeStack.id)
      setPreviewData(prev)
      setIsPreviewOpen(true)
    } catch (err: any) {
      alert(`Preview failed: ${err.message}`)
    }
  }

  return (
    <div className="p-6 space-y-6 max-w-7xl mx-auto flex flex-col h-full">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-100 flex items-center gap-2">
            <IconLayers size={22} className="text-cyan-400" />
            Docker Compose Stacks
          </h1>
          <p className="text-xs text-slate-400 mt-0.5">
            Scoped to <span className="font-mono text-slate-300">/srv/docker-panel/stacks/</span>
          </p>
        </div>

        <button
          onClick={() => setIsCreateOpen(true)}
          className="flex items-center gap-2 px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white font-medium text-sm rounded-lg shadow-lg shadow-cyan-600/20 transition-all"
        >
          <IconPlus size={16} /> Create Stack
        </button>
      </div>

      {/* Main Layout */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-1 min-h-0">
        {/* Left: Stacks List */}
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden flex flex-col max-h-[82vh]">
          <div className="p-4 border-b border-slate-800 bg-slate-950/40 flex items-center justify-between">
            <span className="font-semibold text-slate-200 text-sm">All Stacks</span>
            <span className="text-xs text-slate-400 font-mono">{stacks.length}</span>
          </div>

          <div className="divide-y divide-slate-800/60 overflow-y-auto flex-1">
            {stacks.map((stk) => {
              const isSelected = activeStack?.id === stk.id
              const isRunning = stk.status === 'running'
              const stkCpu = (stk.containers || []).reduce((acc, c) => {
                const s = allStats[c.id] || allStats[c.id.slice(0, 12)]
                return acc + (s?.cpu_percent || 0)
              }, 0)
              const stkMem = (stk.containers || []).reduce((acc, c) => {
                const s = allStats[c.id] || allStats[c.id.slice(0, 12)]
                return acc + (s?.memory_used || 0)
              }, 0)
              const stkStorage = storage?.stacks[stk.name]

              return (
                <div
                  key={stk.id}
                  onClick={() => selectStack(stk)}
                  className={`p-4 cursor-pointer transition-colors ${
                    isSelected ? 'bg-cyan-950/30 border-l-4 border-cyan-500' : 'hover:bg-slate-800/40'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-slate-100 text-sm flex items-center gap-2">
                      {stk.name}
                      {stk.is_system && (
                        <span className="text-[9px] uppercase font-bold tracking-wider px-1.5 py-0.2 rounded bg-blue-950 text-blue-300 border border-blue-800">
                          System
                        </span>
                      )}
                    </span>
                    <span
                      className={`text-[11px] font-medium px-2 py-0.5 rounded-full border ${
                        isRunning
                          ? 'bg-emerald-950/60 text-emerald-400 border-emerald-800/60'
                          : stk.status === 'partial'
                          ? 'bg-amber-950/60 text-amber-400 border-amber-800/60'
                          : 'bg-slate-800 text-slate-400 border-slate-700'
                      }`}
                    >
                      {stk.status}
                    </span>
                  </div>

                  <div className="flex items-center justify-between mt-2 text-xs text-slate-400">
                    <span>{stk.containers?.length || 0} containers</span>
                    {stkStorage && (
                      <span className="font-mono text-[11px] text-amber-300 font-medium">
                        Disk {formatBytes(stkStorage.total_bytes > 0 ? stkStorage.total_bytes : stkStorage.writable_bytes + stkStorage.exclusive_volume_bytes)}
                      </span>
                    )}
                    {isRunning && stkCpu > 0 && (
                      <span className="font-mono text-[11px] text-cyan-300">
                        ⚡ {stkCpu.toFixed(1)}% | 💾 {formatBytes(stkMem)}
                      </span>
                    )}
                    <span
                      className={`font-semibold ${
                        stk.security_score >= 80
                          ? 'text-emerald-400'
                          : stk.security_score >= 50
                          ? 'text-amber-400'
                          : 'text-red-400'
                      }`}
                    >
                      Score: {stk.security_score}/100
                    </span>
                  </div>
                </div>
              )
            })}
          </div>
        </div>

        {/* Right: Selected Stack Detail */}
        <div className="lg:col-span-2 bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden flex flex-col max-h-[82vh]">
          {activeStack ? (
            <>
              {/* Stack Header & Actions */}
              <div className="p-4 border-b border-slate-800 bg-slate-950/40 flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div className="flex items-center gap-2">
                    <h2 className="text-lg font-bold text-slate-100">{activeStack.name}</h2>
                    <span
                      className={`text-xs px-2.5 py-0.5 rounded-full font-medium border ${
                        activeStack.status === 'running'
                          ? 'bg-emerald-950/60 text-emerald-400 border-emerald-800/60'
                          : activeStack.status === 'partial'
                          ? 'bg-amber-950/60 text-amber-400 border-amber-800/60'
                          : 'bg-slate-800 text-slate-400 border-slate-700'
                      }`}
                    >
                      {activeStack.status}
                    </span>
                  </div>
                  <div className="text-xs text-slate-400 font-mono mt-0.5">{activeStack.working_dir || activeStack.path}</div>
                </div>

                {/* Operations Bar */}
                <div className="flex flex-wrap items-center gap-2">
                  <button
                    onClick={() => handleStackAction('up')}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg shadow-sm"
                    title="Compose Up (detached)"
                  >
                    <IconPlay size={12} /> Up
                  </button>

                  <button
                    onClick={() => handleStackAction('restart')}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg border border-slate-700"
                    title="Restart Services"
                  >
                    <IconRotateCw size={12} /> Restart
                  </button>

                  <button
                    onClick={() => handleStackAction('pull')}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg border border-slate-700"
                    title="Pull Images"
                  >
                    <IconDownload size={12} /> Pull
                  </button>

                  <button
                    onClick={() => handleOpenPreview()}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-cyan-950 hover:bg-cyan-900 text-cyan-300 border border-cyan-800 rounded-lg"
                  >
                    Preview
                  </button>

                  <button
                    onClick={() => handleStackAction('down')}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-slate-800 hover:bg-slate-700 text-amber-400 rounded-lg border border-slate-700"
                    title="Compose Down (Preserve Volumes)"
                  >
                    <IconSquare size={12} /> Down
                  </button>

                  {!activeStack.is_system && (
                    <button
                      onClick={() => setIsDeleteOpen(true)}
                      className="p-1.5 text-slate-400 hover:text-red-400 hover:bg-red-950/40 rounded-lg transition-colors"
                      title="Delete Stack"
                    >
                      <IconTrash size={16} />
                    </button>
                  )}
                </div>
              </div>

              {storage?.stacks[activeStack.name] && (() => {
                const current = storage.stacks[activeStack.name]
                return (
                  <div className="px-4 py-3 bg-amber-950/30 border-b border-amber-900/60 text-xs">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <span className="font-semibold text-amber-200">
                        Stack Disk Usage: <strong className="font-mono text-amber-300 text-sm">{formatBytes(current.total_bytes > 0 ? current.total_bytes : current.writable_bytes + current.exclusive_volume_bytes)}</strong>
                      </span>
                      <span className="text-amber-400 font-mono text-[11px]">{storage.status === 'partial' || current.incomplete ? 'Partial measurement' : 'Docker Engine snapshot'}</span>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 mt-2 font-mono">
                      <div className="bg-slate-900/80 p-2 rounded border border-slate-800">
                        <span className="text-slate-400 block text-[10px] uppercase">Images</span>
                        <strong className="text-amber-300">{formatBytes(current.image_bytes)}</strong>
                      </div>
                      <div className="bg-slate-900/80 p-2 rounded border border-slate-800">
                        <span className="text-slate-400 block text-[10px] uppercase">Volumes</span>
                        <strong className="text-amber-300">{formatBytes(current.exclusive_volume_bytes)}</strong>
                      </div>
                      <div className="bg-slate-900/80 p-2 rounded border border-slate-800">
                        <span className="text-slate-400 block text-[10px] uppercase">Writable Layer</span>
                        <strong className="text-amber-300">{formatBytes(current.writable_bytes)}</strong>
                      </div>
                      <div className="bg-slate-900/80 p-2 rounded border border-slate-800">
                        <span className="text-slate-400 block text-[10px] uppercase">Shared Volumes</span>
                        <strong className="text-amber-300">{formatBytes(current.shared_volume_bytes)}</strong>
                      </div>
                    </div>
                    {current.volume_names && current.volume_names.length > 0 && (
                      <div className="text-[11px] text-slate-400 mt-2 font-mono">
                        Volumes: {current.volume_names.join(', ')}
                      </div>
                    )}
                    <p className="text-slate-500 mt-1">{storage.scope}</p>
                  </div>
                )
              })()}

              {/* Job Running Banner */}
              {activeJob && activeJob.status === 'running' && (
                <div className="px-4 py-2 bg-cyan-950/70 border-b border-cyan-800 text-cyan-200 text-xs flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full bg-cyan-400 animate-ping"></div>
                    <span>
                      Executing job <span className="font-mono font-bold">{activeJob.type}</span>...
                    </span>
                  </div>
                  <span className="text-[11px] text-cyan-400 font-mono">ID: {activeJob.id}</span>
                </div>
              )}

              {/* Tabs */}
              <div className="flex border-b border-slate-800 bg-slate-950/20 px-4 text-xs font-medium text-slate-400 overflow-x-auto">
                {[
                  { id: 'overview' as const, label: 'Overview' },
                  { id: 'compose' as const, label: 'Compose YAML' },
                  { id: 'dockerfile' as const, label: 'Dockerfile', badge: dockerfileContent ? 'Found' : undefined },
                  { id: 'containers' as const, label: `Containers (${activeStack.containers?.length || 0})` },
                  { id: 'security' as const, label: `Security (${securityReport?.score || activeStack.security_score})` },
                  { id: 'revisions' as const, label: 'Revisions' },
                ].map((t) => (
                  <button
                    key={t.id}
                    onClick={() => setActiveTab(t.id)}
                    className={`py-3 px-4 border-b-2 transition-all flex items-center gap-1.5 whitespace-nowrap ${
                      activeTab === t.id
                        ? 'border-cyan-400 text-cyan-400 font-semibold'
                        : 'border-transparent hover:text-slate-200'
                    }`}
                  >
                    <span>{t.label}</span>
                    {t.badge && (
                      <span className="text-[10px] px-1.5 py-0.2 rounded bg-cyan-950 text-cyan-300 border border-cyan-800 font-mono">
                        {t.badge}
                      </span>
                    )}
                  </button>
                ))}
              </div>

              {/* Tab Contents */}
              <div className="p-4 flex-1 overflow-y-auto space-y-4">
                {/* 1. Overview */}
                {activeTab === 'overview' && (() => {
                  const activeStackCpu = (activeStack.containers || []).reduce((acc, c) => {
                    const s = allStats[c.id] || allStats[c.id.slice(0, 12)]
                    return acc + (s?.cpu_percent || 0)
                  }, 0)
                  const activeStackMem = (activeStack.containers || []).reduce((acc, c) => {
                    const s = allStats[c.id] || allStats[c.id.slice(0, 12)]
                    return acc + (s?.memory_used || 0)
                  }, 0)

                  return (
                    <div className="space-y-4">
                      <div className="p-3 bg-slate-950/50 rounded-lg border border-slate-800 space-y-2">
                        <div className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-1">
                          Stack Location & Compose Files
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
                          <div>
                            <span className="text-slate-500 block text-[11px]">Working Directory:</span>
                            <span className="font-mono text-cyan-400 break-all">{activeStack.working_dir || activeStack.path}</span>
                          </div>
                          <div>
                            <span className="text-slate-500 block text-[11px]">Compose File:</span>
                            <span className="font-mono text-slate-300 break-all">{activeStack.compose_file || 'compose.yaml'}</span>
                          </div>
                        </div>
                      </div>

                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                        <div className="p-3 bg-slate-950/50 rounded-lg border border-slate-800">
                          <div className="text-xs text-slate-400">Security Score</div>
                          <div
                            className={`text-xl font-bold mt-1 ${
                              activeStack.security_score >= 80
                                ? 'text-emerald-400'
                                : activeStack.security_score >= 50
                                ? 'text-amber-400'
                                : 'text-red-400'
                            }`}
                          >
                            {activeStack.security_score} / 100
                          </div>
                          <div className="text-[10px] text-slate-400 mt-0.5">
                            {activeStack.security_score >= 80 ? 'Production Ready' : 'Needs Hardening'}
                          </div>
                        </div>

                        <div className="p-3 bg-slate-950/50 rounded-lg border border-slate-800">
                          <div className="text-xs text-slate-400">Containers</div>
                          <div className="text-xl font-bold text-slate-100 mt-1">{activeStack.containers?.length || 0}</div>
                          <div className="text-[10px] text-slate-400 mt-0.5">{activeStack.status}</div>
                        </div>

                        <div className="p-3 bg-slate-950/50 rounded-lg border border-slate-800">
                          <div className="text-xs text-slate-400">Stack CPU</div>
                          <div className="text-xl font-bold text-cyan-400 font-mono mt-1">
                            ⚡ {activeStackCpu.toFixed(1)}%
                          </div>
                          <div className="text-[10px] text-slate-400 mt-0.5">Live CPU usage</div>
                        </div>

                        <div className="p-3 bg-slate-950/50 rounded-lg border border-slate-800">
                          <div className="text-xs text-slate-400">Stack RAM</div>
                          <div className="text-xl font-bold text-indigo-400 font-mono mt-1">
                            💾 {formatBytes(activeStackMem)}
                          </div>
                          <div className="text-[10px] text-slate-400 mt-0.5">Live RAM usage</div>
                        </div>
                      </div>

                      <div className="p-4 bg-slate-950/40 rounded-lg border border-slate-800">
                        <h4 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                          Containers in Stack
                        </h4>
                        {activeStack.containers && activeStack.containers.length > 0 ? (
                          <div className="space-y-2">
                            {activeStack.containers.map((c) => {
                              const cs = allStats[c.id] || allStats[c.id.slice(0, 12)]
                              return (
                                <div
                                  key={c.id}
                                  className="flex items-center justify-between p-2.5 bg-slate-900/60 rounded border border-slate-800 text-xs"
                                >
                                  <div>
                                    <span className="font-semibold text-slate-200">{c.names.join(', ')}</span>
                                    <span className="ml-2 font-mono text-[11px] text-slate-400">{c.image}</span>
                                  </div>
                                  <div className="flex items-center gap-3">
                                    {cs && c.state === 'running' && (
                                      <div className="font-mono text-[11px] flex items-center gap-2">
                                        <span className="text-cyan-400">⚡ {cs.cpu_percent.toFixed(1)}%</span>
                                        <span className="text-indigo-400">💾 {formatBytes(cs.memory_used)}</span>
                                      </div>
                                    )}
                                    <span
                                      className={`px-2 py-0.5 rounded text-[10px] font-medium ${
                                        c.state === 'running'
                                          ? 'bg-emerald-950 text-emerald-400'
                                          : 'bg-slate-800 text-slate-400'
                                      }`}
                                    >
                                      {c.state}
                                    </span>
                                  </div>
                                </div>
                              )
                            })}
                          </div>
                        ) : (
                          <div className="text-xs text-slate-400">No active containers found for this stack.</div>
                        )}
                      </div>
                    </div>
                  )
                })()}

                {/* 2. Compose Editor */}
                {activeTab === 'compose' && (
                  <div className="space-y-3 flex flex-col h-full">
                    {saveFeedback && (
                      <div
                        className={`p-3 rounded-lg text-xs ${
                          saveFeedback.startsWith('Error')
                            ? 'bg-red-950/50 text-red-200 border border-red-800'
                            : 'bg-emerald-950/50 text-emerald-200 border border-emerald-800'
                        }`}
                      >
                        {saveFeedback}
                      </div>
                    )}

                    <div>
                      <div className="text-xs text-slate-400 mb-1 flex items-center justify-between">
                        <span>compose.yaml</span>
                        <span className="text-[11px] text-slate-400">Validates YAML & Scans Security on Save</span>
                      </div>
                      <textarea
                        value={composeContent}
                        onChange={(e) => setComposeContent(e.target.value)}
                        rows={12}
                        className="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 font-mono text-xs text-slate-100 focus:outline-none focus:border-cyan-500 leading-relaxed resize-none"
                      />
                    </div>

                    <div>
                      <div className="text-xs text-slate-400 mb-1">.env (optional environment variables)</div>
                      <textarea
                        value={envContent}
                        onChange={(e) => setEnvContent(e.target.value)}
                        rows={3}
                        placeholder="KEY=VALUE"
                        className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2 font-mono text-xs text-slate-100 focus:outline-none focus:border-cyan-500 leading-relaxed resize-none"
                      />
                    </div>

                    <div className="flex items-center gap-3">
                      <input
                        type="text"
                        value={saveMessage}
                        onChange={(e) => setSaveMessage(e.target.value)}
                        placeholder="Revision message (e.g. updated redis tag)"
                        className="flex-1 bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-slate-200 focus:outline-none focus:border-cyan-500"
                      />
                      <button
                        onClick={handleSaveCompose}
                        disabled={saving}
                        className="px-4 py-1.5 bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white text-xs font-semibold rounded-lg shadow-sm transition-all"
                      >
                        {saving ? 'Saving...' : 'Save & Validate'}
                      </button>
                    </div>
                  </div>
                )}

                {/* 2.5. Dockerfile Viewer */}
                {activeTab === 'dockerfile' && (
                  <div className="space-y-3 flex flex-col h-full">
                    <div className="flex items-center justify-between">
                      <div className="text-xs text-slate-300 font-medium flex items-center gap-2">
                        <span>Dockerfile</span>
                        <span className="text-[11px] text-cyan-400 font-mono">
                          {activeStack.working_dir ? `${activeStack.working_dir}/Dockerfile` : 'Dockerfile'}
                        </span>
                      </div>
                      {dockerfileContent && (
                        <button
                          onClick={() => navigator.clipboard.writeText(dockerfileContent)}
                          className="px-2.5 py-1 text-xs bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700 font-mono"
                        >
                          Copy
                        </button>
                      )}
                    </div>

                    {dockerfileContent ? (
                      <pre className="flex-1 bg-slate-950 border border-slate-800 rounded-lg p-3 font-mono text-xs text-slate-100 overflow-auto leading-relaxed max-h-[600px] whitespace-pre">
                        {dockerfileContent}
                      </pre>
                    ) : (
                      <div className="text-xs text-slate-400 text-center py-12 bg-slate-950/40 rounded-lg border border-slate-800/60">
                        <p className="font-semibold text-slate-300 mb-1">No Dockerfile Found</p>
                        <p className="text-slate-500">
                          This stack uses pre-built Docker images directly or no Dockerfile was detected in the project directory.
                        </p>
                      </div>
                    )}
                  </div>
                )}

                {/* 3. Containers */}
                {activeTab === 'containers' && (
                  <div className="space-y-3">
                    {activeStack.containers && activeStack.containers.length > 0 ? (
                      activeStack.containers.map((c) => {
                        const cs = allStats[c.id] || allStats[c.id.slice(0, 12)]
                        return (
                          <div
                            key={c.id}
                            className="p-3 bg-slate-950/60 rounded-lg border border-slate-800 flex items-center justify-between"
                          >
                            <div>
                              <div className="font-semibold text-slate-200 text-sm">{c.names.join(', ')}</div>
                              <div className="text-xs text-slate-400 font-mono mt-0.5">{c.image}</div>
                              <div className="text-[11px] text-slate-400 mt-1 flex flex-wrap items-center gap-3">
                                <span
                                  className={
                                    c.state === 'running'
                                      ? 'text-emerald-400 font-medium'
                                      : c.status.toLowerCase().includes('exited (0)')
                                      ? 'text-blue-400 font-medium'
                                      : 'text-slate-400'
                                  }
                                >
                                  Status: {c.status}
                                </span>
                                {cs && c.state === 'running' && (
                                  <>
                                    <span className="text-cyan-400 font-mono">⚡ {cs.cpu_percent.toFixed(1)}% CPU</span>
                                    <span className="text-indigo-400 font-mono">💾 {formatBytes(cs.memory_used)} RAM</span>
                                  </>
                                )}
                              </div>
                            </div>

                            <div className="flex items-center gap-2">
                              <button
                                onClick={() => api.containerAction(c.id, 'restart').then(() => loadStacks())}
                                className="px-2.5 py-1 text-xs bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700"
                              >
                                Restart
                              </button>
                            </div>
                          </div>
                        )
                      })
                    ) : (
                      <div className="text-xs text-slate-400 text-center py-6">
                        No containers currently running for this stack. Click "Up" to start services.
                      </div>
                    )}
                  </div>
                )}

                {/* 4. Security Scan Report */}
                {activeTab === 'security' && (
                  <div className="space-y-4">
                    {securityReport ? (
                      <>
                        <div className="p-4 bg-slate-950/70 border border-slate-800 rounded-lg flex items-center justify-between">
                          <div>
                            <div className="text-xs text-slate-400">Security Score</div>
                            <div
                              className={`text-3xl font-extrabold ${
                                securityReport.score >= 80
                                  ? 'text-emerald-400'
                                  : securityReport.score >= 50
                                  ? 'text-amber-400'
                                  : 'text-red-400'
                              }`}
                            >
                              {securityReport.score} / 100
                            </div>
                          </div>
                          <div
                            className={`px-3 py-1 rounded-full text-xs font-semibold uppercase ${
                              securityReport.status === 'SAFE'
                                ? 'bg-emerald-950 text-emerald-400 border border-emerald-800'
                                : securityReport.status === 'WARNING'
                                ? 'bg-amber-950 text-amber-400 border border-amber-800'
                                : 'bg-red-950 text-red-400 border border-red-800'
                            }`}
                          >
                            {securityReport.status}
                          </div>
                        </div>

                        {/* Network Exposures */}
                        {securityReport.exposures.length > 0 && (
                          <div className="p-3 bg-slate-950/40 rounded-lg border border-slate-800 space-y-2">
                            <div className="text-xs font-semibold text-slate-300 uppercase tracking-wider">
                              Network Exposure Analysis
                            </div>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                              {securityReport.exposures.map((exp, idx) => (
                                <div
                                  key={idx}
                                  className="p-2 bg-slate-900/80 rounded border border-slate-800 flex items-center justify-between text-xs"
                                >
                                  <div>
                                    <span className="font-semibold text-slate-200">{exp.service}</span>
                                    <span className="text-slate-400 font-mono ml-2">
                                      {exp.host_port}:{exp.container_port}
                                    </span>
                                  </div>
                                  <span
                                    className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                                      exp.exposure === 'PUBLIC'
                                        ? 'bg-red-950 text-red-400 border border-red-800'
                                        : exp.exposure === 'LOCALHOST'
                                        ? 'bg-cyan-950 text-cyan-400 border border-cyan-800'
                                        : 'bg-slate-800 text-slate-400'
                                    }`}
                                  >
                                    {exp.exposure}
                                  </span>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}

                        {/* Findings */}
                        {securityReport.findings.length > 0 ? (
                          <div className="space-y-2">
                            <div className="text-xs font-semibold text-slate-300 uppercase tracking-wider">
                              Security Findings ({securityReport.findings.length})
                            </div>
                            {securityReport.findings.map((f, i) => (
                              <div
                                key={i}
                                className="p-3 bg-slate-950/60 rounded-lg border border-slate-800/80 space-y-1 text-xs"
                              >
                                <div className="flex items-center justify-between">
                                  <span className="font-semibold text-slate-200">
                                    [{f.service}] {f.rule}
                                  </span>
                                  <span
                                    className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                                      f.severity === 'CRITICAL'
                                        ? 'bg-red-950 text-red-400'
                                        : f.severity === 'HIGH'
                                        ? 'bg-orange-950 text-orange-400'
                                        : f.severity === 'MEDIUM'
                                        ? 'bg-amber-950 text-amber-400'
                                        : 'bg-slate-800 text-slate-400'
                                    }`}
                                  >
                                    {f.severity}
                                  </span>
                                </div>
                                <div className="text-slate-300">{f.message}</div>
                                <div className="text-slate-400 text-[11px] pt-1">
                                  💡 Recommendation: {f.recommendation}
                                </div>
                              </div>
                            ))}
                          </div>
                        ) : (
                          <div className="p-4 bg-emerald-950/30 border border-emerald-800/50 rounded-lg text-emerald-300 text-xs flex items-center gap-2">
                            <IconCheckCircle size={16} /> All security scanner checks passed!
                          </div>
                        )}
                      </>
                    ) : (
                      <div className="text-xs text-slate-400">Loading security scan report...</div>
                    )}
                  </div>
                )}

                {/* 5. Revisions */}
                {activeTab === 'revisions' && (
                  <div className="space-y-3">
                    <div className="text-xs text-slate-400">
                      Immutable revision history stored in SQLite. Roll back to any prior version anytime.
                    </div>

                    {revisions.map((rev) => (
                      <div
                        key={rev.id}
                        className="p-3 bg-slate-950/60 rounded-lg border border-slate-800 flex items-center justify-between text-xs"
                      >
                        <div>
                          <div className="font-semibold text-slate-200">
                            v{rev.id} &bull; {rev.message}
                          </div>
                          <div className="text-slate-400 text-[11px] mt-0.5">
                            By {rev.user_email} on {new Date(rev.created_at).toLocaleString()}
                          </div>
                          <div className="font-mono text-[10px] text-slate-400 mt-0.5">
                            SHA: {rev.sha256.substring(0, 16)}...
                          </div>
                        </div>

                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => setSelectedRev(rev)}
                            className="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded border border-slate-700"
                          >
                            View Content
                          </button>
                          <button
                            onClick={() => handleRestoreRevision(rev.id)}
                            className="px-2.5 py-1 bg-cyan-950 hover:bg-cyan-900 text-cyan-300 rounded border border-cyan-800 font-medium"
                          >
                            Restore
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </>
          ) : (
            <div className="p-12 text-center text-slate-400 my-auto">
              <IconLayers size={40} className="mx-auto text-slate-400 mb-3" />
              <div className="text-base font-medium text-slate-300">Select a stack to manage</div>
              <div className="text-xs text-slate-400 mt-1">
                Or create a new Docker Compose stack with instant security checks.
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Modal: Create Stack */}
      <Modal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} title="Create Docker Compose Stack">
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">Stack Name</label>
            <input
              type="text"
              value={newStackName}
              onChange={(e) => setNewStackName(e.target.value)}
              placeholder="e.g. immich, ollama, postgres"
              className="w-full bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-100 focus:outline-none focus:border-cyan-500 font-mono"
            />
            <span className="text-[10px] text-slate-400">
              Only alphanumeric characters, hyphens, and underscores are allowed.
            </span>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">Compose YAML</label>
            <textarea
              value={newCompose}
              onChange={(e) => setNewCompose(e.target.value)}
              rows={8}
              className="w-full bg-slate-950 border border-slate-700 rounded-lg p-3 font-mono text-xs text-slate-100 focus:outline-none focus:border-cyan-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">.env (Optional)</label>
            <textarea
              value={newEnv}
              onChange={(e) => setNewEnv(e.target.value)}
              rows={2}
              placeholder="ENV_VAR=value"
              className="w-full bg-slate-950 border border-slate-700 rounded-lg p-2 font-mono text-xs text-slate-100 focus:outline-none focus:border-cyan-500"
            />
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <button
              onClick={() => setIsCreateOpen(false)}
              className="px-4 py-2 text-sm bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg"
            >
              Cancel
            </button>
            <button
              onClick={handleCreateStack}
              disabled={!newStackName.trim() || !newCompose.trim()}
              className="px-4 py-2 text-sm bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-medium rounded-lg shadow-lg shadow-cyan-600/20"
            >
              Create Stack
            </button>
          </div>
        </div>
      </Modal>

      {/* Modal: View Revision Diff/Content */}
      {selectedRev && (
        <Modal
          isOpen={true}
          onClose={() => setSelectedRev(null)}
          title={`Revision #${selectedRev.id} — ${selectedRev.message}`}
        >
          <div className="space-y-3">
            <div className="text-xs text-slate-400 flex justify-between">
              <span>Author: {selectedRev.user_email}</span>
              <span className="font-mono">{selectedRev.sha256}</span>
            </div>
            <textarea
              readOnly
              value={selectedRev.content}
              rows={14}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 font-mono text-xs text-slate-200"
            />
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setSelectedRev(null)}
                className="px-4 py-2 text-xs bg-slate-800 text-slate-300 rounded-lg"
              >
                Close
              </button>
              <button
                onClick={() => handleRestoreRevision(selectedRev.id)}
                className="px-4 py-2 text-xs bg-cyan-600 text-white font-medium rounded-lg"
              >
                Restore This Revision
              </button>
            </div>
          </div>
        </Modal>
      )}

      {/* Modal: Deployment Preview */}
      {isPreviewOpen && previewData && (
        <Modal isOpen={true} onClose={() => setIsPreviewOpen(false)} title={`Deployment Preview: ${previewData.stack_name}`}>
          <div className="space-y-4 text-xs">
            <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
              <div className="font-semibold text-slate-200 mb-1">Services ({previewData.services?.length})</div>
              <div className="flex flex-wrap gap-2">
                {previewData.services?.map((s: string) => (
                  <span key={s} className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono">
                    {s}
                  </span>
                ))}
              </div>
            </div>

            <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
              <div className="font-semibold text-slate-200 mb-1">Images ({previewData.images?.length})</div>
              <div className="flex flex-wrap gap-2">
                {previewData.images?.map((img: string) => (
                  <span key={img} className="px-2 py-0.5 rounded bg-cyan-950 text-cyan-300 border border-cyan-800 font-mono">
                    {img}
                  </span>
                ))}
              </div>
            </div>

            <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
              <div className="font-semibold text-slate-200 mb-1">Security Score</div>
              <div className="text-lg font-bold text-emerald-400">
                {previewData.security?.score} / 100 ({previewData.security?.status})
              </div>
            </div>

            <div className="flex justify-end gap-3 pt-2">
              <button
                onClick={() => setIsPreviewOpen(false)}
                className="px-4 py-2 bg-slate-800 text-slate-300 rounded-lg"
              >
                Cancel
              </button>
              <button
                onClick={() => {
                  setIsPreviewOpen(false)
                  handleStackAction('up')
                }}
                className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white font-medium rounded-lg shadow-sm"
              >
                Confirm Deploy (Up)
              </button>
            </div>
          </div>
        </Modal>
      )}

      {/* Critical Modal: Delete Stack */}
      {activeStack && (
        <CriticalConfirmModal
          isOpen={isDeleteOpen}
          onClose={() => setIsDeleteOpen(false)}
          onConfirm={handleDeleteStack}
          title={`Delete Stack ${activeStack.name}`}
          message={`This will permanently remove the Compose stack '${activeStack.name}', its configuration files, and execute 'docker compose down -v'. This cannot be undone.`}
          confirmWord={activeStack.name}
        />
      )}
    </div>
  )
}
