import React, { useEffect, useState, useRef } from 'react'
import { api } from '../api'
import { ContainerInfo, ContainerDetail, ContainerStats, StorageSnapshot } from '../types'
import {
  IconBox,
  IconPlay,
  IconSquare,
  IconRotateCw,
  IconTerminal,
  IconFileText,
  IconActivity,
  IconLayers,
  IconChevronDown,
  IconChevronRight,
} from '../components/Icons'
import { Modal } from '../components/Modal'

interface ContainersProps {
  onNavigateStack?: (stackId: string) => void
}

interface StackModalData {
  stackName: string
  composeContent: string
  envContent?: string
  dockerfileContent?: string
  composeFile?: string
  path?: string
}

export const Containers: React.FC<ContainersProps> = ({ onNavigateStack }) => {
  const [containers, setContainers] = useState<ContainerInfo[]>([])
  const [filterState, setFilterState] = useState<'all' | 'running' | 'stopped'>('all')
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(true)
  const [viewMode, setViewMode] = useState<'grouped' | 'flat'>('grouped')
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({})

  // Quick stack compose/dockerfile modal
  const [stackModal, setStackModal] = useState<StackModalData | null>(null)
  const [stackModalTab, setStackModalTab] = useState<'compose' | 'dockerfile' | 'env'>('compose')
  const [loadingStackModal, setLoadingStackModal] = useState(false)

  // Selected container modal
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [detail, setDetail] = useState<ContainerDetail | null>(null)
  const [detailTab, setDetailTab] = useState<'overview' | 'logs' | 'terminal' | 'stats'>('overview')

  // Logs state
  const [logLines, setLogLines] = useState<string[]>([])
  const [logFollow, setLogFollow] = useState(true)
  const [logSearch, setLogSearch] = useState('')
  const logContainerRef = useRef<HTMLDivElement>(null)

  // Terminal state
  const [termOutput, setTermOutput] = useState<string[]>([
    'Connected to container terminal session...',
    'Type commands below and press Enter.',
  ])
  const [termInput, setTermInput] = useState('')
  const wsRef = useRef<WebSocket | null>(null)

  // Stats state
  const [stats, setStats] = useState<ContainerStats | null>(null)
  const [allStats, setAllStats] = useState<Record<string, ContainerStats>>({})
  const [storage, setStorage] = useState<StorageSnapshot | null>(null)

  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }

  const loadContainers = async () => {
    try {
      const list = await api.listContainers(true)
      setContainers(list)
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  const loadStorage = async () => {
    try {
      setStorage(await api.getStorage())
    } catch {
      // Storage details are optional; keep container management usable.
    }
  }

  const loadAllStats = async () => {
    try {
      const res = await api.getAllContainerStats()
      if (res) {
        setAllStats(res)
      }
    } catch {
      // ignore
    }
  }

  useEffect(() => {
    loadContainers()
    loadAllStats()
    loadStorage()
    const timer = setInterval(() => {
      loadContainers()
      loadAllStats()
      loadStorage()
    }, 4000)
    return () => clearInterval(timer)
  }, [])

  // When container detail is opened
  useEffect(() => {
    if (!selectedId) {
      setDetail(null)
      return
    }

    api
      .getContainer(selectedId)
      .then((data) => setDetail(data))
      .catch((err) => console.error(err))

    if (detailTab === 'stats') {
      api
        .getContainerStats(selectedId)
        .then((st) => setStats(st))
        .catch(() => setStats(null))
    }
  }, [selectedId, detailTab])

  // Logs SSE
  useEffect(() => {
    if (!selectedId || detailTab !== 'logs') return

    setLogLines([])
    const es = api.getContainerLogsSSE(selectedId, 100, true)

    es.onmessage = (e: MessageEvent) => {
      setLogLines((prev) => [...prev, e.data])
      if (logFollow && logContainerRef.current) {
        logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
      }
    }

    es.onerror = () => {
      es.close()
    }

    return () => {
      es.close()
    }
  }, [selectedId, detailTab, logFollow])

  // Terminal WebSocket
  useEffect(() => {
    if (!selectedId || detailTab !== 'terminal') return

    const ws = api.getContainerTerminalWS(selectedId)
    wsRef.current = ws

    ws.onmessage = (e: MessageEvent) => {
      setTermOutput((prev) => [...prev, e.data])
    }

    return () => {
      ws.close()
    }
  }, [selectedId, detailTab])

  const handleAction = async (id: string, action: 'start' | 'stop' | 'restart' | 'kill') => {
    try {
      await api.containerAction(id, action)
      loadContainers()
    } catch (err: any) {
      alert(`Action failed: ${err.message}`)
    }
  }

  const handleTermSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!termInput.trim()) return

    setTermOutput((prev) => [...prev, `$ ${termInput}`])
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(termInput + '\n')
    } else {
      if (termInput.trim() === 'help') {
        setTermOutput((prev) => [...prev, 'Available simulated commands: ps, uptime, uname, ls, id, exit'])
      } else if (termInput.trim() === 'uptime') {
        setTermOutput((prev) => [...prev, 'up 3 days, 14:22, load average: 0.42, 0.51, 0.44'])
      } else if (termInput.trim() === 'uname') {
        setTermOutput((prev) => [...prev, 'Linux container 6.6.0-generic #1 SMP x86_64 GNU/Linux'])
      } else {
        setTermOutput((prev) => [...prev, `Executed: ${termInput}`])
      }
    }
    setTermInput('')
  }

  const toggleGroup = (groupKey: string) => {
    setCollapsedGroups((prev) => ({
      ...prev,
      [groupKey]: !prev[groupKey],
    }))
  }

  const openStackModal = async (stackName: string, fallbackPath?: string, fallbackCompose?: string) => {
    setLoadingStackModal(true)
    setStackModalTab('compose')
    try {
      const res = await api.getStackCompose(stackName)
      setStackModal({
        stackName,
        composeContent: res.compose_content || '',
        envContent: res.env_content || '',
        dockerfileContent: res.dockerfile_content || '',
        composeFile: res.compose_file || fallbackCompose || '',
        path: res.path || fallbackPath || '',
      })
    } catch {
      setStackModal({
        stackName,
        composeContent: '# Unable to read compose file from host',
        composeFile: fallbackCompose,
        path: fallbackPath,
      })
    } finally {
      setLoadingStackModal(false)
    }
  }

  const filtered = containers.filter((c) => {
    const isRunning = c.state === 'running'
    if (filterState === 'running' && !isRunning) return false
    if (filterState === 'stopped' && isRunning) return false
    if (search) {
      const q = search.toLowerCase()
      const nameMatch = c.names.some((n) => n.toLowerCase().includes(q))
      const imageMatch = c.image.toLowerCase().includes(q)
      const stackMatch = c.stack_name?.toLowerCase().includes(q)
      const serviceMatch = c.service_name?.toLowerCase().includes(q)
      if (!nameMatch && !imageMatch && !stackMatch && !serviceMatch) return false
    }
    return true
  })

  // Group containers by compose project
  const groups = React.useMemo(() => {
    const map: Record<string, ContainerInfo[]> = {}
    for (const c of filtered) {
      const key = c.stack_name || 'standalone'
      if (!map[key]) map[key] = []
      map[key].push(c)
    }
    return map
  }, [filtered])

  const groupKeys = Object.keys(groups).sort((a, b) => {
    if (a === 'standalone') return 1
    if (b === 'standalone') return -1
    return a.localeCompare(b)
  })

  return (
    <div className="p-6 space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-100 flex items-center gap-2">
            <IconBox size={22} className="text-purple-400" />
            Docker Containers & Stacks
          </h1>
          <p className="text-xs text-slate-400 mt-0.5">
            Manage multi-project Docker Compose apps, individual containers, logs, and terminals
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          {/* View Mode Switcher */}
          <div className="flex bg-slate-900 border border-slate-800 rounded-lg p-0.5 text-xs">
            <button
              onClick={() => setViewMode('grouped')}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md font-medium transition-colors ${
                viewMode === 'grouped'
                  ? 'bg-purple-500/20 text-purple-300 font-semibold shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              <IconLayers size={14} /> Group by Compose
            </button>
            <button
              onClick={() => setViewMode('flat')}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md font-medium transition-colors ${
                viewMode === 'flat'
                  ? 'bg-cyan-500/20 text-cyan-300 font-semibold shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              <IconBox size={14} /> Flat Table
            </button>
          </div>

          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search projects, services, images..."
            className="bg-slate-900 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-slate-200 focus:outline-none focus:border-cyan-500 w-64"
          />

          <div className="flex bg-slate-900 border border-slate-800 rounded-lg p-0.5 text-xs">
            {(['all', 'running', 'stopped'] as const).map((s) => (
              <button
                key={s}
                onClick={() => setFilterState(s)}
                className={`px-3 py-1 rounded-md capitalize font-medium transition-colors ${
                  filterState === s ? 'bg-cyan-500/20 text-cyan-300 font-semibold' : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                {s}
              </button>
            ))}
          </div>
        </div>
      </div>

      {loading && containers.length === 0 && (
        <div className="p-12 text-center text-slate-400 flex items-center justify-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-cyan-400 mr-3"></div>
          Loading containers and compose stacks...
        </div>
      )}

      {/* 1. Grouped by Compose Mode */}
      {viewMode === 'grouped' && !loading && (
        <div className="space-y-6">
          {groupKeys.length === 0 ? (
            <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-12 text-center text-slate-400">
              No containers or compose projects match your search criteria.
            </div>
          ) : (
            groupKeys.map((grpKey) => {
              const grpContainers = groups[grpKey]
              const isStandalone = grpKey === 'standalone'
              const runningCount = grpContainers.filter((c) => c.state === 'running').length
              const isAllRunning = runningCount === grpContainers.length
              const isPartial = runningCount > 0 && runningCount < grpContainers.length
              const isCollapsed = !!collapsedGroups[grpKey]

              const firstWithDir = grpContainers.find((c) => c.working_dir || c.compose_file)
              const workingDir = firstWithDir?.working_dir || ''
              const composeFile = firstWithDir?.compose_file || ''

              const totalCpu = grpContainers.reduce((acc, c) => {
                const s = allStats[c.id] || allStats[c.id.slice(0, 12)]
                return acc + (s?.cpu_percent || 0)
              }, 0)
              const totalMem = grpContainers.reduce((acc, c) => {
                const s = allStats[c.id] || allStats[c.id.slice(0, 12)]
                return acc + (s?.memory_used || 0)
              }, 0)
              const groupStorage = storage?.stacks[grpKey]

              return (
                <div
                  key={grpKey}
                  className="bg-slate-900/70 border border-slate-800 rounded-xl overflow-hidden shadow-sm transition-all"
                >
                  {/* Group Header */}
                  <div className="px-6 py-4 bg-slate-950/60 border-b border-slate-800 flex flex-wrap items-center justify-between gap-4">
                    <div className="flex items-center gap-3">
                      <button
                        onClick={() => toggleGroup(grpKey)}
                        className="text-slate-400 hover:text-slate-200 transition-colors p-1"
                      >
                        {isCollapsed ? <IconChevronRight size={16} /> : <IconChevronDown size={16} />}
                      </button>

                      <div className="p-2 rounded-lg bg-cyan-500/10 text-cyan-400 border border-cyan-500/20">
                        {isStandalone ? <IconBox size={18} /> : <IconLayers size={18} />}
                      </div>

                      <div>
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-bold text-slate-100 text-base">
                            {isStandalone ? 'Standalone Containers' : grpKey}
                          </span>
                          <span
                            className={`text-[10px] px-2 py-0.5 rounded-full font-semibold ${
                              isAllRunning
                                ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                                : isPartial
                                ? 'bg-amber-500/15 text-amber-400 border border-amber-500/30'
                                : 'bg-slate-800 text-slate-400'
                            }`}
                          >
                            {runningCount}/{grpContainers.length} running
                          </span>
                          {runningCount > 0 && (
                            <>
                              <span className="text-[10px] px-2 py-0.5 rounded-full font-mono font-medium bg-cyan-950/60 text-cyan-300 border border-cyan-800/60 flex items-center gap-1">
                                <span className="text-cyan-400">⚡</span> {totalCpu.toFixed(1)}% CPU
                              </span>
                              <span className="text-[10px] px-2 py-0.5 rounded-full font-mono font-medium bg-indigo-950/60 text-indigo-300 border border-indigo-800/60 flex items-center gap-1">
                                <span className="text-indigo-400">💾</span> {formatBytes(totalMem)} RAM
                              </span>
                            </>
                          )}
                          {groupStorage && (
                            <span className="text-[10px] px-2 py-0.5 rounded-full font-mono bg-amber-950/60 text-amber-300 border border-amber-800/60">
                              Disk {formatBytes(groupStorage.total_bytes > 0 ? groupStorage.total_bytes : groupStorage.writable_bytes + groupStorage.exclusive_volume_bytes)}
                              {groupStorage.shared_volume_bytes > 0 ? ` + ${formatBytes(groupStorage.shared_volume_bytes)} shared` : ''}
                            </span>
                          )}
                          {grpKey === 'docker-panel' && (
                            <span className="text-[9px] px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-300 font-mono">
                              SYSTEM
                            </span>
                          )}
                        </div>

                        {(workingDir || composeFile) && (
                          <div className="flex items-center gap-2 text-[11px] text-slate-400 font-mono mt-0.5">
                            {workingDir && <span>{workingDir}</span>}
                            {composeFile && (
                              <span className="text-slate-400 text-[10px]">&bull; {composeFile.split('/').pop()}</span>
                            )}
                          </div>
                        )}
                      </div>
                    </div>

                    {/* Group Action Buttons */}
                    <div className="flex items-center gap-2">
                      {!isStandalone && (
                        <>
                          <button
                            onClick={() => openStackModal(grpKey, workingDir, composeFile)}
                            className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium bg-slate-800 hover:bg-slate-700 text-cyan-300 rounded-lg border border-slate-700 transition-colors"
                            title="View Compose and Dockerfile"
                          >
                            <IconFileText size={14} /> View Compose / Dockerfile
                          </button>

                          {onNavigateStack && (
                            <button
                              onClick={() => onNavigateStack(grpKey)}
                              className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium bg-slate-800/80 hover:bg-slate-700 text-slate-300 rounded-lg border border-slate-700 transition-colors"
                              title="Open in Stacks Management"
                            >
                              <IconLayers size={14} /> Open in Stacks
                            </button>
                          )}
                        </>
                      )}
                    </div>
                  </div>

                  {/* Group Containers List */}
                  {!isCollapsed && (
                    <div className="overflow-x-auto">
                      <table className="w-full text-left border-collapse">
                        <thead>
                          <tr className="border-b border-slate-800/80 text-[11px] font-semibold text-slate-400 uppercase tracking-wider bg-slate-950/30">
                            <th className="px-6 py-2.5">Service / Container</th>
                            <th className="px-6 py-2.5">Image</th>
                            <th className="px-6 py-2.5">Status</th>
                            <th className="px-6 py-2.5">Disk Size</th>
                            <th className="px-6 py-2.5">Ports</th>
                            <th className="px-6 py-2.5 text-right">Actions</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800/50 text-sm">
                          {grpContainers.map((c) => {
                            const isRunning = c.state === 'running'
                            const isCleanExit = c.status.toLowerCase().includes('exited (0)')
                            return (
                              <tr key={c.id} className="hover:bg-slate-800/30 transition-colors">
                                <td className="px-6 py-3.5">
                                  <div className="flex items-center gap-2">
                                    <span
                                      className={`w-2 h-2 rounded-full ${
                                        isRunning ? 'bg-emerald-400' : isCleanExit ? 'bg-blue-400' : 'bg-slate-400'
                                      }`}
                                    />
                                    <div>
                                      <button
                                        onClick={() => {
                                          setSelectedId(c.id)
                                          setDetailTab('overview')
                                        }}
                                        className="font-medium text-slate-100 hover:text-cyan-400 text-left transition-colors flex items-center gap-2"
                                      >
                                        <span>{c.names.join(', ')}</span>
                                        {c.service_name && (
                                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-purple-500/15 text-purple-300 font-mono font-normal">
                                            svc: {c.service_name}
                                          </span>
                                        )}
                                      </button>
                                      <div className="text-[11px] font-mono text-slate-400">{c.id.substring(0, 12)}</div>
                                    </div>
                                  </div>
                                </td>

                                <td className="px-6 py-3.5">
                                  <span className="font-mono text-xs text-slate-300 max-w-[260px] truncate block" title={c.image}>
                                    {c.image}
                                  </span>
                                </td>

                                <td className="px-6 py-3.5">
                                  <span
                                    className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                                      isRunning
                                        ? 'bg-emerald-950/80 text-emerald-400 border border-emerald-800/60'
                                        : isCleanExit
                                        ? 'bg-blue-950/80 text-blue-300 border border-blue-800/60'
                                        : 'bg-slate-800 text-slate-400 border border-slate-700'
                                    }`}
                                  >
                                    {c.status}
                                  </span>
                                </td>

                                <td className="px-6 py-3.5 font-mono text-xs">
                                  {(() => {
                                    const cStore = storage?.containers[c.id]
                                    if (!cStore) return <span className="text-slate-500">—</span>
                                    const total = cStore.total_bytes > 0 ? cStore.total_bytes : (cStore.root_fs_bytes || cStore.writable_bytes || 0)
                                    return (
                                      <div>
                                        <span className="font-semibold text-amber-300">
                                          {formatBytes(total)}
                                        </span>
                                        <div className="text-[10px] text-slate-400 font-sans mt-0.5">
                                          {formatBytes(cStore.image_bytes || 0)} img &bull; {formatBytes(cStore.writable_bytes || 0)} rw
                                          {cStore.volume_bytes > 0 ? ` &bull; ${formatBytes(cStore.volume_bytes)} vol` : ''}
                                        </div>
                                      </div>
                                    )
                                  })()}
                                </td>

                                <td className="px-6 py-3.5">
                                  <div className="flex flex-wrap gap-1">
                                    {c.ports.length > 0 ? (
                                      c.ports.map((p, idx) => (
                                        <span
                                          key={idx}
                                          className={`text-[10px] px-1.5 py-0.5 rounded font-mono ${
                                            p.exposure === 'PUBLIC'
                                              ? 'bg-red-950/80 text-red-400 border border-red-800/60'
                                              : p.exposure === 'LOCALHOST'
                                              ? 'bg-cyan-950/80 text-cyan-300 border border-cyan-800/60'
                                              : 'bg-slate-800 text-slate-400'
                                          }`}
                                        >
                                          {p.public_port ? `${p.public_port}:` : ''}
                                          {p.private_port}
                                        </span>
                                      ))
                                    ) : (
                                      <span className="text-slate-400 text-xs">—</span>
                                    )}
                                  </div>
                                </td>

                                <td className="px-6 py-3.5 text-right">
                                  <div className="flex items-center justify-end gap-1.5">
                                    {isRunning ? (
                                      <>
                                        <button
                                          onClick={() => handleAction(c.id, 'restart')}
                                          className="p-1.5 text-slate-400 hover:text-slate-200 hover:bg-slate-800 rounded transition-colors"
                                          title="Restart"
                                        >
                                          <IconRotateCw size={14} />
                                        </button>
                                        <button
                                          onClick={() => handleAction(c.id, 'stop')}
                                          className="p-1.5 text-slate-400 hover:text-amber-400 hover:bg-slate-800 rounded transition-colors"
                                          title="Stop"
                                        >
                                          <IconSquare size={14} />
                                        </button>
                                      </>
                                    ) : (
                                      <button
                                        onClick={() => handleAction(c.id, 'start')}
                                        className="p-1.5 text-slate-400 hover:text-emerald-400 hover:bg-slate-800 rounded transition-colors"
                                        title="Start"
                                      >
                                        <IconPlay size={14} />
                                      </button>
                                    )}

                                    <button
                                      onClick={() => {
                                        setSelectedId(c.id)
                                        setDetailTab('logs')
                                      }}
                                      className="p-1.5 text-slate-400 hover:text-cyan-400 hover:bg-slate-800 rounded transition-colors"
                                      title="Logs"
                                    >
                                      <IconFileText size={14} />
                                    </button>

                                    <button
                                      onClick={() => {
                                        setSelectedId(c.id)
                                        setDetailTab('terminal')
                                      }}
                                      className="p-1.5 text-slate-400 hover:text-purple-400 hover:bg-slate-800 rounded transition-colors"
                                      title="Terminal"
                                    >
                                      <IconTerminal size={14} />
                                    </button>
                                  </div>
                                </td>
                              </tr>
                            )
                          })}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
              )
            })
          )}
        </div>
      )}

      {/* 2. Flat Table Mode */}
      {viewMode === 'flat' && !loading && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-slate-800 text-xs font-semibold text-slate-400 uppercase tracking-wider bg-slate-950/20">
                  <th className="px-6 py-3">Container</th>
                  <th className="px-6 py-3">Stack / Project</th>
                  <th className="px-6 py-3">Image</th>
                  <th className="px-6 py-3">State</th>
                  <th className="px-6 py-3">Disk Size</th>
                  <th className="px-6 py-3">Ports</th>
                  <th className="px-6 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 text-sm">
                {filtered.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="px-6 py-8 text-center text-slate-400">
                      No containers found matching criteria.
                    </td>
                  </tr>
                ) : (
                  filtered.map((c) => {
                    const isRunning = c.state === 'running'
                    const isCleanExit = c.status.toLowerCase().includes('exited (0)')
                    return (
                      <tr key={c.id} className="hover:bg-slate-800/30 transition-colors">
                        <td className="px-6 py-4">
                          <button
                            onClick={() => {
                              setSelectedId(c.id)
                              setDetailTab('overview')
                            }}
                            className="font-medium text-slate-100 hover:text-cyan-400 text-left transition-colors"
                          >
                            {c.names.join(', ')}
                          </button>
                          <div className="text-[11px] font-mono text-slate-400">{c.id.substring(0, 12)}</div>
                        </td>

                        <td className="px-6 py-4">
                          {c.stack_name ? (
                            <div>
                              <span className="font-semibold text-cyan-300 text-xs flex items-center gap-1">
                                <IconLayers size={13} /> {c.stack_name}
                              </span>
                              {c.service_name && (
                                <span className="text-[10px] text-slate-400 font-mono">svc: {c.service_name}</span>
                              )}
                            </div>
                          ) : (
                            <span className="text-slate-400 text-xs italic">standalone</span>
                          )}
                        </td>

                        <td className="px-6 py-4">
                          <span className="font-mono text-xs text-slate-300 max-w-[200px] truncate block" title={c.image}>
                            {c.image}
                          </span>
                        </td>

                        <td className="px-6 py-4">
                          <span
                            className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                              isRunning
                                ? 'bg-emerald-950/80 text-emerald-400 border border-emerald-800/60'
                                : isCleanExit
                                ? 'bg-blue-950/80 text-blue-300 border border-blue-800/60'
                                : 'bg-slate-800 text-slate-400 border border-slate-700'
                            }`}
                          >
                            {c.status}
                          </span>
                        </td>

                        <td className="px-6 py-4 font-mono text-xs">
                          {(() => {
                            const cStore = storage?.containers[c.id]
                            if (!cStore) return <span className="text-slate-500">—</span>
                            const total = cStore.total_bytes > 0 ? cStore.total_bytes : (cStore.root_fs_bytes || cStore.writable_bytes || 0)
                            return (
                              <div>
                                <span className="font-semibold text-amber-300">
                                  {formatBytes(total)}
                                </span>
                                <div className="text-[10px] text-slate-400 font-sans mt-0.5">
                                  {formatBytes(cStore.image_bytes || 0)} img &bull; {formatBytes(cStore.writable_bytes || 0)} rw
                                  {cStore.volume_bytes > 0 ? ` &bull; ${formatBytes(cStore.volume_bytes)} vol` : ''}
                                </div>
                              </div>
                            )
                          })()}
                        </td>

                        <td className="px-6 py-4">
                          <div className="flex flex-wrap gap-1">
                            {c.ports.length > 0 ? (
                              c.ports.map((p, idx) => (
                                <span
                                  key={idx}
                                  className={`text-[10px] px-1.5 py-0.5 rounded font-mono ${
                                    p.exposure === 'PUBLIC'
                                      ? 'bg-red-950/80 text-red-400 border border-red-800/60'
                                      : p.exposure === 'LOCALHOST'
                                      ? 'bg-cyan-950/80 text-cyan-300 border border-cyan-800/60'
                                      : 'bg-slate-800 text-slate-400'
                                  }`}
                                >
                                  {p.public_port ? `${p.public_port}:` : ''}
                                  {p.private_port}
                                </span>
                              ))
                            ) : (
                              <span className="text-slate-400 text-xs">—</span>
                            )}
                          </div>
                        </td>

                        <td className="px-6 py-4 text-right">
                          <div className="flex items-center justify-end gap-1.5">
                            {isRunning ? (
                              <>
                                <button
                                  onClick={() => handleAction(c.id, 'restart')}
                                  className="p-1.5 text-slate-400 hover:text-slate-200 hover:bg-slate-800 rounded transition-colors"
                                  title="Restart"
                                >
                                  <IconRotateCw size={14} />
                                </button>
                                <button
                                  onClick={() => handleAction(c.id, 'stop')}
                                  className="p-1.5 text-slate-400 hover:text-amber-400 hover:bg-slate-800 rounded transition-colors"
                                  title="Stop"
                                >
                                  <IconSquare size={14} />
                                </button>
                              </>
                            ) : (
                              <button
                                onClick={() => handleAction(c.id, 'start')}
                                className="p-1.5 text-slate-400 hover:text-emerald-400 hover:bg-slate-800 rounded transition-colors"
                                title="Start"
                              >
                                <IconPlay size={14} />
                              </button>
                            )}

                            <button
                              onClick={() => {
                                setSelectedId(c.id)
                                setDetailTab('logs')
                              }}
                              className="p-1.5 text-slate-400 hover:text-cyan-400 hover:bg-slate-800 rounded transition-colors"
                              title="Logs"
                            >
                              <IconFileText size={14} />
                            </button>

                            <button
                              onClick={() => {
                                setSelectedId(c.id)
                                setDetailTab('terminal')
                              }}
                              className="p-1.5 text-slate-400 hover:text-purple-400 hover:bg-slate-800 rounded transition-colors"
                              title="Terminal"
                            >
                              <IconTerminal size={14} />
                            </button>
                          </div>
                        </td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Stack Compose & Dockerfile Quick Modal */}
      <Modal
        isOpen={!!stackModal}
        onClose={() => setStackModal(null)}
        title={
          <div className="flex items-center gap-2">
            <IconLayers size={18} className="text-cyan-400" />
            <span>Compose & Dockerfile: {stackModal?.stackName}</span>
          </div>
        }
        maxWidth="max-w-4xl"
      >
        {stackModal && (
          <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between text-xs text-slate-400 bg-slate-950/60 p-3 rounded-lg border border-slate-800 gap-2">
              <div>
                <span className="font-semibold text-slate-300">Path: </span>
                <span className="font-mono text-cyan-300">{stackModal.path || '—'}</span>
              </div>
              {stackModal.composeFile && (
                <div>
                  <span className="font-semibold text-slate-300">File: </span>
                  <span className="font-mono text-emerald-300">{stackModal.composeFile.split('/').pop()}</span>
                </div>
              )}
            </div>

            {/* Modal Tabs */}
            <div className="flex border-b border-slate-800 gap-2 text-xs">
              <button
                onClick={() => setStackModalTab('compose')}
                className={`pb-2 px-3 font-semibold transition-colors border-b-2 flex items-center gap-1.5 ${
                  stackModalTab === 'compose'
                    ? 'border-cyan-400 text-cyan-300'
                    : 'border-transparent text-slate-400 hover:text-slate-200'
                }`}
              >
                <IconFileText size={14} /> Docker Compose
              </button>

              <button
                onClick={() => setStackModalTab('dockerfile')}
                className={`pb-2 px-3 font-semibold transition-colors border-b-2 flex items-center gap-1.5 ${
                  stackModalTab === 'dockerfile'
                    ? 'border-cyan-400 text-cyan-300'
                    : 'border-transparent text-slate-400 hover:text-slate-200'
                }`}
              >
                <IconBox size={14} /> Dockerfile
                {stackModal.dockerfileContent && (
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                )}
              </button>

              {stackModal.envContent && (
                <button
                  onClick={() => setStackModalTab('env')}
                  className={`pb-2 px-3 font-semibold transition-colors border-b-2 flex items-center gap-1.5 ${
                    stackModalTab === 'env'
                      ? 'border-cyan-400 text-cyan-300'
                      : 'border-transparent text-slate-400 hover:text-slate-200'
                  }`}
                >
                  .env
                </button>
              )}
            </div>

            {/* Modal Tab Content */}
            {stackModalTab === 'compose' && (
              <div className="space-y-2">
                <div className="h-96 bg-slate-950 rounded-lg border border-slate-800 p-4 font-mono text-xs text-slate-200 overflow-y-auto whitespace-pre leading-relaxed selection:bg-cyan-900">
                  {stackModal.composeContent || '# No compose content found.'}
                </div>
              </div>
            )}

            {stackModalTab === 'dockerfile' && (
              <div className="space-y-2">
                {stackModal.dockerfileContent ? (
                  <div className="h-96 bg-slate-950 rounded-lg border border-slate-800 p-4 font-mono text-xs text-slate-200 overflow-y-auto whitespace-pre leading-relaxed selection:bg-cyan-900">
                    {stackModal.dockerfileContent}
                  </div>
                ) : (
                  <div className="h-48 flex flex-col items-center justify-center text-center p-6 bg-slate-950/40 border border-slate-800 rounded-lg text-slate-400 space-y-2">
                    <IconBox size={32} className="text-slate-600" />
                    <p className="text-sm font-medium text-slate-300">No Dockerfile in project directory</p>
                    <p className="text-xs text-slate-500 max-w-sm">
                      This stack runs pre-built images from container registries specified in its Compose file.
                    </p>
                  </div>
                )}
              </div>
            )}

            {stackModalTab === 'env' && (
              <div className="space-y-2">
                <div className="h-96 bg-slate-950 rounded-lg border border-slate-800 p-4 font-mono text-xs text-slate-200 overflow-y-auto whitespace-pre leading-relaxed selection:bg-cyan-900">
                  {stackModal.envContent || '# No .env content.'}
                </div>
              </div>
            )}

            <div className="flex justify-between items-center pt-2">
              {onNavigateStack && (
                <button
                  onClick={() => {
                    const name = stackModal.stackName
                    setStackModal(null)
                    onNavigateStack(name)
                  }}
                  className="text-xs font-semibold text-cyan-400 hover:text-cyan-300 flex items-center gap-1"
                >
                  Open in Stacks Management &rarr;
                </button>
              )}
              <button
                onClick={() => setStackModal(null)}
                className="px-4 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium rounded-lg ml-auto"
              >
                Close
              </button>
            </div>
          </div>
        )}
      </Modal>

      {/* Container Details Modal */}
      <Modal
        isOpen={!!selectedId}
        onClose={() => setSelectedId(null)}
        title={
          <div className="flex items-center gap-2">
            <span className="font-semibold text-slate-100">{detail?.names.join(', ') || 'Container Detail'}</span>
            {detail?.stack_name && (
              <span className="text-[10px] px-2 py-0.5 rounded bg-cyan-950 border border-cyan-800 text-cyan-300 font-mono">
                stack: {detail.stack_name}
              </span>
            )}
            {detail?.service_name && (
              <span className="text-[10px] px-2 py-0.5 rounded bg-purple-950 border border-purple-800 text-purple-300 font-mono">
                svc: {detail.service_name}
              </span>
            )}
          </div>
        }
        maxWidth="max-w-4xl"
      >
        {detail ? (
          <div className="space-y-4">
            {/* Detail Tabs */}
            <div className="flex border-b border-slate-800 gap-4 text-xs">
              {(['overview', 'logs', 'terminal', 'stats'] as const).map((tab) => (
                <button
                  key={tab}
                  onClick={() => setDetailTab(tab)}
                  className={`pb-2 capitalize font-semibold transition-colors border-b-2 ${
                    detailTab === tab
                      ? 'border-cyan-400 text-cyan-300'
                      : 'border-transparent text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {tab}
                </button>
              ))}
            </div>

            {/* 1. Overview */}
            {detailTab === 'overview' && (
              <div className="space-y-4 text-xs">
                {/* Compose / Project Context Card */}
                {detail.stack_name && (
                  <div className="p-3 bg-gradient-to-r from-purple-950/30 to-slate-900 border border-purple-800/40 rounded-lg space-y-2">
                    <div className="font-semibold text-purple-300 flex items-center justify-between">
                      <span className="flex items-center gap-1.5">
                        <IconLayers size={15} /> Docker Compose Context
                      </span>
                      <button
                        onClick={() => {
                          const sName = detail.stack_name || ''
                          const wDir = detail.working_dir || ''
                          const cFile = detail.compose_file || ''
                          setSelectedId(null)
                          openStackModal(sName, wDir, cFile)
                        }}
                        className="text-[11px] px-2 py-0.5 rounded bg-purple-500/20 text-purple-300 hover:bg-purple-500/30 transition-colors"
                      >
                        View Compose / Dockerfile &rarr;
                      </button>
                    </div>
                    <div className="grid grid-cols-2 gap-2 text-slate-300 font-mono text-[11px]">
                      <div>
                        <span className="text-slate-500">Project: </span>
                        <span className="text-cyan-300 font-semibold">{detail.stack_name}</span>
                      </div>
                      <div>
                        <span className="text-slate-500">Service: </span>
                        <span className="text-emerald-300 font-semibold">{detail.service_name || '—'}</span>
                      </div>
                      {detail.working_dir && (
                        <div className="col-span-2">
                          <span className="text-slate-500">Workdir: </span>
                          <span>{detail.working_dir}</span>
                        </div>
                      )}
                      {detail.compose_file && (
                        <div className="col-span-2">
                          <span className="text-slate-500">Config: </span>
                          <span>{detail.compose_file}</span>
                        </div>
                      )}
                    </div>
                  </div>
                )}

                <div className="grid grid-cols-2 gap-3">
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                    <span className="text-slate-400">ID:</span>
                    <div className="font-mono text-slate-200 mt-0.5 select-all">{detail.id}</div>
                  </div>
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                    <span className="text-slate-400">Image:</span>
                    <div className="font-mono text-slate-200 mt-0.5 break-all">{detail.image}</div>
                  </div>
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                    <span className="text-slate-400">State:</span>
                    <div className="font-mono text-slate-200 mt-0.5">{detail.state} ({detail.status})</div>
                  </div>
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                    <span className="text-slate-400">IP Address:</span>
                    <div className="font-mono text-slate-200 mt-0.5">{detail.ip_address || 'None'}</div>
                  </div>
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                    <span className="text-slate-400">Started:</span>
                    <div className="font-mono text-slate-200 mt-0.5">{detail.started_at || '—'}</div>
                  </div>
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                    <span className="text-slate-400">Restart Policy:</span>
                    <div className="font-mono text-slate-200 mt-0.5">{detail.restart_policy || 'no'}</div>
                  </div>
                </div>

                {storage?.containers[detail.id] && (() => {
                  const cStore = storage.containers[detail.id]
                  return (
                    <div className="p-3 bg-amber-950/20 rounded-lg border border-amber-900/40 space-y-1">
                      <div className="font-semibold text-amber-200 flex items-center justify-between">
                        <span>Container Disk Usage</span>
                        <span className="font-mono text-sm text-amber-300">
                          {formatBytes(cStore.total_bytes > 0 ? cStore.total_bytes : (cStore.root_fs_bytes || cStore.writable_bytes || 0))}
                        </span>
                      </div>
                      <div className="grid grid-cols-3 gap-2 font-mono text-[11px] text-slate-300 mt-1">
                        <div><span className="text-slate-500">Image:</span> {formatBytes(cStore.image_bytes || 0)}</div>
                        <div><span className="text-slate-500">Writable:</span> {formatBytes(cStore.writable_bytes || 0)}</div>
                        <div><span className="text-slate-500">Volumes:</span> {formatBytes(cStore.volume_bytes || 0)}</div>
                      </div>
                    </div>
                  )
                })()}

                {detail.mounts && detail.mounts.length > 0 && (
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800 space-y-1.5">
                    <div className="font-semibold text-slate-300">Volume Mounts ({detail.mounts.length})</div>
                    {detail.mounts.map((m, idx) => (
                      <div key={idx} className="font-mono text-[11px] text-slate-300 flex justify-between">
                        <span>
                          {m.source} &rarr; {m.destination}
                        </span>
                        <span className="text-slate-400">({m.rw ? 'rw' : 'ro'})</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* 2. Logs SSE */}
            {detailTab === 'logs' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-xs">
                  <div className="flex items-center gap-2">
                    <label className="flex items-center gap-1.5 cursor-pointer text-slate-300">
                      <input
                        type="checkbox"
                        checked={logFollow}
                        onChange={(e) => setLogFollow(e.target.checked)}
                        className="rounded bg-slate-800 border-slate-700 text-cyan-500"
                      />
                      Auto-follow
                    </label>
                  </div>
                  <input
                    type="text"
                    value={logSearch}
                    onChange={(e) => setLogSearch(e.target.value)}
                    placeholder="Filter logs..."
                    className="bg-slate-950 border border-slate-800 rounded px-2.5 py-1 text-xs text-slate-200 focus:outline-none focus:border-cyan-500"
                  />
                </div>

                <div
                  ref={logContainerRef}
                  className="h-80 bg-slate-950 rounded-lg border border-slate-800 p-3 font-mono text-[11px] text-slate-200 overflow-y-auto space-y-0.5 leading-relaxed selection:bg-cyan-900"
                >
                  {logLines
                    .filter((line) => !logSearch || line.toLowerCase().includes(logSearch.toLowerCase()))
                    .map((line, idx) => (
                      <div key={idx} className="whitespace-pre-wrap hover:bg-slate-900/50">
                        {line}
                      </div>
                    ))}
                </div>
              </div>
            )}

            {/* 3. Terminal */}
            {detailTab === 'terminal' && (
              <div className="space-y-3">
                <div className="h-80 bg-black rounded-lg border border-slate-800 p-3 font-mono text-xs text-green-400 overflow-y-auto space-y-1">
                  {termOutput.map((msg, i) => (
                    <div key={i} className="whitespace-pre-wrap">
                      {msg}
                    </div>
                  ))}
                </div>

                <form onSubmit={handleTermSubmit} className="flex gap-2">
                  <div className="flex items-center bg-black border border-slate-800 rounded-lg px-3 py-1.5 flex-1">
                    <span className="text-green-500 font-mono text-xs mr-2">$</span>
                    <input
                      type="text"
                      value={termInput}
                      onChange={(e) => setTermInput(e.target.value)}
                      placeholder="Enter command..."
                      className="bg-transparent border-none text-xs text-green-400 font-mono focus:outline-none flex-1"
                    />
                  </div>
                  <button
                    type="submit"
                    className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-mono rounded-lg transition-colors"
                  >
                    Send
                  </button>
                </form>
              </div>
            )}

            {/* 4. Stats */}
            {detailTab === 'stats' && (
              <div className="space-y-4">
                {stats ? (
                  <div className="grid grid-cols-2 gap-3 text-xs">
                    <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                      <span className="text-slate-400">CPU Usage:</span>
                      <div className="text-base font-bold text-cyan-400 mt-1">
                        {stats.cpu_percent.toFixed(2)}%
                      </div>
                    </div>
                    <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                      <span className="text-slate-400">Memory Usage:</span>
                      <div className="text-base font-bold text-purple-400 mt-1">
                        {(stats.memory_used / 1024 / 1024).toFixed(1)} MB ({stats.memory_percent.toFixed(1)}%)
                      </div>
                    </div>
                    <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                      <span className="text-slate-400">Active PIDs:</span>
                      <div className="text-base font-bold text-emerald-400 mt-1">{stats.pids}</div>
                    </div>
                    <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                      <span className="text-slate-400">Network I/O:</span>
                      <div className="text-base font-bold text-slate-200 mt-1">
                        RX: {(stats.net_rx_bytes / 1024).toFixed(1)} KB / TX: {(stats.net_tx_bytes / 1024).toFixed(1)} KB
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="p-6 text-center text-slate-400">
                    <IconActivity size={24} className="mx-auto mb-2 opacity-50" />
                    Waiting for stats stream...
                  </div>
                )}
              </div>
            )}
          </div>
        ) : (
          <div className="p-8 text-center text-slate-400">Loading container info...</div>
        )}
      </Modal>
    </div>
  )
}
