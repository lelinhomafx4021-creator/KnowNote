import { LogOut, Moon, Settings, Sun, HelpCircle } from 'lucide-react'
import { Dropdown } from '@/components/ui/Dropdown'
import { IconButton } from '@/components/ui/Button'
import { useUIStore, toast } from '@/store/useUIStore'

export function ThemeToggle() {
  const { theme, toggleTheme } = useUIStore()
  return (
    <IconButton label={theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'} onClick={toggleTheme}>
      {theme === 'dark' ? <Sun /> : <Moon />}
    </IconButton>
  )
}

export function UserMenu() {
  const setSettingsOpen = useUIStore((s) => s.setSettingsOpen)
  return (
    <Dropdown
      trigger={({ toggle }) => (
        <button onClick={toggle} className="focus-ring ml-1 flex h-9 w-9 items-center justify-center rounded-full bg-gradient-to-br from-amber-400 to-rose-500 text-sm font-semibold text-white ring-2 ring-surface" aria-label="账户菜单">
          杨
        </button>
      )}
      items={[
        { label: '设置', icon: <Settings />, onClick: () => setSettingsOpen(true) },
        { label: '帮助与反馈', icon: <HelpCircle />, onClick: () => toast('帮助文档即将上线') },
        { label: '退出登录', icon: <LogOut />, onClick: () => toast('已退出（演示）'), divider: true, danger: true },
      ]}
    />
  )
}
