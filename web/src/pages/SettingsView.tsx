import React, { useEffect, useState } from 'react'
import { api } from '../api'
import { User } from '../types'
import { IconSettings, IconShield, IconCheckCircle } from '../components/Icons'

interface SettingsProps {
  user: User | null
}

export const SettingsView: React.FC<SettingsProps> = ({ user }) => {
  const [allowedPaths, setAllowedPaths] = useState('')
  const [retention, setRetention] = useState('7')
  const [saved, setSaved] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api.getSettings().then((s) => {
      if (s.allowed_host_paths) setAllowedPaths(s.allowed_host_paths)
      if (s.metrics_retention_days) setRetention(s.metrics_retention_days)
    }).catch(console.error)
  }, [])

  const handleSave = async () => {
    setSaving(true)
    setSaved(false)
    try {
      await api.updateSettings({
        allowed_host_paths: allowedPaths,
        metrics_retention_days: retention,
      })
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (err: any) {
      alert(`Failed to save settings: ${err.message}`)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="p-6 space-y-6 max-w-5xl mx-auto">
      {/* Header */}
      <div>
        <h1 className="text-xl font-bold text-slate-100 flex items-center gap-2">
          <IconSettings size={22} className="text-slate-400" />
          Panel Configuration & Security Posture
        </h1>
        <p className="text-xs text-slate-400 mt-0.5">Runtime settings, allowed filesystem paths, and zero-trust perimeter</p>
      </div>

      {saved && (
        <div className="p-3 bg-emerald-950/40 border border-emerald-800 rounded-lg text-xs text-emerald-300 flex items-center gap-2">
          <IconCheckCircle size={16} /> Settings saved successfully.
        </div>
      )}

      {/* Security Architecture Box */}
      <div className="p-5 bg-slate-900/60 border border-slate-800 rounded-xl space-y-4">
        <h3 className="text-sm font-semibold text-slate-100 flex items-center gap-2">
          <IconShield size={16} className="text-cyan-400" />
          Security Architecture Verification
        </h3>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
          <div className="p-3.5 bg-slate-950/60 rounded-lg border border-slate-800 space-y-1.5">
            <div className="font-semibold text-slate-200">Panel Core Isolation</div>
            <div className="text-slate-400">
              Mounts <span className="text-red-400 line-through">/var/run/docker.sock</span>: <strong className="text-emerald-400">NO</strong>
            </div>
            <div className="text-slate-400">
              IPC Communication: <strong className="text-slate-200 font-mono">/run/panel-agent/agent.sock</strong>
            </div>
            <div className="text-slate-400">
              Capabilities: <strong className="text-emerald-400 font-mono">cap_drop: ALL, read_only: true</strong>
            </div>
          </div>

          <div className="p-3.5 bg-slate-950/60 rounded-lg border border-slate-800 space-y-1.5">
            <div className="font-semibold text-slate-200">Panel Agent Isolation</div>
            <div className="text-slate-400">
              Network Mode: <strong className="text-emerald-400 font-mono">network_mode: none</strong> (Zero TCP/Internet)
            </div>
            <div className="text-slate-400">
              Docker Access: <strong className="text-amber-400">Dedicated agent only</strong>
            </div>
            <div className="text-slate-400">
              Filesystem: <strong className="text-slate-200 font-mono">/srv/docker-panel/stacks</strong> restricted
            </div>
          </div>
        </div>
      </div>

      {/* Form Settings */}
      <div className="p-5 bg-slate-900/60 border border-slate-800 rounded-xl space-y-5">
        <h3 className="text-sm font-semibold text-slate-100">Filesystem & Policy Settings</h3>

        <div className="space-y-2">
          <label className="block text-xs font-medium text-slate-300">
            Allowed Host Paths (One per line)
          </label>
          <div className="text-[11px] text-slate-400">
            Compose stacks can only mount volumes from these approved host paths.
          </div>
          <textarea
            value={allowedPaths}
            onChange={(e) => setAllowedPaths(e.target.value)}
            rows={4}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 font-mono text-xs text-slate-200 focus:outline-none focus:border-cyan-500"
          />
        </div>

        <div className="space-y-2">
          <label className="block text-xs font-medium text-slate-300">Metrics Retention (Days)</label>
          <input
            type="number"
            value={retention}
            onChange={(e) => setRetention(e.target.value)}
            className="w-32 bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 font-mono text-xs text-slate-200 focus:outline-none focus:border-cyan-500"
          />
        </div>

        <div className="pt-2">
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-5 py-2 bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-medium text-xs rounded-lg shadow-sm"
          >
            {saving ? 'Saving...' : 'Save Settings'}
          </button>
        </div>
      </div>
    </div>
  )
}
