import React, { useEffect, useState } from 'react'
import { api } from './api'
import { User } from './types'
import { Navbar } from './components/Navbar'
import { Sidebar, TabKey } from './components/Sidebar'
import { Dashboard } from './pages/Dashboard'
import { Stacks } from './pages/Stacks'
import { Containers } from './pages/Containers'
import { Resources } from './pages/Resources'
import { MetricsView } from './pages/MetricsView'
import { EventsAudit } from './pages/EventsAudit'
import { AlertsBackups } from './pages/AlertsBackups'
import { SettingsView } from './pages/SettingsView'

export const App: React.FC = () => {
  const [user, setUser] = useState<User | null>(null)
  const [currentTab, setCurrentTab] = useState<TabKey>('dashboard')
  const [selectedStackId, setSelectedStackId] = useState<string | null>(null)

  useEffect(() => {
    api.getMe()
      .then((res) => {
        setUser(res.user)
      })
      .catch((err) => {
        console.error('Auth error:', err)
      })
  }, [])

  const handleNavigateStack = (stackId: string) => {
    setSelectedStackId(stackId)
    setCurrentTab('stacks')
  }

  return (
    <div className="flex flex-col min-h-screen bg-slate-950 text-slate-100">
      <Navbar user={user} />

      <div className="flex flex-1 overflow-hidden">
        <Sidebar currentTab={currentTab} onSelectTab={(tab) => {
          if (tab !== 'stacks') {
            setSelectedStackId(null)
          }
          setCurrentTab(tab)
        }} />

        <main className="flex-1 overflow-y-auto bg-gradient-to-b from-slate-950 via-slate-900/20 to-slate-950">
          {currentTab === 'dashboard' && <Dashboard onNavigateStack={handleNavigateStack} />}
          {currentTab === 'stacks' && (
            <Stacks selectedStackId={selectedStackId} onClearSelected={() => setSelectedStackId(null)} />
          )}
          {currentTab === 'containers' && <Containers />}
          {(currentTab === 'images' || currentTab === 'volumes' || currentTab === 'networks') && <Resources />}
          {currentTab === 'metrics' && <MetricsView />}
          {(currentTab === 'events' || currentTab === 'audit') && <EventsAudit />}
          {currentTab === 'alerts' && <AlertsBackups />}
          {currentTab === 'settings' && <SettingsView user={user} />}
        </main>
      </div>
    </div>
  )
}
