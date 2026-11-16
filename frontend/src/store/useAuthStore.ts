import { create } from 'zustand'
import type { User } from '@/types'
import { authApi, type AuthResult } from '@/api/auth'
import { configureAuth } from '@/api/client'
import { useWorkspace } from './useWorkspaceStore'
import { useUIStore } from './useUIStore'

function savedToken() {
  try { return localStorage.getItem('kn-token') } catch { return null }
}

interface AuthState {
  token: string | null
  user: User | null
  avatarVersion: number
  initialized: boolean
  restoreError: string | null
  restore: () => Promise<void>
  accept: (result: AuthResult) => void
  updateUser: (user: User) => void
  logout: () => void
}

let restoring: Promise<void> | null = null
const initialToken = savedToken()

export const useAuthStore = create<AuthState>((set, get) => ({
  token: initialToken,
  user: null,
  avatarVersion: Date.now(),
  initialized: !initialToken,
  restoreError: null,
  accept: ({ token, user }) => {
    try { localStorage.setItem('kn-token', token) } catch { /* 存储不可用时保留本次会话 */ }
    set({ token, user, avatarVersion: Date.now(), initialized: true, restoreError: null })
  },
  updateUser: (user) => {
    if (get().user?.id === user.id) set({ user, avatarVersion: Math.max(Date.now(), get().avatarVersion + 1) })
  },
  logout: () => {
    try { localStorage.removeItem('kn-token') } catch { /* 存储不可用时仍清除内存登录态 */ }
    useWorkspace.getState().reset()
    useUIStore.getState().setSettingsOpen(false)
    useUIStore.getState().setHelpOpen(false)
    set({ token: null, user: null, initialized: true, restoreError: null })
  },
  restore: async () => {
    if (get().initialized || restoring) return restoring ?? undefined
    const token = get().token
    if (!token) { set({ initialized: true }); return }
    set({ restoreError: null })
    restoring = (async () => {
      try {
        const user = await authApi.me(token)
        if (get().token === token) set({ user, initialized: true })
      } catch (e) {
        if (get().token === token) set({ restoreError: (e as Error).message })
      } finally { restoring = null }
    })()
    await restoring
  },
}))

configureAuth(() => useAuthStore.getState().token, () => useAuthStore.getState().logout())
