import React, { useEffect, useState } from 'react'
import { api } from '../api'
import { HostMetrics, GPUMetrics } from '../types'
import { IconCpu, IconActivity, IconZap } from '../components/Icons'

export const MetricsView: React.FC = () => {
  const [range, setRange] = useState<'1h' | '6h' | '24h' | '7d'>('1h')
  const [history, setHistory] = useState<HostMetrics[]>([])
  const [liveHost, setLiveHost] = useState<HostMetrics | null>(null)
  const [gpu, setGpu] = useState<GPUMetrics | null>(null)

  const loadData = async () => {
    try {
      const [hist, live, g] = await Promise.all([
        api.getHostMetricsHistory(range),
        api.getHostMetrics(),
        api.getGPUMetrics(),
      ])
      setHistory(hist)
      setLiveHost(live)
      setGpu(g)
    } catch (err) {
      console.error(err)
    }
  }

  useEffect(() => {
    loadData()
    const timer = setInterval(loadData, 5000)
    return () => clearInterval(timer)
  }, [range])

  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }

  return (
    <div className="p-6 space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-100 flex items-center gap-2">
            <IconCpu size={22} className="text-cyan-400" />
            Host & Hardware Metrics
          </h1>
          <p className="text-xs text-slate-400 mt-0.5">Historical telemetry, load profiles, and GPU acceleration</p>
        </div>

        <div className="flex bg-slate-900 border border-slate-800 rounded-lg p-0.5 text-xs">
          {(['1h', '6h', '24h', '7d'] as const).map((r) => (
            <button
              key={r}
              onClick={() => setRange(r)}
              className={`px-3 py-1 rounded-md font-medium transition-colors ${
                range === r ? 'bg-cyan-500/20 text-cyan-300 font-semibold' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              {r}
            </button>
          ))}
        </div>
      </div>

      {/* Live Overview Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="p-4 bg-slate-900/60 border border-slate-800 rounded-xl">
          <div className="text-xs text-slate-400 font-medium">Current CPU</div>
          <div className="text-2xl font-bold font-mono text-cyan-400 mt-1">
            {liveHost?.cpu_percent?.toFixed(1) || '0.0'}%
          </div>
          <div className="text-[11px] text-slate-400 mt-1">
            Load: {liveHost?.load_1?.toFixed(2)}, {liveHost?.load_5?.toFixed(2)}, {liveHost?.load_15?.toFixed(2)}
          </div>
        </div>

        <div className="p-4 bg-slate-900/60 border border-slate-800 rounded-xl">
          <div className="text-xs text-slate-400 font-medium">Memory (RAM)</div>
          <div className="text-2xl font-bold font-mono text-purple-400 mt-1">
            {liveHost?.memory_percent?.toFixed(1) || '0.0'}%
          </div>
          <div className="text-[11px] text-slate-400 mt-1">
            {formatBytes(liveHost?.memory_used || 0)} used of {formatBytes(liveHost?.memory_total || 0)}
          </div>
        </div>

        <div className="p-4 bg-slate-900/60 border border-slate-800 rounded-xl">
          <div className="text-xs text-slate-400 font-medium">Disk Storage</div>
          <div className="text-2xl font-bold font-mono text-amber-400 mt-1">
            {liveHost?.disk_percent?.toFixed(1) || '0.0'}%
          </div>
          <div className="text-[11px] text-slate-400 mt-1">
            {formatBytes(liveHost?.disk_used || 0)} used of {formatBytes(liveHost?.disk_total || 0)}
          </div>
        </div>

        <div className="p-4 bg-slate-900/60 border border-slate-800 rounded-xl">
          <div className="text-xs text-slate-400 font-medium">Network I/O Rate</div>
          <div className="text-sm font-mono text-emerald-400 font-bold mt-2">
            &darr; {formatBytes(liveHost?.net_rx_bytes_rate || 0)}/s
          </div>
          <div className="text-sm font-mono text-cyan-400 font-bold mt-1">
            &uarr; {formatBytes(liveHost?.net_tx_bytes_rate || 0)}/s
          </div>
        </div>
      </div>

      {/* GPU Telemetry */}
      {gpu && gpu.available && gpu.gpus.length > 0 && (
        <div className="p-5 bg-slate-900/60 border border-slate-800 rounded-xl space-y-3">
          <h3 className="text-sm font-semibold text-slate-200 flex items-center gap-2">
            <IconZap size={16} className="text-emerald-400" />
            NVIDIA GPU Telemetry
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            {gpu.gpus.map((g) => (
              <React.Fragment key={g.id}>
                <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                  <div className="text-xs text-slate-400">Utilization</div>
                  <div className="text-xl font-bold font-mono text-cyan-400 mt-1">{g.utilization.toFixed(0)}%</div>
                  <div className="text-[11px] text-slate-400 mt-1">{g.name}</div>
                </div>
                <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                  <div className="text-xs text-slate-400">VRAM Allocation</div>
                  <div className="text-xl font-bold font-mono text-purple-400 mt-1">{formatBytes(g.vram_used)}</div>
                  <div className="text-[11px] text-slate-400 mt-1">Total: {formatBytes(g.vram_total)}</div>
                </div>
                <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                  <div className="text-xs text-slate-400">Core Temp & Fan</div>
                  <div className="text-xl font-bold font-mono text-amber-400 mt-1">{g.temperature.toFixed(0)} °C</div>
                  <div className="text-[11px] text-slate-400 mt-1">Fan Speed: {g.fan_speed.toFixed(0)}%</div>
                </div>
                <div className="p-3 bg-slate-950/60 rounded-lg border border-slate-800">
                  <div className="text-xs text-slate-400">Power Consumption</div>
                  <div className="text-xl font-bold font-mono text-emerald-400 mt-1">{g.power_draw.toFixed(0)} W</div>
                  <div className="text-[11px] text-slate-400 mt-1">Limit: {g.power_limit.toFixed(0)} W</div>
                </div>
              </React.Fragment>
            ))}
          </div>
        </div>
      )}

      {/* Metrics History Sparklines/Timeline */}
      <div className="p-5 bg-slate-900/60 border border-slate-800 rounded-xl space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-slate-200">
            CPU & Memory Utilization History ({range})
          </h3>
          <span className="text-xs text-slate-400">{history.length} samples recorded</span>
        </div>

        {history.length > 0 ? (
          <div className="space-y-4">
            {/* SVG Trend Chart */}
            <div className="h-44 w-full bg-slate-950/60 border border-slate-800 rounded-lg p-3 flex items-end gap-1 overflow-hidden">
              {history.map((pt, idx) => {
                const height = Math.max(4, Math.min(100, pt.cpu_percent))
                return (
                  <div
                    key={idx}
                    className="flex-1 bg-cyan-500/80 hover:bg-cyan-400 rounded-t transition-all cursor-pointer relative group"
                    style={{ height: `${height}%` }}
                  >
                    <div className="hidden group-hover:block absolute bottom-full mb-1 left-1/2 -translate-x-1/2 px-2 py-1 bg-slate-900 text-[10px] text-slate-200 rounded whitespace-nowrap shadow border border-slate-700 z-10 font-mono">
                      CPU: {pt.cpu_percent.toFixed(1)}% | RAM: {pt.memory_percent.toFixed(1)}%
                    </div>
                  </div>
                )
              })}
            </div>
            <div className="flex justify-between text-[11px] text-slate-400 font-mono">
              <span>{new Date(history[0]?.timestamp).toLocaleTimeString()}</span>
              <span>Timeline ({range})</span>
              <span>{new Date(history[history.length - 1]?.timestamp).toLocaleTimeString()}</span>
            </div>
          </div>
        ) : (
          <div className="p-10 text-center text-slate-400 text-xs">
            Collecting telemetry samples into SQLite... Samples will appear here shortly.
          </div>
        )}
      </div>
    </div>
  )
}
