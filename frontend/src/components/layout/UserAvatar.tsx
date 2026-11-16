import { useState } from 'react'
import { useAuthStore } from '@/store/useAuthStore'
import { avatarImageUrl } from '@/api/avatar'

export function UserAvatar() {
  const { user, token, avatarVersion } = useAuthStore()
  const [failedUrl, setFailedUrl] = useState('')
  const url = avatarImageUrl(user?.avatarUrl, token, avatarVersion)
  const initial = Array.from(user?.nickname || user?.email || '知')[0].toUpperCase()

  return url && failedUrl !== url ? (
    <img
      src={url}
      alt="用户头像"
      className="h-full w-full rounded-full object-cover"
      referrerPolicy="no-referrer"
      onError={() => setFailedUrl(url)}
    />
  ) : <>{initial}</>
}
