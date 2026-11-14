import type { User } from '@/types'
import { http, USE_MOCK } from './client'
import { sleep } from '@/lib/utils'

export interface AuthResult { token: string; user: User }
export interface LoginInput { email: string; password: string }
export interface RegisterInput extends LoginInput { nickname: string }

const demoUser: User = { id: 'demo-user', email: 'demo@knownote.local', nickname: '知页体验者', createdAt: '2026-11-14T00:00:00Z' }
async function demo(email: string, nickname?: string): Promise<AuthResult> {
  await sleep(250)
  const user = { ...demoUser, email, nickname: nickname || email.split('@')[0] || demoUser.nickname }
  return { token: `demo.${encodeURIComponent(JSON.stringify(user))}`, user }
}
export const authApi = {
  register: (input: RegisterInput): Promise<AuthResult> => USE_MOCK ? demo(input.email, input.nickname) : http.post('/auth/register', input),
  login: (input: LoginInput): Promise<AuthResult> => USE_MOCK ? demo(input.email) : http.post('/auth/login', input),
  me: async (token: string): Promise<User> => {
    if (!USE_MOCK) return http.get('/me')
    await sleep(100)
    try { return JSON.parse(decodeURIComponent(token.slice(5))) as User } catch { return demoUser }
  },
}
