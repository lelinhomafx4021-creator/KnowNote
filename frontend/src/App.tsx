import { useEffect } from 'react'
import { BrowserRouter, Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom'
import HomePage from '@/pages/HomePage'
import NotebookPage from '@/pages/NotebookPage'
import AuthPage from '@/pages/AuthPage'
import AdminPage from '@/pages/AdminPage'
import NotFound from '@/pages/NotFound'
import { Toaster, ConfirmHost, SettingsModal } from '@/components/layout/Overlays'
import { useAuthStore } from '@/store/useAuthStore'
import { Button } from '@/components/ui/Button'

function RequireAuth() {
  const user = useAuthStore((s) => s.user)
  const location = useLocation()
  return user ? <Outlet /> : <Navigate replace to={`/login?redirect=${encodeURIComponent(location.pathname + location.search + location.hash)}`} />
}
function AuthRoutes() {
  const { initialized, restoreError, restore, logout } = useAuthStore()
  useEffect(() => { void restore() }, [restore])
  if (!initialized) return <div className="flex min-h-full flex-col items-center justify-center gap-4 text-sm text-muted"><p>{restoreError || '正在恢复登录状态…'}</p>{restoreError && <div className="flex gap-2"><Button onClick={() => void restore()}>重试</Button><Button variant="outline" onClick={logout}>重新登录</Button></div>}</div>
  return <Routes>
    <Route path="/login" element={<AuthPage key="login" />} />
    <Route path="/register" element={<AuthPage key="register" register />} />
    <Route element={<RequireAuth />}>
      <Route path="/" element={<HomePage />} />
      <Route path="/admin" element={<AdminPage />} />
      <Route path="/notebook/:id" element={<NotebookPage />} />
      <Route path="*" element={<NotFound />} />
    </Route>
  </Routes>
}
export default function App() {
  return <BrowserRouter><AuthRoutes /><Toaster /><ConfirmHost /><SettingsModal /></BrowserRouter>
}
