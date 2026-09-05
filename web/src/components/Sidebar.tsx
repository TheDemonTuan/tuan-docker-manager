import React from 'react'
import {
  IconServer,
  IconLayers,
  IconBox,
  IconHardDrive,
  IconCpu,
  IconActivity,
  IconAlertCircle,
  IconShield,
  IconSettings,
  IconDownload,
} from './Icons'

// Define Tab type
export type TabKey =
  | 'dashboard'
  | 'stacks'
  | 'containers'
  | 'images'
  | 'volumes'
  | 'networks'
  | 'metrics'
  | 'events'
  | 'alerts'
  | 'audit'
  | 'settings'

interface SidebarProps {
  currentTab: TabKey
  onSelectTab: (tab: TabKey) => void
}

export const Sidebar: React.FC<SidebarProps> = ({ currentTab, onSelectTab }) => {
  const navItems: Array<{ key: TabKey; label: string; icon: React.ReactNode; group?: string }> = [
    { key: 'dashboard', label: 'Dashboard', icon: <IconServer size={18} /> },
    { key: 'stacks', label: 'Stacks', icon: <IconLayers size={18} /> },
    { key: 'containers', label: 'Containers', icon: <IconBox size={18} /> },
    { key: 'images', label: 'Images', icon: <IconDownload size={18} />, group: 'Resources' },
    { key: 'volumes', label: 'Volumes', icon: <IconHardDrive size={18} /> },
    { key: 'networks', label: 'Networks', icon: <IconActivity size={18} /> },
    { key: 'metrics', label: 'Host & GPU', icon: <IconCpu size={18} />, group: 'Monitoring' },
    { key: 'events', label: 'Events', icon: <IconActivity size={18} /> },
    { key: 'alerts', label: 'Alerts', icon: <IconAlertCircle size={18} /> },
    { key: 'audit', label: 'Audit Log', icon: <IconShield size={18} />, group: 'Administration' },
    { key: 'settings', label: 'Backups & Settings', icon: <IconSettings size={18} /> },
  ]

  let lastGroup: string | undefined = undefined

  return (
    <aside className="w-64 border-r border-slate-800 bg-slate-900/40 p-4 flex flex-col justify-between shrink-0">
      <nav className="space-y-1">
        {navItems.map((item) => {
          const showGroup = item.group && item.group !== lastGroup
          if (item.group) {
            lastGroup = item.group
          }

          return (
            <React.Fragment key={item.key}>
              {showGroup && (
                <div className="pt-4 pb-1 px-3 text-[11px] font-semibold text-slate-400 uppercase tracking-wider">
                  {item.group}
                </div>
              )}
              <button
                onClick={() => onSelectTab(item.key)}
                className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all ${
                  currentTab === item.key
                    ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 shadow-sm'
                    : 'text-slate-300 hover:text-slate-100 hover:bg-slate-800/60'
                }`}
              >
                <span className={currentTab === item.key ? 'text-cyan-400' : 'text-slate-400'}>{item.icon}</span>
                <span>{item.label}</span>
              </button>
            </React.Fragment>
          )
        })}
      </nav>

      {/* Agent Socket Status */}
      <div className="p-3 bg-slate-950/60 border border-slate-800 rounded-lg text-xs space-y-1">
        <div className="text-slate-400 flex items-center justify-between">
          <span>Agent Socket</span>
          <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
        </div>
        <div className="font-mono text-[11px] text-slate-300 truncate">/run/panel-agent/agent.sock</div>
        <div className="text-[10px] text-slate-400">Core never mounts docker.sock</div>
      </div>
    </aside>
  )
}
