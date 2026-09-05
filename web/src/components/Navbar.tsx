import React from 'react'
import { User } from '../types'
import { IconShield } from './Icons'

interface NavbarProps {
  user: User | null
}

export const Navbar: React.FC<NavbarProps> = ({ user }) => {
  const getRoleBadgeColor = (role?: string) => {
    switch (role) {
      case 'OWNER':
        return 'bg-purple-900/50 text-purple-300 border-purple-700/60'
      case 'ADMIN':
        return 'bg-cyan-900/50 text-cyan-300 border-cyan-700/60'
      case 'OPERATOR':
        return 'bg-amber-900/50 text-amber-300 border-amber-700/60'
      default:
        return 'bg-slate-800 text-slate-400 border-slate-700'
    }
  }

  return (
    <header className="h-16 border-b border-slate-800 bg-slate-900/80 backdrop-blur-md px-6 flex items-center justify-between sticky top-0 z-30">
      <div className="flex items-center gap-3">
        <div className="w-8 h-8 rounded-lg bg-gradient-to-tr from-cyan-600 to-sky-400 flex items-center justify-center font-bold text-slate-950 text-base shadow-lg shadow-cyan-500/20">
          DP
        </div>
        <div>
          <span className="font-semibold text-slate-100 tracking-tight">Docker Panel</span>
          <span className="ml-2 text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 border border-slate-700">
            v0.1-mvp
          </span>
        </div>
      </div>

      <div className="flex items-center gap-4">
        {/* Security Posture Status */}
        <div className="hidden md:flex items-center gap-1.5 px-3 py-1 rounded-full bg-emerald-950/40 border border-emerald-800/50 text-emerald-400 text-xs font-medium">
          <IconShield size={14} className="text-emerald-400" />
          <span>Zero Public Ports</span>
          <span className="text-emerald-600">•</span>
          <span className="text-emerald-300">CF Tunnel</span>
        </div>

        {/* User Identity */}
        {user ? (
          <div className="flex items-center gap-2.5">
            <div className="text-right">
              <div className="text-xs font-medium text-slate-200">{user.email}</div>
              <div className="text-[10px] text-slate-400">Cloudflare Authenticated</div>
            </div>
            <span
              className={`text-[10px] font-semibold tracking-wider uppercase px-2 py-0.5 rounded border ${getRoleBadgeColor(
                user.role
              )}`}
            >
              {user.role}
            </span>
          </div>
        ) : (
          <div className="text-xs text-slate-400">Authenticating...</div>
        )}
      </div>
    </header>
  )
}
