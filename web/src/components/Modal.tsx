import React, { useState } from 'react'

interface ModalProps {
  isOpen: boolean
  onClose: () => void
  title: React.ReactNode
  children: React.ReactNode
  maxWidth?: string
}

export const Modal: React.FC<ModalProps> = ({ isOpen, onClose, title, children, maxWidth = 'max-w-2xl' }) => {
  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fadeIn">
      <div className={`relative w-full ${maxWidth} bg-slate-900 border border-slate-700/80 rounded-xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]`}>
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-950/40">
          <h3 className="text-lg font-semibold text-slate-100">{title}</h3>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-white transition-colors p-1 rounded-lg hover:bg-slate-800"
          >
            ✕
          </button>
        </div>
        <div className="p-6 overflow-y-auto flex-1">{children}</div>
      </div>
    </div>
  )
}

interface CriticalConfirmModalProps {
  isOpen: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  message: string
  confirmWord?: string
}

export const CriticalConfirmModal: React.FC<CriticalConfirmModalProps> = ({
  isOpen,
  onClose,
  onConfirm,
  title,
  message,
  confirmWord = 'DELETE',
}) => {
  const [input, setInput] = useState('')

  if (!isOpen) return null

  const isMatched = input.trim().toUpperCase() === confirmWord.toUpperCase()

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={title} maxWidth="max-w-md">
      <div className="space-y-4">
        <div className="p-3 bg-red-950/40 border border-red-800/60 rounded-lg text-red-200 text-sm">
          <div className="font-semibold mb-1 flex items-center gap-1.5">
            <span>⚠️</span> Critical Security Action
          </div>
          <div>{message}</div>
        </div>

        <div className="text-xs text-slate-400">
          To confirm, type <span className="font-mono text-red-400 font-bold">{confirmWord}</span> below:
        </div>

        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={confirmWord}
          className="w-full bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-100 focus:outline-none focus:border-red-500 font-mono"
        />

        <div className="flex justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition-colors"
          >
            Cancel
          </button>
          <button
            type="button"
            disabled={!isMatched}
            onClick={() => {
              onConfirm()
              setInput('')
              onClose()
            }}
            className="px-4 py-2 text-sm bg-red-600 hover:bg-red-500 disabled:opacity-40 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-colors shadow-lg shadow-red-900/30"
          >
            Confirm Action
          </button>
        </div>
      </div>
    </Modal>
  )
}
