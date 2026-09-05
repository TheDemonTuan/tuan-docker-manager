import React, { useEffect, useState } from 'react'
import { api } from '../api'
import { DashboardData } from '../types'
import { IconCpu, IconHardDrive, IconBox, IconLayers, IconShield, IconActivity, IconZap } from '../components/Icons'

interface DashboardProps {
  onNavigateStack: (stackId: string) => void
}

export const Dashboard: React.FC<DashboardProps> = ({ onNavigateStack }) => {
  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [allStats, setAllStats] = useState<Record<string, any>>({})

  const loadDashboard = async () => {
    try {
      const res = await api.getDashboard()
      setData(res)
      setError(null)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
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

  useEffect(() => {
    loadDashboard()
    loadAllStats()
    const timer = setInterval(() => {
      loadDashboard()
      loadAllStats()
    }, 4000)
    return () => clearInterval(timer)
  }, [])

  if (loading && !data) {
    return (
      <div className="p-8 text-center text-slate-400 flex items-center justify-center h-full">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-cyan-400 mr-3"></div>
        Loading dashboard metrics...
      </div>
    )
  }

  if (error && !data) {
    return (
      <div className="p-8">
        <div className="p-4 bg-red-950/50 border border-red-800 rounded-lg text-red-200">
          Failed to load dashboard: {error}
        </div>
      </div>
    )
  }

  const host = data?.host
  const gpu = data?.gpu
  const summary = data?.summary
  const stacks = data?.stacks || []

  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }

  const formatUptime = (seconds: number) => {
    if (!seconds) return '0m'
    const days = Math.floor(seconds / 86400)
    const hours = Math.floor((seconds % 86400) / 3600)
    const mins = Math.floor((seconds % 3600) / 60)
    if (days > 0) return `${days}d ${hours}h`
    if (hours > 0) return `${hours}h ${mins}m`
    return `${mins}m`
  }

  return (
    <div className="p-6 space-y-6 max-w-7xl mx-auto">
      {/* VPS Status Banner */}
      <div className="flex flex-wrap items-center justify-between p-4 bg-slate-900/90 border border-slate-800 rounded-xl shadow-sm">
        <div className="flex items-center gap-3">
          <div className="w-3 h-3 rounded-full bg-emerald-400 shadow-lg shadow-emerald-500/50"></div>
          <span className="font-semibold text-slate-100 text-lg">VPS-01</span>
          <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-950/60 text-emerald-400 border border-emerald-800/60 font-medium">
            Healthy
          </span>
        </div>
        <div className="flex items-center gap-6 text-xs text-slate-300">
          <div>
            <span className="text-slate-400">Load: </span>
            <span className="font-mono text-slate-200 font-medium">
              {host?.load_1?.toFixed(2) || '0.00'}, {host?.load_5?.toFixed(2) || '0.00'}, {host?.load_15?.toFixed(2) || '0.00'}
            </span>
          </div>
          <div>
            <span className="text-slate-400">Uptime: </span>
            <span className="font-mono text-slate-200">{formatUptime(host?.uptime_seconds || 0)}</span>
          </div>
        </div>
      </div>

      {/* Host Metrics Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* CPU */}
        <div className="p-5 bg-slate-900/60 border border-slate-800 rounded-xl space-y-3">
          <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
            <span className="flex items-center gap-1.5">
              <IconCpu size={16} className="text-cyan-400" /> CPU Usage
            </span>
            <span className="font-mono text-slate-200 font-semibold">{host?.cpu_percent?.toFixed(1)}%</span>
          </div>
          <div className="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
            <div
              className="bg-cyan-400 h-full rounded-full transition-all duration-500"
              style={{ width: `${Math.min(host?.cpu_percent || 0, 100)}%` }}
            ></div>
          </div>
          <div className="text-[11px] text-slate-400">All cores utilization</div>
        </div>

        {/* RAM */}
        <div className="p-5 bg-slate-900/60 border border-slate-800 rounded-xl space-y-3">
          <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
            <span className="flex items-center gap-1.5">
              <IconActivity size={16} className="text-purple-400" /> Memory (RAM)
            </span>
            <span className="font-mono text-slate-200 font-semibold">{host?.memory_percent?.toFixed(1)}%</span>
          </div>
          <div className="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
            <div
              className="bg-purple-400 h-full rounded-full transition-all duration-500"
              style={{ width: `${Math.min(host?.memory_percent || 0, 100)}%` }}
            ></div>
          </div>
          <div className="text-[11px] text-slate-400">
            {formatBytes(host?.memory_used || 0)} / {formatBytes(host?.memory_total || 0)}
          </div>
        </div>

        {/* Disk */}
        <div className="p-5 bg-slate-900/60 border border-slate-800 rounded-xl space-y-3">
          <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
            <span className="flex items-center gap-1.5">
              <IconHardDrive size={16} className="text-amber-400" /> Disk Storage
            </span>
            <span className="font-mono text-slate-200 font-semibold">{host?.disk_percent?.toFixed(1)}%</span>
          </div>
          <div className="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
            <div
              className="bg-amber-400 h-full rounded-full transition-all duration-500"
              style={{ width: `${Math.min(host?.disk_percent || 0, 100)}%` }}
            ></div>
          </div>
          <div className="text-[11px] text-slate-400">
            {formatBytes(host?.disk_used || 0)} / {formatBytes(host?.disk_total || 0)}
          </div>
        </div>

        {/* Network Rate */}
        <div className="p-5 bg-slate-900/60 border border-slate-800 rounded-xl space-y-3">
          <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
            <span className="flex items-center gap-1.5">
              <IconZap size={16} className="text-emerald-400" /> Network Rate
            </span>
            <span className="text-slate-300 font-mono text-xs">Live</span>
          </div>
          <div className="grid grid-cols-2 gap-2 pt-1">
            <div>
              <div className="text-[10px] text-slate-400 uppercase">RX In</div>
              <div className="font-mono text-sm font-semibold text-slate-200">
                {formatBytes(host?.net_rx_bytes_rate || 0)}/s
              </div>
            </div>
            <div>
              <div className="text-[10px] text-slate-400 uppercase">TX Out</div>
              <div className="font-mono text-sm font-semibold text-slate-200">
                {formatBytes(host?.net_tx_bytes_rate || 0)}/s
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* GPU Telemetry if Available */}
      {gpu?.available && gpu.gpus.length > 0 && (
        <div className="p-5 bg-gradient-to-r from-slate-900 to-slate-900/70 border border-slate-800 rounded-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="w-2.5 h-2.5 rounded-full bg-green-400"></span>
              <span className="font-semibold text-slate-100 text-sm">NVIDIA GPU Acceleration</span>
              {gpu.driver_version && (
                <span className="text-[10px] px-2 py-0.5 rounded bg-slate-800 text-slate-400 font-mono">
                  Driver {gpu.driver_version}
                </span>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            {gpu.gpus.map((g) => (
              <React.Fragment key={g.id}>
                <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800/80">
                  <div className="text-xs text-slate-400">GPU Utilization</div>
                  <div className="text-xl font-bold font-mono text-cyan-400 mt-1">{g.utilization.toFixed(0)}%</div>
                  <div className="text-[11px] text-slate-400 truncate mt-1">{g.name}</div>
                </div>

                <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800/80">
                  <div className="text-xs text-slate-400">VRAM Memory</div>
                  <div className="text-xl font-bold font-mono text-purple-400 mt-1">
                    {formatBytes(g.vram_used)}
                  </div>
                  <div className="text-[11px] text-slate-400 mt-1">Total: {formatBytes(g.vram_total)}</div>
                </div>

                <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800/80">
                  <div className="text-xs text-slate-400">Temperature</div>
                  <div className="text-xl font-bold font-mono text-amber-400 mt-1">{g.temperature.toFixed(0)} °C</div>
                  <div className="text-[11px] text-slate-400 mt-1">Fan: {g.fan_speed.toFixed(0)}%</div>
                </div>

                <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800/80">
                  <div className="text-xs text-slate-400">Power Draw</div>
                  <div className="text-xl font-bold font-mono text-emerald-400 mt-1">{g.power_draw.toFixed(0)} W</div>
                  <div className="text-[11px] text-slate-400 mt-1">Limit: {g.power_limit.toFixed(0)} W</div>
                </div>
              </React.Fragment>
            ))}
          </div>
        </div>
      )}

      {/* Summary Counters */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="p-4 bg-slate-900/40 border border-slate-800 rounded-xl flex items-center gap-4">
          <div className="p-3 rounded-lg bg-cyan-500/10 text-cyan-400">
            <IconLayers size={24} />
          </div>
          <div>
            <div className="text-2xl font-bold text-slate-100">{summary?.total_stacks || 0}</div>
            <div className="text-xs text-slate-400">
              Total Stacks ({summary?.running_stacks || 0} active)
            </div>
          </div>
        </div>

        <div className="p-4 bg-slate-900/40 border border-slate-800 rounded-xl flex items-center gap-4">
          <div className="p-3 rounded-lg bg-purple-500/10 text-purple-400">
            <IconBox size={24} />
          </div>
          <div>
            <div className="text-2xl font-bold text-slate-100">{summary?.total_containers || 0}</div>
            <div className="text-xs text-slate-400">
              Containers ({summary?.running_containers || 0} running, {summary?.stopped_containers || 0} stopped)
            </div>
          </div>
        </div>

        <div className="p-4 bg-slate-900/40 border border-slate-800 rounded-xl flex items-center gap-4">
          <div className="p-3 rounded-lg bg-emerald-500/10 text-emerald-400">
            <IconShield size={24} />
          </div>
          <div>
            <div className="text-2xl font-bold text-slate-100">{summary?.average_security_score || 100} / 100</div>
            <div className="text-xs text-slate-400">Average Compose Security Score</div>
          </div>
        </div>
      </div>

      {/* Stacks Overview Table */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
        <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/30">
          <h2 className="font-semibold text-slate-100 text-base flex items-center gap-2">
            <IconLayers size={18} className="text-cyan-400" /> Compose Stacks
          </h2>
          <span className="text-xs text-slate-400">{stacks.length} managed stack(s)</span>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-800 text-xs font-semibold text-slate-400 uppercase tracking-wider bg-slate-950/20">
                <th className="px-6 py-3">Stack</th>
                <th className="px-6 py-3">Status</th>
                <th className="px-6 py-3">Containers</th>
                <th className="px-6 py-3">Resource Usage</th>
                <th className="px-6 py-3">Security Score</th>
                <th className="px-6 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 text-sm">
              {stacks.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-8 text-center text-slate-400">
                    No stacks discovered yet. Create or import your first Compose stack!
                  </td>
                </tr>
              ) : (
                stacks.map((stk) => {
                  const isRunning = stk.status === 'running'
                  const isPartial = stk.status === 'partial'
                  const score = stk.security_score

                  const stkCpu = (stk.containers || []).reduce((acc, c) => {
                    const s = allStats[c.id] || allStats[c.id.slice(0, 12)]
                    return acc + (s?.cpu_percent || 0)
                  }, 0)
                  const stkMem = (stk.containers || []).reduce((acc, c) => {
                    const s = allStats[c.id] || allStats[c.id.slice(0, 12)]
                    return acc + (s?.memory_used || 0)
                  }, 0)

                  let scoreBadge = 'text-emerald-400 bg-emerald-950/50 border-emerald-800/60'
                  if (score < 50) scoreBadge = 'text-red-400 bg-red-950/50 border-red-800/60'
                  else if (score < 80) scoreBadge = 'text-amber-400 bg-amber-950/50 border-amber-800/60'

                  return (
                    <tr key={stk.id} className="hover:bg-slate-800/30 transition-colors">
                      <td className="px-6 py-4">
                        <div className="font-medium text-slate-100 flex items-center gap-2">
                          {stk.name}
                          {stk.is_system && (
                            <span className="text-[10px] uppercase font-bold tracking-wider px-1.5 py-0.5 rounded bg-blue-950 text-blue-300 border border-blue-800">
                              System
                            </span>
                          )}
                        </div>
                        <div className="text-xs text-slate-400 font-mono truncate max-w-xs">{stk.working_dir || stk.path}</div>
                      </td>

                      <td className="px-6 py-4">
                        <span
                          className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border ${
                            isRunning
                              ? 'bg-emerald-950/50 text-emerald-300 border-emerald-800/60'
                              : isPartial
                              ? 'bg-amber-950/50 text-amber-300 border-amber-800/60'
                              : 'bg-slate-800 text-slate-400 border-slate-700'
                          }`}
                        >
                          <span
                            className={`w-1.5 h-1.5 rounded-full ${
                              isRunning ? 'bg-emerald-400' : isPartial ? 'bg-amber-400' : 'bg-slate-400'
                            }`}
                          ></span>
                          {stk.status}
                        </span>
                      </td>

                      <td className="px-6 py-4 text-slate-300 font-mono text-xs">
                        {stk.containers?.length || 0} container(s)
                      </td>

                      <td className="px-6 py-4">
                        {isRunning || isPartial ? (
                          <div className="flex items-center gap-2 font-mono text-xs">
                            <span className="px-2 py-0.5 rounded bg-cyan-950/60 text-cyan-300 border border-cyan-800/60">
                              ⚡ {stkCpu.toFixed(1)}% CPU
                            </span>
                            <span className="px-2 py-0.5 rounded bg-indigo-950/60 text-indigo-300 border border-indigo-800/60">
                              💾 {formatBytes(stkMem)} RAM
                            </span>
                          </div>
                        ) : (
                          <span className="text-xs text-slate-400 font-mono">—</span>
                        )}
                      </td>

                      <td className="px-6 py-4">
                        <span className={`px-2.5 py-0.5 rounded-full text-xs font-semibold border ${scoreBadge}`}>
                          {score} / 100
                        </span>
                      </td>

                      <td className="px-6 py-4 text-right">
                        <button
                          onClick={() => onNavigateStack(stk.id)}
                          className="px-3 py-1.5 text-xs bg-slate-800 hover:bg-slate-700 text-cyan-400 hover:text-cyan-300 font-medium rounded-lg transition-colors border border-slate-700"
                        >
                          Manage Stack →
                        </button>
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
