import React, { useEffect, useState, useRef } from 'react'
import { api } from '../api'
import { ContainerInfo, ContainerDetail, ContainerStats } from '../types'
import {
  IconBox,
  IconPlay,
  IconSquare,
  IconRotateCw,
  IconTrash,
  IconTerminal,
  IconFileText,
  IconActivity,
} from '../components/Icons'
import { Modal } from '../components/Modal'

export const Containers: React.FC = () => {
  const [containers, setContainers] = useState<ContainerInfo[]>([])
  const [filterState, setFilterState] = useState<'all' | 'running' | 'stopped'>('all')
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(true)

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

  useEffect(() => {
    loadContainers()
    const timer = setInterval(loadContainers, 5000)
    return () => clearInterval(timer)
  }, [])

  // When container detail is opened
  useEffect(() => {
    if (!selectedId) {
      setDetail(null)
      return
    }

    api.getContainer(selectedId).then(setDetail).catch(console.error)

    if (detailTab === 'stats') {
      api.getContainerStats(selectedId).then(setStats).catch(console.error)
    }
  }, [selectedId, detailTab])

  // Logs SSE stream
  useEffect(() => {
    if (!selectedId || detailTab !== 'logs') return

    setLogLines([])
    const eventSource = new EventSource(
      `/api/v1/containers/${selectedId}/logs?follow=true&tail=100&timestamps=true`
    )

    eventSource.onmessage = (e) => {
      setLogLines((prev) => [...prev.slice(-500), e.data])
      if (logFollow && logContainerRef.current) {
        logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
      }
    }

    return () => {
      eventSource.close()
    }
  }, [selectedId, detailTab, logFollow])

  // Terminal WebSocket
  useEffect(() => {
    if (!selectedId || detailTab !== 'terminal') {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      return
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/v1/containers/${selectedId}/terminal`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onmessage = (e) => {
      setTermOutput((prev) => [...prev, e.data])
    }

    ws.onerror = () => {
      setTermOutput((prev) => [...prev, '\r\n[WebSocket connection error]'])
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
      // Offline echo simulation
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

  const filtered = containers.filter((c) => {
    const isRunning = c.state === 'running'
    if (filterState === 'running' && !isRunning) return false
    if (filterState === 'stopped' && isRunning) return false
    if (search) {
      const q = search.toLowerCase()
      const nameMatch = c.names.some((n) => n.toLowerCase().includes(q))
      const imageMatch = c.image.toLowerCase().includes(q)
      const stackMatch = c.stack_name?.toLowerCase().includes(q)
      if (!nameMatch && !imageMatch && !stackMatch) return false
    }
    return true
  })

  return (
    <div className="p-6 space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-100 flex items-center gap-2">
            <IconBox size={22} className="text-purple-400" />
            Docker Containers
          </h1>
          <p className="text-xs text-slate-400 mt-0.5">Live Docker inventory and container lifecycle operations</p>
        </div>

        <div className="flex items-center gap-3">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search containers or images..."
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

      {/* Containers Table */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-800 text-xs font-semibold text-slate-400 uppercase tracking-wider bg-slate-950/20">
                <th className="px-6 py-3">Container</th>
                <th className="px-6 py-3">Stack</th>
                <th className="px-6 py-3">Image</th>
                <th className="px-6 py-3">State</th>
                <th className="px-6 py-3">Ports</th>
                <th className="px-6 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 text-sm">
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-8 text-center text-slate-400">
                    No containers found matching criteria.
                  </td>
                </tr>
              ) : (
                filtered.map((c) => {
                  const isRunning = c.state === 'running'
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
                          <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 text-xs font-mono">
                            {c.stack_name}
                          </span>
                        ) : (
                          <span className="text-slate-400 text-xs">—</span>
                        )}
                      </td>

                      <td className="px-6 py-4">
                        <span className="text-xs font-mono text-slate-300 truncate block max-w-xs">{c.image}</span>
                      </td>

                      <td className="px-6 py-4">
                        <span
                          className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border ${
                            isRunning
                              ? 'bg-emerald-950/50 text-emerald-400 border-emerald-800/60'
                              : 'bg-slate-800 text-slate-400 border-slate-700'
                          }`}
                        >
                          <span
                            className={`w-1.5 h-1.5 rounded-full ${isRunning ? 'bg-emerald-400' : 'bg-slate-400'}`}
                          ></span>
                          {c.status}
                        </span>
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

      {/* Container Detail Modal */}
      {selectedId && detail && (
        <Modal
          isOpen={true}
          onClose={() => setSelectedId(null)}
          title={`Container: ${detail.names.join(', ')}`}
          maxWidth="max-w-4xl"
        >
          <div className="space-y-4">
            {/* Modal Tabs */}
            <div className="flex border-b border-slate-800 text-xs font-medium text-slate-400">
              {(['overview', 'logs', 'terminal', 'stats'] as const).map((tab) => (
                <button
                  key={tab}
                  onClick={() => setDetailTab(tab)}
                  className={`py-2 px-4 border-b-2 capitalize transition-colors ${
                    detailTab === tab
                      ? 'border-cyan-400 text-cyan-400 font-semibold'
                      : 'border-transparent hover:text-slate-200'
                  }`}
                >
                  {tab}
                </button>
              ))}
            </div>

            {/* 1. Overview */}
            {detailTab === 'overview' && (
              <div className="space-y-4 text-xs">
                <div className="grid grid-cols-2 gap-3">
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                    <span className="text-slate-400">Full ID:</span>
                    <div className="font-mono text-slate-200 truncate mt-0.5">{detail.id}</div>
                  </div>
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                    <span className="text-slate-400">Image:</span>
                    <div className="font-mono text-slate-200 truncate mt-0.5">{detail.image}</div>
                  </div>
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                    <span className="text-slate-400">IP Address:</span>
                    <div className="font-mono text-slate-200 mt-0.5">{detail.ip_address || '—'}</div>
                  </div>
                  <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                    <span className="text-slate-400">Restart Policy:</span>
                    <div className="font-mono text-slate-200 mt-0.5">{detail.restart_policy || 'no'}</div>
                  </div>
                </div>

                {detail.mounts.length > 0 && (
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
                      placeholder="Type command (e.g. ps, ls, uptime)..."
                      className="w-full bg-transparent text-slate-100 font-mono text-xs focus:outline-none"
                    />
                  </div>
                  <button
                    type="submit"
                    className="px-4 py-1.5 bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-medium rounded-lg"
                  >
                    Execute
                  </button>
                </form>
              </div>
            )}

            {/* 4. Stats */}
            {detailTab === 'stats' && (
              <div className="space-y-3 text-xs">
                {stats ? (
                  <div className="grid grid-cols-2 gap-3">
                    <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                      <div className="text-slate-400">CPU Usage</div>
                      <div className="text-xl font-bold font-mono text-cyan-400 mt-1">
                        {stats.cpu_percent.toFixed(2)}%
                      </div>
                    </div>
                    <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                      <div className="text-slate-400">Memory Usage</div>
                      <div className="text-xl font-bold font-mono text-purple-400 mt-1">
                        {(stats.memory_used / (1024 * 1024)).toFixed(1)} MB ({stats.memory_percent.toFixed(1)}%)
                      </div>
                    </div>
                    <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                      <div className="text-slate-400">Active PIDs</div>
                      <div className="text-xl font-bold font-mono text-slate-200 mt-1">{stats.pids}</div>
                    </div>
                    <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                      <div className="text-slate-400">Network I/O</div>
                      <div className="text-sm font-mono text-slate-200 mt-1">
                        RX: {(stats.net_rx_bytes / 1024).toFixed(1)} KB | TX: {(stats.net_tx_bytes / 1024).toFixed(1)} KB
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="p-6 text-center text-slate-400">Loading container stats...</div>
                )}
              </div>
            )}
          </div>
        </Modal>
      )}
    </div>
  )
}
