import { useEffect } from 'react'
import { BrowserRouter, Link, Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom'
import HomePage from '@/pages/HomePage'
import LandingPage from '@/pages/LandingPage'
import NotebookPage from '@/pages/NotebookPage'
import AuthPage from '@/pages/AuthPage'
import AdminPage from '@/pages/AdminPage'
import NotFound from '@/pages/NotFound'
import { LogoMark } from '@/components/layout/Logo'
import { Toaster, ConfirmHost, SettingsModal, HelpModal } from '@/components/layout/Overlays'
import { useAuthStore } from '@/store/useAuthStore'
import { Button } from '@/components/ui/Button'

function AuthSplash() {
  const { restoreError, restore, logout } = useAuthStore()
  return (
    <div className="flex min-h-full flex-col items-center justify-center gap-4 bg-bg px-4 text-center">
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-surface shadow-card ring-1 ring-border/70">
        <LogoMark className={restoreError ? 'h-9 w-9' : 'h-9 w-9 animate-pulse'} />
      </div>
      <div className="space-y-1">
        <p className="text-sm font-medium text-fg">{restoreError ? '登录状态验证失败' : '正在恢复你的工作台…'}</p>
        {restoreError && <p className="max-w-xs text-xs text-muted">{restoreError}</p>}
      </div>
      {restoreError && (
        <div className="flex flex-wrap items-center justify-center gap-2">
          <Button size="sm" onClick={() => void restore()}>重试</Button>
          <Button size="sm" variant="outline" onClick={logout}>切换账号</Button>
          <Link to="/welcome"><Button size="sm" variant="ghost">返回首页</Button></Link>
        </div>
      )}
    </div>
  )
}

function RootEntry() {
  const { user, initialized, restoreError, restore, logout } = useAuthStore()
  if (user) return <HomePage />
  if (!initialized && !restoreError) return <AuthSplash />
  if (restoreError) {
    return (
      <div className="flex min-h-full flex-col">
        <div role="status" className="flex flex-wrap items-center justify-center gap-3 border-b border-warning/25 bg-warning/10 px-4 py-2 text-xs text-warning">
          <span>上次登录状态恢复失败：{restoreError}</span>
          <button onClick={() => void restore()} className="font-medium underline">重试恢复</button>
          <button onClick={logout} className="font-medium underline">清除登录态</button>
        </div>
        <div className="flex-1"><LandingPage /></div>
      </div>
    )
  }
  return <LandingPage />
}

function RequireAuth() {
  const { user, initialized } = useAuthStore()
  const location = useLocation()
  if (!initialized) return <AuthSplash />
  return user
    ? <Outlet />
    : <Navigate replace to={`/login?redirect=${encodeURIComponent(location.pathname + location.search + location.hash)}`} />
}

function AuthRoutes() {
  const restore = useAuthStore((s) => s.restore)
  useEffect(() => { void restore() }, [restore])

  return (
    <Routes>
      <Route path="/" element={<RootEntry />} />
      <Route path="/welcome" element={<LandingPage />} />
      <Route path="/login" element={<AuthPage key="login" />} />
      <Route path="/register" element={<AuthPage key="register" register />} />
      <Route element={<RequireAuth />}>
        <Route path="/admin" element={<AdminPage />} />
        <Route path="/notebook/:id" element={<NotebookPage />} />
        <Route path="*" element={<NotFound />} />
      </Route>
      <Route path="*" element={<NotFound />} />
    </Routes>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthRoutes />
      <Toaster />
      <ConfirmHost />
      <SettingsModal />
      <HelpModal />
    </BrowserRouter>
  )
}
