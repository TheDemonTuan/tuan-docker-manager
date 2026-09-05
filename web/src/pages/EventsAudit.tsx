import React, { useEffect, useState } from 'react'
import { api } from '../api'
import { DockerEvent, AuditLog } from '../types'
import { IconActivity, IconShield } from '../components/Icons'

export const EventsAudit: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'events' | 'audit'>('events')
  const [events, setEvents] = useState<DockerEvent[]>([])
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([])
  const [connected, setConnected] = useState(false)

  // Realtime Docker events SSE
  useEffect(() => {
    if (activeTab !== 'events') return

    const es = new EventSource('/api/v1/events')
    es.onopen = () => setConnected(true)
    es.onerror = () => setConnected(false)

    es.onmessage = (e) => {
      try {
        const ev: DockerEvent = JSON.parse(e.data)
        setEvents((prev) => [ev, ...prev.slice(0, 100)])
      } catch {
        // ignore ping
      }
    }

    return () => {
      es.close()
    }
  }, [activeTab])

  // Load audit logs
  useEffect(() => {
    if (activeTab !== 'audit') return
    api.getAuditLogs(100).then(setAuditLogs).catch(console.error)
  }, [activeTab])

  return (
    <div className="p-6 space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-100 flex items-center gap-2">
            <IconShield size={22} className="text-cyan-400" />
            Events & Audit Trail
          </h1>
          <p className="text-xs text-slate-400 mt-0.5">Realtime Docker daemon activity and immutable user audit trail</p>
        </div>

        <div className="flex border border-slate-800 bg-slate-900 rounded-lg p-0.5 text-xs">
          <button
            onClick={() => setActiveTab('events')}
            className={`px-3 py-1.5 rounded-md font-medium transition-colors flex items-center gap-2 ${
              activeTab === 'events' ? 'bg-cyan-500/20 text-cyan-300 font-semibold' : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <IconActivity size={14} /> Docker Events
            {connected && <span className="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>}
          </button>
          <button
            onClick={() => setActiveTab('audit')}
            className={`px-3 py-1.5 rounded-md font-medium transition-colors flex items-center gap-2 ${
              activeTab === 'audit' ? 'bg-cyan-500/20 text-cyan-300 font-semibold' : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <IconShield size={14} /> Audit Trail
          </button>
        </div>
      </div>

      {/* 1. Docker Events SSE */}
      {activeTab === 'events' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
          <div className="p-4 border-b border-slate-800 bg-slate-950/40 flex items-center justify-between">
            <div className="text-xs font-semibold text-slate-300">Live Docker Daemon Stream</div>
            <div className="flex items-center gap-2 text-xs text-slate-400">
              <span className={`w-2 h-2 rounded-full ${connected ? 'bg-emerald-400 animate-pulse' : 'bg-red-400'}`}></span>
              <span>{connected ? 'Connected (SSE)' : 'Connecting...'}</span>
            </div>
          </div>

          <div className="divide-y divide-slate-800/60 max-h-[70vh] overflow-y-auto">
            {events.length === 0 ? (
              <div className="p-8 text-center text-slate-400 text-xs">
                Listening for Docker daemon events (container start, die, exec, image pull)...
              </div>
            ) : (
              events.map((ev, i) => (
                <div key={i} className="p-3.5 hover:bg-slate-800/30 transition-colors flex items-center justify-between text-xs">
                  <div className="flex items-center gap-3">
                    <span className="font-mono text-[11px] px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-semibold uppercase">
                      {ev.action}
                    </span>
                    <span className="font-medium text-slate-200">{ev.actor_name || ev.actor_id.substring(0, 12)}</span>
                    <span className="text-slate-400 font-mono text-[11px]">{ev.type}</span>
                  </div>
                  <span className="text-slate-400 font-mono text-[11px]">
                    {new Date(ev.timestamp).toLocaleTimeString()}
                  </span>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* 2. Persistent Audit Logs */}
      {activeTab === 'audit' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-800 text-xs font-semibold text-slate-400 uppercase tracking-wider bg-slate-950/20">
                <th className="px-6 py-3">Timestamp</th>
                <th className="px-6 py-3">User Email</th>
                <th className="px-6 py-3">Action</th>
                <th className="px-6 py-3">Target Resource</th>
                <th className="px-6 py-3">IP Address</th>
                <th className="px-6 py-3 text-right">Result</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 text-sm">
              {auditLogs.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-8 text-center text-slate-400">
                    No audit logs recorded yet.
                  </td>
                </tr>
              ) : (
                auditLogs.map((log) => (
                  <tr key={log.id} className="hover:bg-slate-800/30 transition-colors text-xs">
                    <td className="px-6 py-3.5 font-mono text-slate-400">
                      {new Date(log.created_at).toLocaleString()}
                    </td>
                    <td className="px-6 py-3.5 font-medium text-slate-200">{log.user_email}</td>
                    <td className="px-6 py-3.5 font-mono text-cyan-300 font-semibold">{log.action}</td>
                    <td className="px-6 py-3.5 font-mono text-slate-300">{log.resource}</td>
                    <td className="px-6 py-3.5 font-mono text-slate-400">{log.ip}</td>
                    <td className="px-6 py-3.5 text-right">
                      <span
                        className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase ${
                          log.result === 'success'
                            ? 'bg-emerald-950 text-emerald-400 border border-emerald-800'
                            : 'bg-red-950 text-red-400 border border-red-800'
                        }`}
                      >
                        {log.result}
                      </span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
