import { create } from 'zustand'
import { uid } from '@/lib/utils'

export type Toast = { id: string; message: string; tone: 'default' | 'success' | 'error' }
export type ConfirmOptions = { title: string; description?: string; confirmText?: string; danger?: boolean }
type Theme = 'light' | 'dark'
export type MobileTab = 'sources' | 'chat' | 'studio'

interface UIState {
  theme: Theme
  toggleTheme: () => void
  toasts: Toast[]
  toast: (message: string, tone?: Toast['tone']) => void
  dismissToast: (id: string) => void
  confirmState: (ConfirmOptions & { resolve: (ok: boolean) => void }) | null
  confirm: (opts: ConfirmOptions) => Promise<boolean>
  settingsOpen: boolean
  setSettingsOpen: (v: boolean) => void
  /** 移动端工作台当前面板 */
  mobileTab: MobileTab
  setMobileTab: (t: MobileTab) => void
}

const initialTheme: Theme = document.documentElement.classList.contains('dark') ? 'dark' : 'light'

export const useUIStore = create<UIState>((set, get) => ({
  theme: initialTheme,
  toggleTheme: () => {
    const next = get().theme === 'dark' ? 'light' : 'dark'
    document.documentElement.classList.toggle('dark', next === 'dark')
    try {
      localStorage.setItem('kn-theme', next)
    } catch {
      /* 隐私模式下忽略 */
    }
    set({ theme: next })
  },
  toasts: [],
  toast: (message, tone = 'default') => {
    if (get().toasts.some((t) => t.message === message && t.tone === tone)) return
    const id = uid('t')
    set((s) => ({ toasts: [...s.toasts, { id, message, tone }] }))
    setTimeout(() => get().dismissToast(id), 2800)
  },
  dismissToast: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
  confirmState: null,
  confirm: (opts) =>
    new Promise<boolean>((resolve) =>
      set({
        confirmState: {
          ...opts,
          resolve: (ok) => {
            set({ confirmState: null })
            resolve(ok)
          },
        },
      }),
    ),
  settingsOpen: false,
  setSettingsOpen: (v) => set({ settingsOpen: v }),
  mobileTab: 'chat',
  setMobileTab: (t) => set({ mobileTab: t }),
}))

export const toast = (m: string, tone?: Toast['tone']) => useUIStore.getState().toast(m, tone)
export const confirmDialog = (o: ConfirmOptions) => useUIStore.getState().confirm(o)
