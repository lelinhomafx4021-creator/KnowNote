import type { User } from '@/types'

const avatars = new Map<string, string>()

export function withMockAvatar(user: User): User {
  return avatars.has(user.id) ? { ...user, avatarUrl: avatars.get(user.id) || undefined } : user
}

export function uploadMockAvatar(user: User, file: File): User {
  const previous = avatars.get(user.id)
  const next = URL.createObjectURL(file)
  avatars.set(user.id, next)
  if (previous) URL.revokeObjectURL(previous)
  return withMockAvatar(user)
}

export function deleteMockAvatar(user: User): User {
  const previous = avatars.get(user.id)
  if (previous) URL.revokeObjectURL(previous)
  avatars.set(user.id, '')
  return withMockAvatar(user)
}
