import { http, USE_MOCK } from './client'
import type { AdminSettings, AdminUsage, AdminUserPage, AuditLog, ConnectionTest, ReindexProgress, SettingKey, User } from '@/types'
import { sleep } from '@/lib/utils'

type Patch = Partial<Record<SettingKey, string>>
const labels = ['CHAT_BASE_URL', 'CHAT_API_KEY', 'CHAT_MODEL', 'CHAT_MODELS', 'EMBED_BASE_URL', 'EMBED_API_KEY', 'EMBED_MODEL', 'QUOTA_DAILY_CHAT_REQUESTS', 'QUOTA_DAILY_TOKENS'] as SettingKey[]
const defaults = ['https://api.deepseek.com', 'sk-****demo', 'deepseek-flash', 'deepseek-flash,deepseek-v4-pro', 'https://dashscope.aliyuncs.com/compatible-mode/v1', 'sk-****demo', 'text-embedding-v4', '200', '200000']
let demoSettings: AdminSettings = { chat: [], embed: [], quota: [], reindexRequired: false, message: '' }
labels.forEach((key, i) => demoSettings[key.startsWith('CHAT_') ? 'chat' : key.startsWith('EMBED_') ? 'embed' : 'quota'].push({ key, value: defaults[i], source: 'env' }))
const demoUsers: AdminUserPage = { items: [
  { id: 'demo-user', role: 'admin', email: 'demo@knownote.local', nickname: '知页体验者', createdAt: new Date().toISOString(), disabled: false, notebookCount: 4, todayRequests: 12, todayTokens: 8400, weekTokens: 42000 },
  { id: 'demo-reader', role: 'user', email: 'reader@knownote.local', nickname: '阅读者', createdAt: new Date().toISOString(), disabled: false, notebookCount: 2, todayRequests: 6, todayTokens: 3600, weekTokens: 19000 },
], total: 2, page: 1, pageSize: 20 }
const demoLogs: AuditLog[] = []
let demoProgress: ReindexProgress = { total: 0, completed: 0, failed: 0, running: false, message: '' }
const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v))
const log = (action: string, target: string, detail: string) => demoLogs.unshift({ id: String(Date.now()), userId: 'demo-user', action, target, detail, createdAt: new Date().toISOString() })
const demoTest = async (values: Patch): Promise<ConnectionTest> => {
  await sleep(400)
  const valid = Object.entries(values).every(([k, v]) => !k.endsWith('BASE_URL') || !v || v === '__CLEAR__' || /^https?:\/\/[^\s]+$/.test(v))
  return { ok: valid, latencyMs: 400, message: valid ? '离线演示：连接测试通过（未请求真实服务）' : '模型地址无效' }
}
export const adminApi = {
  settings: (): Promise<AdminSettings> => USE_MOCK ? Promise.resolve(copy(demoSettings)) : http.get('/admin/settings'),
  test: (values: Patch, capability?: 'chat' | 'embed'): Promise<ConnectionTest> => USE_MOCK ? demoTest(values) : http.post('/admin/settings/test', { values, capability }),
  save: async (values: Patch): Promise<AdminSettings> => {
    if (!USE_MOCK) return http.patch('/admin/settings', { values })
    const test = await demoTest(values); if (!test.ok) throw new Error(test.message)
    demoSettings = copy(demoSettings)
    Object.entries(values).forEach(([key, value]) => {
      if (!value) return
      const item = [...demoSettings.chat, ...demoSettings.embed, ...demoSettings.quota].find((v) => v.key === key)!
      if (['EMBED_MODEL', 'EMBED_BASE_URL'].includes(key) && item.value !== value) { demoSettings.reindexRequired = true; demoSettings.message = '向量模型已更换，旧资料需要重建向量才能被准确检索' }
      item.source = value === '__CLEAR__' ? 'env' : 'database'
      item.value = value === '__CLEAR__' ? defaults[labels.indexOf(key as SettingKey)] : key.endsWith('API_KEY') ? (value.length > 7 ? value.slice(0, 3) + '****' + value.slice(-4) : '****') : value
    })
    log('修改配置', 'system_settings', '更新配置项：' + Object.keys(values).join('、'))
    return copy(demoSettings)
  },
  users: (query = '', page = 1): Promise<AdminUserPage> => {
    if (!USE_MOCK) return http.get(`/admin/users?query=${encodeURIComponent(query)}&page=${page}`)
    const items = demoUsers.items.filter((u) => (u.email + u.nickname).includes(query))
    return Promise.resolve(copy({ ...demoUsers, items: page === 1 ? items : [], total: items.length, page }))
  },
  updateUser: async (id: string, patch: { disabled?: boolean; role?: User['role'] }): Promise<void> => {
    if (!USE_MOCK) return http.patch(`/admin/users/${id}`, patch)
    if (id === 'demo-user' && (patch.disabled || patch.role === 'user')) throw new Error('不能停用自己或撤销自己的管理员权限')
    const user = demoUsers.items.find((u) => u.id === id)!
    Object.assign(user, patch); log(patch.role ? '变更角色' : patch.disabled ? '停用用户' : '启用用户', id, '账号权限已更新')
  },
  usage: (): Promise<AdminUsage> => USE_MOCK ? Promise.resolve({
    days: Array.from({ length: 7 }, (_, i) => { const date = new Date(); date.setDate(date.getDate() - 6 + i); return { date: date.toLocaleDateString('sv-SE'), requests: 8 + i * 3, tokens: 2500 + i * 1700 } }),
    topUsers: demoUsers.items.map((u) => ({ id: u.id, email: u.email, nickname: u.nickname, requests: u.todayRequests, tokens: u.todayTokens })),
  }) : http.get('/admin/usage'),
  logs: (): Promise<AuditLog[]> => USE_MOCK ? Promise.resolve(copy(demoLogs.slice(0, 50))) : http.get('/admin/audit-logs'),
  reindex: (): Promise<ReindexProgress> => USE_MOCK ? Promise.resolve(copy(demoProgress)) : http.get('/admin/reindex'),
  startReindex: async (): Promise<ReindexProgress> => {
    if (!USE_MOCK) return http.post('/admin/reindex')
    if (demoProgress.running) throw new Error('已有向量重建任务正在运行')
    demoProgress = { total: 20, completed: 0, failed: 0, running: true, message: '演示重建中' }
    log('启动重建', 'chunks', '使用当前向量配置重建全部分块')
    const timer = setInterval(() => { demoProgress.completed += 5; if (demoProgress.completed >= 20) { clearInterval(timer); demoProgress.running = false; demoProgress.message = '全部向量已重建'; demoSettings.reindexRequired = false; demoSettings.message = '' } }, 600)
    return copy(demoProgress)
  },
}
