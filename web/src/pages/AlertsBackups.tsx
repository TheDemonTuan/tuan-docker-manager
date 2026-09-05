import React, { useEffect, useState } from 'react'
import { api } from '../api'
import { AlertRule, AlertEvent, BackupRecord } from '../types'
import { IconAlertCircle, IconDownload, IconPlus, IconShield, IconCheckCircle } from '../components/Icons'

export const AlertsBackups: React.FC = () => {
  const [subTab, setSubTab] = useState<'alerts' | 'backups'>('alerts')
  const [rules, setRules] = useState<AlertRule[]>([])
  const [alertEvents, setAlertEvents] = useState<AlertEvent[]>([])
  const [backups, setBackups] = useState<BackupRecord[]>([])
  const [backingUp, setBackingUp] = useState(false)
  const [backupSuccess, setBackupSuccess] = useState<string | null>(null)

  const loadData = async () => {
    try {
      if (subTab === 'alerts') {
        const [r, ev] = await Promise.all([api.listAlertRules(), api.listAlertEvents()])
        setRules(r)
        setAlertEvents(ev)
      } else {
        const b = await api.listBackups()
        setBackups(b)
      }
    } catch (err) {
      console.error(err)
    }
  }

  useEffect(() => {
    loadData()
  }, [subTab])

  const handleCreateBackup = async () => {
    setBackingUp(true)
    setBackupSuccess(null)
    try {
      const rec = await api.createBackup()
      setBackupSuccess(`Encrypted backup '${rec.filename}' created successfully.`)
      loadData()
    } catch (err: any) {
      alert(`Backup failed: ${err.message}`)
    } finally {
      setBackingUp(false)
    }
  }

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
            <IconAlertCircle size={22} className="text-amber-400" />
            Alerts & Encrypted Backups
          </h1>
          <p className="text-xs text-slate-400 mt-0.5">Threshold triggers, system health alarms, and encrypted snapshots</p>
        </div>

        <div className="flex items-center gap-3">
          <div className="flex bg-slate-900 border border-slate-800 rounded-lg p-0.5 text-xs">
            <button
              onClick={() => setSubTab('alerts')}
              className={`px-3 py-1.5 rounded-md font-medium transition-colors ${
                subTab === 'alerts' ? 'bg-cyan-500/20 text-cyan-300 font-semibold' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Alert Rules ({rules.length})
            </button>
            <button
              onClick={() => setSubTab('backups')}
              className={`px-3 py-1.5 rounded-md font-medium transition-colors ${
                subTab === 'backups' ? 'bg-cyan-500/20 text-cyan-300 font-semibold' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Backups ({backups.length})
            </button>
          </div>

          {subTab === 'backups' && (
            <button
              onClick={handleCreateBackup}
              disabled={backingUp}
              className="flex items-center gap-1.5 px-3.5 py-1.5 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white text-xs font-semibold rounded-lg shadow-sm"
            >
              <IconPlus size={14} />
              {backingUp ? 'Creating Encrypted Archive...' : 'Create Backup Now'}
            </button>
          )}
        </div>
      </div>

      {backupSuccess && (
        <div className="p-3 bg-emerald-950/40 border border-emerald-800 rounded-lg text-xs text-emerald-300 flex items-center gap-2">
          <IconCheckCircle size={16} />
          {backupSuccess}
        </div>
      )}

      {/* 1. Alerts Rules & Events */}
      {subTab === 'alerts' && (
        <div className="space-y-6">
          <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
            <div className="p-4 border-b border-slate-800 bg-slate-950/40">
              <div className="text-xs font-semibold text-slate-300">Configured Alert Rules</div>
            </div>

            <div className="divide-y divide-slate-800/60">
              {rules.map((r) => (
                <div key={r.id} className="p-4 flex items-center justify-between text-xs hover:bg-slate-800/30">
                  <div>
                    <div className="font-semibold text-slate-200 text-sm flex items-center gap-2">
                      {r.name}
                      <span
                        className={`text-[10px] uppercase font-bold px-2 py-0.5 rounded ${
                          r.severity === 'critical'
                            ? 'bg-red-950 text-red-400 border border-red-800'
                            : 'bg-amber-950 text-amber-400 border border-amber-800'
                        }`}
                      >
                        {r.severity}
                      </span>
                    </div>
                    <div className="text-slate-400 mt-1 font-mono">
                      Condition: {r.metric_type} {r.condition} {r.threshold} (sustained for {r.duration_seconds}s)
                    </div>
                  </div>

                  <span className="px-2.5 py-1 rounded-full text-xs font-medium bg-emerald-950/60 text-emerald-400 border border-emerald-800/60">
                    Active
                  </span>
                </div>
              ))}
            </div>
          </div>

          <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
            <div className="p-4 border-b border-slate-800 bg-slate-950/40">
              <div className="text-xs font-semibold text-slate-300">Alert History</div>
            </div>

            <div className="divide-y divide-slate-800/60">
              {alertEvents.length === 0 ? (
                <div className="p-8 text-center text-slate-400 text-xs">No alert events fired. System healthy!</div>
              ) : (
                alertEvents.map((ae) => (
                  <div key={ae.id} className="p-4 flex items-center justify-between text-xs">
                    <div>
                      <div className="font-semibold text-slate-200">{ae.rule_name}</div>
                      <div className="text-slate-400 mt-0.5">{ae.message}</div>
                    </div>
                    <div className="text-right">
                      <span className="text-[11px] font-mono text-slate-400">
                        {new Date(ae.started_at).toLocaleString()}
                      </span>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}

      {/* 2. Backups List */}
      {subTab === 'backups' && (
        <div className="space-y-4">
          <div className="p-4 bg-slate-950/50 border border-slate-800 rounded-xl text-xs text-slate-300 flex items-center gap-3">
            <IconShield size={22} className="text-cyan-400 shrink-0" />
            <div>
              <div className="font-semibold text-slate-100">Zero-Plaintext Encrypted Backup Architecture</div>
              <div className="text-slate-400 text-[11px] mt-0.5">
                Archives bundle panel database, Compose files, and .env files, compressed with gzip and encrypted at rest
                with XChaCha20-Poly1305 using the host master key.
              </div>
            </div>
          </div>

          <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-slate-800 text-xs font-semibold text-slate-400 uppercase tracking-wider bg-slate-950/20">
                  <th className="px-6 py-3">Archive File</th>
                  <th className="px-6 py-3">Size</th>
                  <th className="px-6 py-3">Encryption</th>
                  <th className="px-6 py-3">Created</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 text-sm">
                {backups.length === 0 ? (
                  <tr>
                    <td colSpan={4} className="px-6 py-8 text-center text-slate-400">
                      No backups found. Click "Create Backup Now" to create your first encrypted snapshot.
                    </td>
                  </tr>
                ) : (
                  backups.map((b) => (
                    <tr key={b.id} className="hover:bg-slate-800/30 transition-colors text-xs">
                      <td className="px-6 py-4 font-mono font-semibold text-slate-200">{b.filename}</td>
                      <td className="px-6 py-4 font-mono text-slate-300">{formatBytes(b.size_bytes)}</td>
                      <td className="px-6 py-4">
                        <span className="px-2 py-0.5 rounded bg-emerald-950 text-emerald-400 border border-emerald-800 text-[10px] font-bold uppercase">
                          XChaCha20 Encrypted
                        </span>
                      </td>
                      <td className="px-6 py-4 font-mono text-slate-400">{new Date(b.created_at).toLocaleString()}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
