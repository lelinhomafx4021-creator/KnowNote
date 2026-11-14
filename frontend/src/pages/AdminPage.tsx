import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowLeft, KeyRound, Users, BarChart3, History, ShieldCheck, RefreshCw, Search } from 'lucide-react'
import { adminApi } from '@/api/admin'
import { authApi } from '@/api/auth'
import { ApiError, USE_MOCK } from '@/api/client'
import { Logo } from '@/components/layout/Logo'
import { ThemeToggle, UserMenu } from '@/components/layout/UserMenu'
import { Button } from '@/components/ui/Button'
import { useAuthStore } from '@/store/useAuthStore'
import { confirmDialog, toast } from '@/store/useUIStore'
import type { AdminSetting, AdminSettings, AdminUsage, AdminUser, AdminUserPage, AuditLog, ReindexProgress, SettingKey } from '@/types'

const inputStyle = 'focus-ring w-full rounded-xl border border-border bg-surface px-3 py-2.5 text-sm'
const names: Record<SettingKey, string> = { CHAT_BASE_URL: '服务地址', CHAT_API_KEY: 'API 密钥', CHAT_MODEL: '默认模型', CHAT_MODELS: '可选模型（逗号分隔）', EMBED_BASE_URL: '服务地址', EMBED_API_KEY: 'API 密钥', EMBED_MODEL: '向量模型', QUOTA_DAILY_CHAT_REQUESTS: '每日对话请求数', QUOTA_DAILY_TOKENS: '每日 Token 数' }
const origins = { database: '数据库', env: '环境变量', default: '默认' }
const tabs = [{ id: 'models', label: '模型与密钥', icon: KeyRound }, { id: 'users', label: '用户管理', icon: Users }, { id: 'usage', label: '用量', icon: BarChart3 }, { id: 'logs', label: '操作记录', icon: History }] as const
const fmt = (n: number) => n.toLocaleString('zh-CN')
const date = (v: string) => new Date(v).toLocaleString('zh-CN')
function Card({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return <section className="rounded-3xl border border-border/60 bg-surface p-5 sm:p-6"><h2 className="text-lg font-semibold">{title}</h2>{description && <p className="mt-1 text-sm text-muted">{description}</p>}<div className="mt-5">{children}</div></section>
}
export default function AdminPage() {
  const navigate = useNavigate()
  const { user, token } = useAuthStore()
  const [verified, setVerified] = useState(false)
  const [verifyError, setVerifyError] = useState(false)
  const [tab, setTab] = useState<(typeof tabs)[number]['id']>('models')
  const fail = useCallback((e: unknown) => {
    if (e instanceof ApiError && e.status === 403) navigate('/', { replace: true })
    if (!(e instanceof ApiError && e.reported)) toast(e instanceof Error ? e.message : '操作失败，请重试', 'error')
  }, [navigate])
  const verify = useCallback(() => {
    if (!token) return
    setVerifyError(false)
    void authApi.me(token).then((current) => {
      useAuthStore.setState({ user: current })
      if (current.role !== 'admin') { toast('仅管理员可访问管理后台', 'error'); navigate('/', { replace: true }) }
      else setVerified(true)
    }).catch((e) => { fail(e); setVerifyError(true) })
  }, [token, navigate, fail])
  useEffect(verify, [verify])
  if (!verified || user?.role !== 'admin') return <div className="p-8 text-center text-muted">{verifyError ? <Button onClick={verify}>重试管理员验证</Button> : '正在验证管理员权限…'}</div>
  return <div className="min-h-full bg-bg">
    <header className="flex items-center justify-between border-b border-border/60 bg-surface px-4 py-4 sm:px-7"><Logo /><div className="flex items-center gap-1"><Link to="/" className="mr-2 flex items-center gap-1 text-sm text-muted"><ArrowLeft size={16} /><span className="hidden sm:inline">返回笔记本</span></Link><ThemeToggle /><UserMenu /></div></header>
    <div className="mx-auto flex max-w-7xl flex-col md:flex-row">
      <aside className="border-b border-border/60 p-3 md:w-56 md:shrink-0 md:border-b-0 md:p-5"><div className="mb-5 hidden items-center gap-2 px-3 text-sm font-medium text-primary md:flex"><ShieldCheck size={18} />管理后台</div><nav aria-label="管理后台导航" className="flex gap-1 overflow-x-auto md:flex-col">{tabs.map(({ id, label, icon: Icon }) => <button key={id} onClick={() => setTab(id)} aria-current={tab === id ? 'page' : undefined} className={`focus-ring flex shrink-0 items-center gap-2 rounded-full px-4 py-3 text-sm transition-colors ${tab === id ? 'bg-primary-soft font-medium text-primary' : 'text-muted hover:bg-surface-3'}`}><Icon size={17} />{label}</button>)}</nav></aside>
      <main className="min-w-0 flex-1 p-4 sm:p-7"><div className="mb-6"><h1 className="text-2xl font-semibold">{tabs.find((t) => t.id === tab)!.label}</h1><p className="mt-2 text-sm text-muted">{USE_MOCK ? '离线演示后台，修改仅保留在当前页面会话。' : '管理全站共享模型、使用额度与账号权限。'}</p></div>
        {tab === 'models' && <ModelSettings onError={fail} />}{tab === 'users' && <UserManagement onError={fail} />}{tab === 'usage' && <Usage onError={fail} />}{tab === 'logs' && <Logs onError={fail} />}
      </main>
    </div>
  </div>
}
type ErrorProps = { onError: (e: unknown) => void }
function ModelSettings({ onError }: ErrorProps) {
  const [data, setData] = useState<AdminSettings | null>(null)
  const [progress, setProgress] = useState<ReindexProgress | null>(null)
  const [loading, setLoading] = useState(true)
  const [starting, setStarting] = useState(false)
  const load = useCallback(() => { setLoading(true); void Promise.all([adminApi.settings(), adminApi.reindex()]).then(([s, p]) => { setData(s); setProgress(p) }).catch(onError).finally(() => setLoading(false)) }, [onError])
  useEffect(load, [load])
  useEffect(() => {
    if (!progress?.running) return
    let live = true
    const timer = setInterval(() => { void adminApi.reindex().then((p) => { if (live) { setProgress(p); if (!p.running) void adminApi.settings().then((s) => live && setData(s)).catch(onError) } }).catch((e) => { clearInterval(timer); onError(e) }) }, 1500)
    return () => { live = false; clearInterval(timer) }
  }, [progress?.running, onError])
  const start = async () => {
    if (!await confirmDialog({ title: '重建全部向量？', description: '将使用当前向量模型处理所有用户资料。重建期间资料读写可能等待，消耗各用户 Token 额度；失败时全部回滚。', confirmText: '开始重建' })) return
    setStarting(true)
    try { setProgress(await adminApi.startReindex()) } catch (e) { onError(e) } finally { setStarting(false) }
  }
  if (!data) return <p className="text-sm text-muted">{loading ? '正在读取配置…' : <Button onClick={load}>重新加载</Button>}</p>
  return <div className="space-y-5"><div className="grid items-start gap-5 xl:grid-cols-2"><SettingsCard title="对话模型" description="用于问答、摘要、概要与工作室。" items={data.chat} capability="chat" onSaved={setData} onError={onError} /><SettingsCard title="向量模型" description="向量维度由数据库固定，后台不可修改。" items={data.embed} capability="embed" disabled={progress?.running} onSaved={setData} onError={onError} /></div>
    <SettingsCard title="每日额度" description="按北京时间每日重置；0 表示不限。对话次数包含摘要、概要与工作室。" items={data.quota} onSaved={setData} onError={onError} />
    <Card title="向量重建" description="更换向量模型或地址后，请为旧资料重建向量。">
      {data.reindexRequired && <p role="status" className="mb-4 rounded-xl bg-primary-soft p-3 text-sm text-primary">{data.message}</p>}
      {progress && <div className="mb-4 space-y-2 text-sm"><p role="status">{progress.message || '当前没有重建任务'}{progress.total > 0 && ` · ${progress.completed}/${progress.total}，失败 ${progress.failed}`}</p>{(progress.running || progress.total > 0) && <progress aria-label="向量重建进度" className="h-2 w-full accent-primary" max={Math.max(1, progress.total)} value={progress.completed} />}</div>}
      <Button variant="outline" icon={<RefreshCw size={16} />} loading={starting} disabled={progress?.running} onClick={() => void start()}>{progress?.running ? '正在重建…' : '重建全部向量'}</Button>
    </Card>
  </div>
}
function SettingsCard({ title, description, items, capability, disabled, onSaved, onError }: ErrorProps & { title: string; description: string; items: AdminSetting[]; capability?: 'chat' | 'embed'; disabled?: boolean; onSaved: (s: AdminSettings) => void }) {
  const [draft, setDraft] = useState<Partial<Record<SettingKey, string>>>({})
  const [busy, setBusy] = useState<'test' | 'save' | null>(null)
  const [result, setResult] = useState('')
  const run = async (action: 'test' | 'save') => {
    setBusy(action); setResult('')
    try {
      if (action === 'test') { const r = await adminApi.test(draft, capability); setResult(`${r.message}（${r.latencyMs} ms）`); if (!r.ok) toast(r.message, 'error') }
      else { const s = await adminApi.save(draft); onSaved(s); setDraft({}); setResult('已保存，后续请求立即使用新配置'); toast('配置已保存并生效', 'success') }
    } catch (e) { onError(e) } finally { setBusy(null) }
  }
  return <Card title={title} description={description}><form onSubmit={(e) => { e.preventDefault(); void run('save') }} className="space-y-4">
    {items.map((item) => {
      const secret = item.key.endsWith('API_KEY'), clear = draft[item.key] === '__CLEAR__'
      return <div key={item.key}><label htmlFor={item.key} className="mb-2 flex items-center justify-between gap-2 text-sm font-medium"><span>{names[item.key]}</span><span className="rounded-md bg-surface-3 px-2 py-0.5 text-[11px] font-normal text-muted">{origins[item.source]}</span></label>
        <input id={item.key} type={secret ? 'password' : capability ? 'text' : 'number'} autoComplete={secret ? 'new-password' : 'off'} min={capability ? undefined : 0} step={capability ? undefined : 1} disabled={!!busy || disabled || clear} className={inputStyle} placeholder={secret ? (item.value || '未配置，使用离线模型') : ''} value={clear ? '' : draft[item.key] ?? (secret ? '' : item.value)} onChange={(e) => setDraft({ ...draft, [item.key]: e.target.value })} />
        {secret && <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted"><span>{clear ? '保存后删除数据库覆盖值' : '留空不修改；密钥只显示掩码'}</span><button type="button" disabled={!!busy || disabled} className="focus-ring rounded text-primary disabled:opacity-40" onClick={() => setDraft({ ...draft, [item.key]: clear ? '' : '__CLEAR__' })}>{clear ? '取消清除' : '清除，改用环境变量'}</button></div>}
      </div>
    })}
    {result && <p role="status" className="break-words text-sm text-muted">{result}</p>}
    <div className="flex flex-wrap justify-end gap-2 pt-1">{capability && <Button type="button" variant="outline" loading={busy === 'test'} disabled={!!busy || disabled} onClick={() => void run('test')}>测试连接</Button>}<Button type="submit" loading={busy === 'save'} disabled={!!busy || disabled || !Object.values(draft).some((v) => v !== '')}>{busy === 'save' && capability ? '测试中，随后保存…' : '保存'}</Button></div>
  </form></Card>
}
function UserManagement({ onError }: ErrorProps) {
  const self = useAuthStore((s) => s.user?.id)
  const [query, setQuery] = useState(''), [search, setSearch] = useState(''), [page, setPage] = useState(1)
  const [data, setData] = useState<AdminUserPage | null>(null), [busy, setBusy] = useState(false), [loading, setLoading] = useState(false)
  const load = useCallback(() => { setLoading(true); void adminApi.users(search, page).then(setData).catch(onError).finally(() => setLoading(false)) }, [search, page, onError])
  useEffect(load, [load])
  const change = async (u: AdminUser, kind: 'disabled' | 'role') => {
    const label = kind === 'disabled' ? u.disabled ? '启用' : '停用' : u.role === 'admin' ? '取消管理员' : '设为管理员'
    if (!await confirmDialog({ title: `${label}此用户？`, description: `${u.nickname} · ${u.email}。账号权限变更立即生效。`, confirmText: label, danger: label === '停用' || label === '取消管理员' })) return
    setBusy(true)
    try { await adminApi.updateUser(u.id, kind === 'disabled' ? { disabled: !u.disabled } : { role: u.role === 'admin' ? 'user' : 'admin' }); load(); toast('用户权限已更新', 'success') } catch (e) { onError(e) } finally { setBusy(false) }
  }
  const actions = (u: AdminUser) => <div className="flex flex-wrap gap-2"><Button size="sm" variant="outline" disabled={busy || u.id === self} onClick={() => void change(u, 'disabled')}>{u.disabled ? '启用' : '停用'}</Button><Button size="sm" variant="soft" disabled={busy || u.id === self} onClick={() => void change(u, 'role')}>{u.role === 'admin' ? '取消管理员' : '设为管理员'}</Button></div>
  return <div className="space-y-5"><form onSubmit={(e) => { e.preventDefault(); setPage(1); setSearch(query.trim()) }} className="flex gap-2"><div className="relative max-w-md flex-1"><Search className="absolute left-3 top-3 text-muted" size={17} /><input aria-label="搜索邮箱或昵称" className={`${inputStyle} pl-10`} placeholder="搜索邮箱或昵称" value={query} onChange={(e) => setQuery(e.target.value)} /></div><Button type="submit" loading={loading}>搜索</Button></form>
    {!data ? <Button variant="outline" onClick={load}>重新加载用户</Button> : <><div className="hidden overflow-x-auto rounded-3xl border border-border/60 bg-surface lg:block"><table className="w-full text-left text-sm"><thead className="bg-surface-2 text-muted"><tr>{['用户 / 注册时间', '角色 / 状态', '笔记本', '今日请求', '今日 Tokens', '7 天 Tokens', '操作'].map((x) => <th key={x} className="whitespace-nowrap px-4 py-4 font-medium">{x}</th>)}</tr></thead><tbody>{data.items.map((u) => <tr key={u.id} className="border-t border-border/60"><td className="px-4 py-4"><p>{u.nickname}</p><p className="text-xs text-muted">{u.email}</p><p className="mt-1 text-xs text-muted">{date(u.createdAt)}</p></td><td className="px-4 py-4"><p>{u.role === 'admin' ? '管理员' : '普通用户'}</p><span className={u.disabled ? 'text-danger' : 'text-muted'}>{u.disabled ? '已停用' : '正常'}</span></td>{[u.notebookCount, u.todayRequests, u.todayTokens, u.weekTokens].map((n, i) => <td key={i} className="px-4 py-4">{fmt(n)}</td>)}<td className="px-4 py-4">{actions(u)}</td></tr>)}</tbody></table></div>
    <div className="space-y-3 lg:hidden">{data.items.map((u) => <Card key={u.id} title={u.nickname} description={`${u.email} · ${u.role === 'admin' ? '管理员' : '普通用户'} · ${u.disabled ? '已停用' : '正常'}`}><p className="mb-3 text-xs text-muted">注册于 {date(u.createdAt)}</p><dl className="mb-4 grid grid-cols-2 gap-3 text-sm">{[['笔记本', u.notebookCount], ['今日请求', u.todayRequests], ['今日 Tokens', u.todayTokens], ['7 天 Tokens', u.weekTokens]].map(([k, n]) => <div key={k}><dt className="text-xs text-muted">{k}</dt><dd className="mt-1">{fmt(Number(n))}</dd></div>)}</dl>{actions(u)}</Card>)}</div>
    {data.items.length === 0 && <p className="py-8 text-center text-muted">没有找到用户</p>}<div className="flex items-center justify-between gap-2 text-sm text-muted"><span>共 {data.total} 位用户 · 第 {page} 页</span><div className="flex gap-2"><Button variant="outline" size="sm" disabled={page <= 1 || loading} onClick={() => setPage(page - 1)}>上一页</Button><Button variant="outline" size="sm" disabled={page * data.pageSize >= data.total || loading} onClick={() => setPage(page + 1)}>下一页</Button></div></div></>}
  </div>
}
function Usage({ onError }: ErrorProps) {
  const [data, setData] = useState<AdminUsage | null>(null)
  const load = useCallback(() => { void adminApi.usage().then(setData).catch(onError) }, [onError])
  useEffect(load, [load])
  if (!data) return <Button variant="outline" onClick={load}>加载用量</Button>
  const max = Math.max(1, ...data.days.map((d) => d.tokens))
  return <div className="space-y-5"><Card title="近 7 天用量" description="北京时间，包含各能力调用；Token 优先使用上游用量，缺失时估算。"><div className="flex h-60 items-end gap-2 sm:gap-5">{data.days.map((d) => <div key={d.date} className="flex h-full min-w-0 flex-1 flex-col items-center justify-end gap-2 text-center"><span className="text-[10px] text-muted sm:text-xs">{fmt(d.tokens)}</span><div title={`${d.date}：${d.requests} 次请求，${d.tokens} Tokens`} aria-label={`${d.date}：${d.requests} 次请求，${d.tokens} Tokens`} className="w-full max-w-14 rounded-t-lg bg-primary/80" style={{ height: `${Math.max(1, d.tokens / max * 65)}%` }} /><span className="text-[10px] sm:text-xs">{d.date.slice(5)}</span><span className="text-[10px] text-muted">{d.requests} 次</span></div>)}</div></Card>
    <Card title="今日用量 Top 10"><div className="overflow-x-auto"><table className="w-full text-left text-sm"><thead className="text-muted"><tr>{['用户', '请求数', 'Tokens'].map((x) => <th key={x} className="px-2 pb-3 font-medium">{x}</th>)}</tr></thead><tbody>{data.topUsers.map((u) => <tr key={u.id} className="border-t border-border/60"><td className="px-2 py-3">{u.nickname}<p className="text-xs text-muted">{u.email}</p></td><td className="px-2 py-3">{fmt(u.requests)}</td><td className="px-2 py-3">{fmt(u.tokens)}</td></tr>)}</tbody></table>{!data.topUsers.length && <p className="py-5 text-muted">今日暂无模型调用</p>}</div></Card></div>
}
function Logs({ onError }: ErrorProps) {
  const [data, setData] = useState<AuditLog[] | null>(null)
  const load = useCallback(() => { void adminApi.logs().then(setData).catch(onError) }, [onError])
  useEffect(load, [load])
  return <Card title="最近 50 条操作" description="仅记录操作类型与脱敏后的变更摘要。"><div className="mb-3"><Button size="sm" variant="outline" onClick={load}>刷新</Button></div>{data?.length === 0 && <p className="py-5 text-muted">暂无操作记录</p>}<ol className="divide-y divide-border/60">{data?.map((l) => <li key={l.id} className="py-4"><div className="flex flex-wrap justify-between gap-2"><span className="text-sm font-medium">{l.action}</span><time className="text-xs text-muted">{date(l.createdAt)}</time></div><p className="mt-2 break-words text-sm text-muted">{l.detail}</p><p className="mt-2 break-all text-xs text-muted">操作人：{l.userId || '已删除用户'} · 对象：{l.target}</p></li>)}</ol></Card>
}
