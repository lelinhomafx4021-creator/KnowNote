import { useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { avatarApi } from '@/api/avatar'
import { reportError } from '@/api/client'
import { Button } from '@/components/ui/Button'
import { useAuthStore } from '@/store/useAuthStore'
import { toast } from '@/store/useUIStore'
import { UserAvatar } from './UserAvatar'

export function AvatarSettings() {
  const { user, token, updateUser } = useAuthStore()
  const input = useRef<HTMLInputElement>(null)
  const busy = useRef(false)
  const [pending, setPending] = useState<'upload' | 'remove' | null>(null)

  const changeAvatar = async (file?: File) => {
    if (!user || busy.current) return
    if (file && !['image/png', 'image/jpeg'].includes(file.type)) {
      toast('请上传 PNG 或 JPEG 图片', 'error')
      return
    }
    if (file && file.size > 2 * 1024 * 1024) {
      toast('头像不能超过 2MB', 'error')
      return
    }
    busy.current = true
    setPending(file ? 'upload' : 'remove')
    try {
      const updated = file ? await avatarApi.upload(file, user) : await avatarApi.remove(user)
      if (useAuthStore.getState().token === token) {
        updateUser(updated)
        toast(file ? '头像已更新' : '头像已移除', 'success')
      }
    } catch (error) { reportError(error) }
    finally { busy.current = false; setPending(null) }
  }

  return (
    <section className="space-y-3">
      <h3 className="text-sm font-semibold">个人头像</h3>
      <div className="flex items-center gap-4">
        <button
          type="button"
          onClick={() => input.current?.click()}
          disabled={!!pending}
          aria-label="更换头像"
          aria-busy={!!pending}
          className="focus-ring relative flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-[#6366f1] to-[#0b57d0] text-xl font-semibold text-white disabled:opacity-60"
        >
          <UserAvatar />
          {pending && <span className="absolute inset-0 flex items-center justify-center bg-black/35"><Loader2 className="h-6 w-6 animate-spin" /></span>}
        </button>
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" disabled={!!pending} loading={pending === 'upload'} onClick={() => input.current?.click()}>
              {pending === 'upload' ? '上传中…' : '更换头像'}
            </Button>
            {user?.avatarUrl && <Button size="sm" variant="ghost" disabled={!!pending} loading={pending === 'remove'} onClick={() => void changeAvatar()}>移除头像</Button>}
          </div>
          <p className="text-xs text-muted">支持 PNG、JPEG，最大 2MB，自动居中裁成正方形。</p>
          {pending && <p role="status" className="text-xs text-primary">{pending === 'upload' ? '正在上传头像…' : '正在移除头像…'}</p>}
        </div>
      </div>
      <input
        ref={input}
        type="file"
        accept="image/png,image/jpeg"
        aria-label="选择头像图片"
        className="hidden"
        disabled={!!pending}
        onChange={(event) => {
          const file = event.currentTarget.files?.[0]
          event.currentTarget.value = ''
          if (file) void changeAvatar(file)
        }}
      />
    </section>
  )
}
