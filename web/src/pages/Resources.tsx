import React, { useEffect, useState } from 'react'
import { api } from '../api'
import { ImageInfo, VolumeInfo, NetworkInfo, StorageSnapshot } from '../types'
import {
  IconDownload,
  IconHardDrive,
  IconActivity,
  IconTrash,
  IconPlus,
} from '../components/Icons'
import { Modal, CriticalConfirmModal } from '../components/Modal'

export const Resources: React.FC = () => {
  const [tab, setTab] = useState<'images' | 'volumes' | 'networks'>('images')

  // Images state
  const [images, setImages] = useState<ImageInfo[]>([])
  const [pullModalOpen, setPullModalOpen] = useState(false)
  const [imageToPull, setImageToPull] = useState('')
  const [pulling, setPulling] = useState(false)

  // Volumes state
  const [volumes, setVolumes] = useState<VolumeInfo[]>([])
  const [volumeToDelete, setVolumeToDelete] = useState<string | null>(null)
  const [isPruneVolumesOpen, setIsPruneVolumesOpen] = useState(false)

  // Networks state
  const [networks, setNetworks] = useState<NetworkInfo[]>([])
  const [storage, setStorage] = useState<StorageSnapshot | null>(null)

  const loadData = async () => {
    try {
      if (tab === 'images') {
        const res = await api.listImages()
        setImages(res)
      } else if (tab === 'volumes') {
        const res = await api.listVolumes()
        setVolumes(res)
      } else if (tab === 'networks') {
        const res = await api.listNetworks()
        setNetworks(res)
      }
    } catch (err) {
      console.error(err)
    }
  }

  useEffect(() => {
    loadData()
    api.getStorage().then(setStorage).catch(() => setStorage(null))
  }, [tab])

  const handlePullImage = async () => {
    if (!imageToPull.trim()) return
    setPulling(true)
    try {
      await api.pullImage(imageToPull.trim())
      setPullModalOpen(false)
      setImageToPull('')
      loadData()
    } catch (err: any) {
      alert(`Pull failed: ${err.message}`)
    } finally {
      setPulling(false)
    }
  }

  const handleDeleteImage = async (id: string) => {
    if (!confirm('Are you sure you want to remove this image?')) return
    try {
      await api.deleteImage(id, false)
      loadData()
    } catch (err: any) {
      alert(`Delete failed: ${err.message}`)
    }
  }

  const handlePruneImages = async () => {
    if (!confirm('Prune unused and dangling images?')) return
    try {
      await api.pruneImages()
      loadData()
    } catch (err: any) {
      alert(`Prune failed: ${err.message}`)
    }
  }

  const handleDeleteVolume = async () => {
    if (!volumeToDelete) return
    try {
      await api.deleteVolume(volumeToDelete, true)
      setVolumeToDelete(null)
      loadData()
    } catch (err: any) {
      alert(`Delete volume failed: ${err.message}`)
    }
  }

  const handlePruneVolumes = async () => {
    try {
      await api.pruneVolumes(true)
      setIsPruneVolumesOpen(false)
      loadData()
    } catch (err: any) {
      alert(`Prune volumes failed: ${err.message}`)
    }
  }

  const handleDeleteNetwork = async (id: string) => {
    if (!confirm('Remove this Docker network?')) return
    try {
      await api.deleteNetwork(id)
      loadData()
    } catch (err: any) {
      alert(`Delete network failed: ${err.message}`)
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
            <IconHardDrive size={22} className="text-amber-400" />
            Storage & Resources
          </h1>
          <p className="text-xs text-slate-400 mt-0.5">Manage Docker images, persistent volumes, and bridge networks</p>
        </div>

        {/* Action button based on tab */}
        <div className="flex items-center gap-3">
          {tab === 'images' && (
            <>
              <button
                onClick={handlePruneImages}
                className="px-3 py-1.5 text-xs bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg border border-slate-700"
              >
                Prune Unused Images
              </button>
              <button
                onClick={() => setPullModalOpen(true)}
                className="flex items-center gap-1.5 px-3.5 py-1.5 bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-medium rounded-lg shadow-sm"
              >
                <IconPlus size={14} /> Pull Image
              </button>
            </>
          )}

          {tab === 'volumes' && (
            <button
              onClick={() => setIsPruneVolumesOpen(true)}
              className="flex items-center gap-1.5 px-3.5 py-1.5 bg-red-900/60 hover:bg-red-800 text-red-200 border border-red-700 text-xs font-medium rounded-lg"
            >
              <IconTrash size={14} /> Prune Unused Volumes
            </button>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-slate-800 text-xs font-semibold text-slate-400">
        <button
          onClick={() => setTab('images')}
          className={`py-3 px-5 border-b-2 flex items-center gap-2 transition-colors ${
            tab === 'images' ? 'border-cyan-400 text-cyan-400' : 'border-transparent hover:text-slate-200'
          }`}
        >
          <IconDownload size={15} /> Images ({images.length})
        </button>
        <button
          onClick={() => setTab('volumes')}
          className={`py-3 px-5 border-b-2 flex items-center gap-2 transition-colors ${
            tab === 'volumes' ? 'border-cyan-400 text-cyan-400' : 'border-transparent hover:text-slate-200'
          }`}
        >
          <IconHardDrive size={15} /> Volumes ({volumes.length})
        </button>
        <button
          onClick={() => setTab('networks')}
          className={`py-3 px-5 border-b-2 flex items-center gap-2 transition-colors ${
            tab === 'networks' ? 'border-cyan-400 text-cyan-400' : 'border-transparent hover:text-slate-200'
          }`}
        >
          <IconActivity size={15} /> Networks ({networks.length})
        </button>
      </div>

      {/* 1. Images Table */}
      {tab === 'images' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-800 text-xs font-semibold text-slate-400 uppercase tracking-wider bg-slate-950/20">
                <th className="px-6 py-3">Repository & Tag</th>
                <th className="px-6 py-3">ID</th>
                <th className="px-6 py-3">Size</th>
                <th className="px-6 py-3">Active Containers</th>
                <th className="px-6 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 text-sm">
              {images.map((img) => (
                <tr key={img.id} className="hover:bg-slate-800/30 transition-colors">
                  <td className="px-6 py-4 font-mono text-xs text-slate-200">
                    {img.repo_tags && img.repo_tags.length > 0 ? (
                      img.repo_tags.map((t, idx) => (
                        <div key={idx} className="font-semibold text-cyan-300">
                          {t}
                        </div>
                      ))
                    ) : (
                      <span className="text-slate-400 italic">&lt;untagged / dangling&gt;</span>
                    )}
                  </td>
                  <td className="px-6 py-4 font-mono text-xs text-slate-400">{img.id.substring(0, 19)}</td>
                  <td className="px-6 py-4 font-mono text-xs text-slate-300">{formatBytes(img.size)}</td>
                  <td className="px-6 py-4 text-xs text-slate-300">{img.containers}</td>
                  <td className="px-6 py-4 text-right">
                    <button
                      onClick={() => handleDeleteImage(img.id)}
                      className="p-1.5 text-slate-400 hover:text-red-400 hover:bg-slate-800 rounded transition-colors"
                      title="Delete Image"
                    >
                      <IconTrash size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* 2. Volumes Table */}
      {tab === 'volumes' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-800 text-xs font-semibold text-slate-400 uppercase tracking-wider bg-slate-950/20">
                <th className="px-6 py-3">Volume Name</th>
                <th className="px-6 py-3">Driver</th>
                <th className="px-6 py-3">Size / References</th>
                <th className="px-6 py-3">Mountpoint</th>
                <th className="px-6 py-3">Created</th>
                <th className="px-6 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 text-sm">
              {volumes.map((v) => (
                <tr key={v.name} className="hover:bg-slate-800/30 transition-colors">
                  <td className="px-6 py-4 font-mono text-xs font-semibold text-slate-200">{v.name}</td>
                  <td className="px-6 py-4 text-xs text-slate-400">{v.driver}</td>
                  <td className="px-6 py-4">
                    {(() => {
                      const usage = storage?.volumes[v.name]
                      return (
                        <div className="font-mono text-xs text-amber-300">
                          {usage?.bytes === undefined ? '—' : formatBytes(usage.bytes || 0)}
                          {usage && <div className="text-[10px] text-slate-500 mt-0.5">{usage.ref_count} container{usage.ref_count === 1 ? '' : 's'}{usage.stack_names?.length ? ` · ${usage.stack_names.join(', ')}` : ''}</div>}
                        </div>
                      )
                    })()}
                  </td>
                  <td className="px-6 py-4 font-mono text-[11px] text-slate-400 truncate max-w-xs">{v.mountpoint}</td>
                  <td className="px-6 py-4 text-xs text-slate-400">{v.created_at?.substring(0, 10) || '—'}</td>
                  <td className="px-6 py-4 text-right">
                    <button
                      onClick={() => setVolumeToDelete(v.name)}
                      className="p-1.5 text-slate-400 hover:text-red-400 hover:bg-slate-800 rounded transition-colors"
                      title="Delete Volume"
                    >
                      <IconTrash size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* 3. Networks Table */}
      {tab === 'networks' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-800 text-xs font-semibold text-slate-400 uppercase tracking-wider bg-slate-950/20">
                <th className="px-6 py-3">Network Name</th>
                <th className="px-6 py-3">ID</th>
                <th className="px-6 py-3">Driver</th>
                <th className="px-6 py-3">Scope</th>
                <th className="px-6 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 text-sm">
              {networks.map((n) => (
                <tr key={n.id} className="hover:bg-slate-800/30 transition-colors">
                  <td className="px-6 py-4 font-medium text-slate-200 text-xs">{n.name}</td>
                  <td className="px-6 py-4 font-mono text-xs text-slate-400">{n.id.substring(0, 12)}</td>
                  <td className="px-6 py-4 text-xs text-slate-400">{n.driver}</td>
                  <td className="px-6 py-4 text-xs text-slate-400">{n.scope}</td>
                  <td className="px-6 py-4 text-right">
                    {n.name !== 'bridge' && n.name !== 'host' && n.name !== 'none' && (
                      <button
                        onClick={() => handleDeleteNetwork(n.id)}
                        className="p-1.5 text-slate-400 hover:text-red-400 hover:bg-slate-800 rounded transition-colors"
                        title="Delete Network"
                      >
                        <IconTrash size={14} />
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Modal: Pull Image */}
      <Modal isOpen={pullModalOpen} onClose={() => setPullModalOpen(false)} title="Pull Docker Image">
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">Image Name & Tag</label>
            <input
              type="text"
              value={imageToPull}
              onChange={(e) => setImageToPull(e.target.value)}
              placeholder="e.g. redis:7-alpine, nginx:1.27, postgres:16"
              className="w-full bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-100 font-mono focus:outline-none focus:border-cyan-500"
            />
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <button
              onClick={() => setPullModalOpen(false)}
              className="px-4 py-2 text-sm bg-slate-800 text-slate-300 rounded-lg"
            >
              Cancel
            </button>
            <button
              onClick={handlePullImage}
              disabled={pulling || !imageToPull.trim()}
              className="px-4 py-2 text-sm bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-medium rounded-lg"
            >
              {pulling ? 'Pulling Image...' : 'Pull Image'}
            </button>
          </div>
        </div>
      </Modal>

      {/* Critical Modal: Delete Volume */}
      {volumeToDelete && (
        <CriticalConfirmModal
          isOpen={true}
          onClose={() => setVolumeToDelete(null)}
          onConfirm={handleDeleteVolume}
          title={`Delete Volume: ${volumeToDelete}`}
          message={`Are you sure you want to permanently delete volume '${volumeToDelete}'? Any stored application data inside this volume will be permanently erased.`}
          confirmWord={volumeToDelete}
        />
      )}

      {/* Critical Modal: Prune Volumes */}
      <CriticalConfirmModal
        isOpen={isPruneVolumesOpen}
        onClose={() => setIsPruneVolumesOpen(false)}
        onConfirm={handlePruneVolumes}
        title="Prune Unused Docker Volumes"
        message="This operation will permanently delete all anonymous and named volumes not currently mounted by at least one active container."
        confirmWord="PRUNE"
      />
    </div>
  )
}
