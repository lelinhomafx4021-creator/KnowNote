import { LogOut, Moon, Settings, Sun, HelpCircle, Shield, Sparkles } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/store/useAuthStore'
import { Dropdown } from '@/components/ui/Dropdown'
import { IconButton } from '@/components/ui/Button'
import { useUIStore } from '@/store/useUIStore'

export function ThemeToggle() {
  const { theme, toggleTheme } = useUIStore()
  return (
    <IconButton label={theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'} onClick={toggleTheme}>
      {theme === 'dark' ? <Sun /> : <Moon />}
    </IconButton>
  )
}

export function UserMenu() {
  const { user, logout } = useAuthStore()
  const navigate = useNavigate()
  const setSettingsOpen = useUIStore((s) => s.setSettingsOpen)
  const setHelpOpen = useUIStore((s) => s.setHelpOpen)
  const initial = Array.from(user?.nickname || user?.email || '知')[0].toUpperCase()

  return (
    <Dropdown
      trigger={({ toggle }) => (
        <button
          onClick={toggle}
          className="focus-ring ml-1 flex h-9 w-9 items-center justify-center rounded-full bg-gradient-to-br from-[#6366f1] to-[#0b57d0] text-sm font-semibold text-white shadow-sm ring-2 ring-surface transition-transform hover:scale-105"
          aria-label="账户菜单"
        >
          {initial}
        </button>
      )}
      header={
        <div className="flex items-center gap-2.5">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-[#6366f1] to-[#0b57d0] text-xs font-semibold text-white">
            {initial}
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <p className="truncate text-sm font-semibold text-fg">{user?.nickname || '用户'}</p>
              {user?.role === 'admin' && (
                <span className="shrink-0 rounded-full bg-primary-soft px-1.5 py-0.5 text-[10px] font-medium text-primary">
                  管理员
                </span>
              )}
            </div>
            <p className="truncate text-xs text-muted">{user?.email}</p>
          </div>
        </div>
      }
      items={[
        ...(user?.role === 'admin' ? [{ label: '管理后台', icon: <Shield />, onClick: () => navigate('/admin') }] : []),
        { label: '偏好设置', icon: <Settings />, onClick: () => setSettingsOpen(true) },
        { label: '产品介绍页', icon: <Sparkles />, onClick: () => navigate('/welcome') },
        { label: '快捷键与帮助', icon: <HelpCircle />, onClick: () => setHelpOpen(true) },
        { label: '退出登录', icon: <LogOut />, onClick: logout, divider: true, danger: true },
      ]}
    />
  )
}
