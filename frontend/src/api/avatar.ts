import type { User } from '@/types'
import { BASE_URL, USE_MOCK, http } from './client'
import { deleteMockAvatar, uploadMockAvatar } from './mock/avatars'
import { sleep } from '@/lib/utils'

export const avatarApi = {
  upload: async (file: File, user: User): Promise<User> => {
    if (USE_MOCK) {
      await sleep(250)
      return uploadMockAvatar(user, file)
    }
    const form = new FormData()
    form.append('avatar', file)
    return http.post('/me/avatar', form)
  },
  remove: async (user: User): Promise<User> => {
    if (USE_MOCK) {
      await sleep(150)
      return deleteMockAvatar(user)
    }
    return http.del<User>('/me/avatar')
  },
}

/** 沿用文件预览的查询参数鉴权方式，版本参数用于覆盖上传后立即刷新。 */
export function avatarImageUrl(avatarUrl: string | undefined, token: string | null, version: number): string {
  if (!avatarUrl) return ''
  if (USE_MOCK) return avatarUrl
  const path = avatarUrl.replace(/^\/api(?=\/)/, '')
  const url = new URL(`${BASE_URL}${path}`, window.location.origin)
  if (token) url.searchParams.set('token', token)
  url.searchParams.set('v', String(version))
  return url.toString()
}
