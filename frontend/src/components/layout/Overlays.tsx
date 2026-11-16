import { CheckCircle2, AlertCircle, Info, SlidersHorizontal, BarChart3, ShieldCheck, Keyboard, Sparkles, FileText, Shield } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useUIStore } from '@/store/useUIStore'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Progress, Segmented, Kbd } from '@/components/ui/misc'
import { reportError } from '@/api/client'
import { useAuthStore } from '@/store/useAuthStore'
import { settingsApi, modelsApi, authApi } from '@/api'
import type { AnswerStyle, UserSettings, ModelCatalog, DailyUsage, LoginLog } from '@/types'
import { cn, formatBytes } from '@/lib/utils'
import { AvatarSettings } from './AvatarSettings'

export function Toaster() {
  const toasts = useUIStore((s) => s.toasts)
  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-20 z-[60] flex flex-col items-center gap-2 px-4 md:bottom-6">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={cn(
            'pointer-events-auto flex max-w-full sm:max-w-md animate-slide-up items-center gap-2.5 rounded-2xl px-4 py-3 text-sm shadow-pop',
            'bg-fg text-bg',
          )}
        >
          {t.tone === 'success' ? <CheckCircle2 className="h-4 w-4 shrink-0 text-success" /> : t.tone === 'error' ? <AlertCircle className="h-4 w-4 text-danger" /> : <Info className="h-4 w-4 opacity-70" />}
          <span className="min-w-0 truncate">{t.message}</span>
        </div>
      ))}
    </div>
  )
}

export function ConfirmHost() {
  const c = useUIStore((s) => s.confirmState)
  return (
    <Modal
      open={!!c}
      onClose={() => c?.resolve(false)}
      title={c?.title}
      size="sm"
      footer={
        <>
          <Button variant="ghost" onClick={() => c?.resolve(false)}>取消</Button>
          <Button variant={c?.danger ? 'danger' : 'primary'} onClick={() => c?.resolve(true)} autoFocus>
            {c?.confirmText ?? '确定'}
          </Button>
        </>
      }
    >
      {c?.description && <p className="pb-2 text-sm leading-6 text-muted">{c.description}</p>}
    </Modal>
  )
}

type SettingsTab = 'preferences' | 'usage' | 'security'

export function SettingsModal() {
  const { settingsOpen, setSettingsOpen, theme, toggleTheme } = useUIStore()
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.user)
  const userId = user?.id
  const isAdmin = user?.role === 'admin'
  const [tab, setTab] = useState<SettingsTab>('preferences')
  const [s, setS] = useState<UserSettings | null>(null)
  const [models, setModels] = useState<ModelCatalog | null>(null)
  const [usage, setUsage] = useState<DailyUsage | null>(null)
  const [loginLogs, setLoginLogs] = useState<LoginLog[]>([])
  const [logsError, setLogsError] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const savingRef = useRef(false)
  const generation = useRef(0)
  const [retry, setRetry] = useState(0)

  useEffect(() => {
    if (settingsOpen) setTab('preferences')
  }, [settingsOpen])

  useEffect(() => {
    const version = ++generation.current
    setS(null); setModels(null); setUsage(null); setError(''); setSaving(false); savingRef.current = false
    setLoginLogs([]); setLogsError('')
    if (settingsOpen && userId) void authApi.loginLogs().then((logs) => { if (generation.current === version) setLoginLogs(logs) }).catch((e) => { if (generation.current === version) setLogsError(reportError(e).message) })
    if (settingsOpen && userId) {
      void Promise.all([
        settingsApi.get(),
        isAdmin ? modelsApi.get() : Promise.resolve(null),
        modelsApi.usage(),
      ]).then(([settings, catalog, daily]) => {
        if (generation.current === version) {
          setS(settings); setModels(catalog); setUsage(daily)
        }
      }).catch((e) => {
        if (generation.current === version) setError(reportError(e).message)
      })
    }
    return () => { generation.current++ }
  }, [settingsOpen, userId, isAdmin, retry])

  const update = async (patch: Partial<Pick<UserSettings, 'model' | 'answerStyle' | 'language'>>) => {
    if (savingRef.current) return
    const version = generation.current
    savingRef.current = true
    setSaving(true); setError('')
    try {
      const settings = await settingsApi.update(patch)
      if (generation.current === version) setS(settings)
    } catch (e) { if (generation.current === version) setError(reportError(e).message) }
    finally { if (generation.current === version) { setSaving(false); savingRef.current = false } }
  }

  return (
    <Modal
      open={settingsOpen}
      onClose={() => setSettingsOpen(false)}
      title="设置"
      description={isAdmin ? '模型切换、个人偏好、用量查看与安全记录' : '个人阅读偏好、服务额度与安全记录'}
      size="lg"
    >
      {!s ? (
        <div className="py-10 text-center text-sm text-muted">
          {error || '加载中…'}
          {error && <Button className="ml-3" size="sm" onClick={() => setRetry((n) => n + 1)}>重试</Button>}
        </div>
      ) : (
        <div className="space-y-5 pb-4">
          <Segmented<SettingsTab>
            value={tab}
            onChange={setTab}
            className="w-full sm:w-auto"
            options={[
              { value: 'preferences', label: isAdmin ? '模型与偏好' : '阅读偏好', icon: <SlidersHorizontal /> },
              { value: 'usage', label: '用量与存储', icon: <BarChart3 /> },
              { value: 'security', label: '账号与记录', icon: <ShieldCheck /> },
            ]}
          />

          {error && <p role="alert" className="text-sm text-danger">{error}</p>}
          {saving && <p role="status" className="text-xs text-primary">正在保存设置…</p>}

          {tab === 'preferences' && (
            <fieldset disabled={saving} className="space-y-6 disabled:opacity-70">
              <AvatarSettings />
              {isAdmin && (
                <section className="space-y-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <h3 className="text-sm font-semibold">对话模型（仅管理员）</h3>
                    <button
                      type="button"
                      onClick={() => { setSettingsOpen(false); navigate('/admin') }}
                      className="inline-flex items-center gap-1 rounded-full bg-primary-soft px-3 py-1 text-xs font-medium text-primary transition hover:bg-primary/15"
                    >
                      <Shield className="h-3.5 w-3.5" /> 管理后台全局配置
                    </button>
                  </div>
                  <div className="grid gap-2.5 sm:grid-cols-2">
                    {models?.chat.map((m) => (
                      <button
                        key={m.id}
                        aria-pressed={s.model === m.id}
                        onClick={() => update({ model: m.id })}
                        className={cn(
                          'focus-ring rounded-2xl border p-3.5 text-left transition-colors',
                          s.model === m.id ? 'border-primary bg-primary-soft/60' : 'border-border hover:bg-surface-2',
                        )}
                      >
                        <div className="text-sm font-medium">{m.name}</div>
                        <div className="mt-0.5 text-xs text-muted">{m.provider === 'fake' ? '离线演示模型' : m.provider}</div>
                      </button>
                    ))}
                  </div>
                </section>
              )}

              <section>
                <h3 className="mb-3 text-sm font-semibold">回答风格</h3>
                <Segmented<AnswerStyle>
                  value={s.answerStyle}
                  onChange={(v) => update({ answerStyle: v })}
                  options={[
                    { value: 'concise', label: '简洁' },
                    { value: 'detailed', label: '详细' },
                    { value: 'learning', label: '学习引导' },
                  ]}
                />
                <p className="mt-2 text-xs text-muted">学习引导模式下，AI 会通过启发式提问帮助你自主推导答案。</p>
              </section>

              <div className="grid gap-5 sm:grid-cols-2">
                <section>
                  <h3 className="mb-3 text-sm font-semibold">回答语言</h3>
                  <Segmented value={s.language} onChange={(language) => void update({ language })} options={[{ value: 'zh-CN', label: '中文' }, { value: 'en-US', label: '英语' }]} />
                </section>
                <section>
                  <h3 className="mb-3 text-sm font-semibold">界面主题</h3>
                  <Segmented value={theme} onChange={(v) => v !== theme && toggleTheme()} options={[{ value: 'light', label: '浅色' }, { value: 'dark', label: '深色' }]} />
                </section>
              </div>
            </fieldset>
          )}

          {tab === 'usage' && (
            <div className="space-y-6">
              {usage && (
                <section className="space-y-3 rounded-2xl border border-border/70 bg-surface-2 p-4">
                  <h3 className="text-sm font-semibold">
                    {isAdmin ? '今日模型用量' : '今日服务额度'} <span className="font-normal text-muted">（{usage.date}）</span>
                  </h3>
                  {(isAdmin
                    ? [
                        { label: '对话请求', used: usage.chatRequests, limit: usage.chatRequestsLimit },
                        { label: 'Token 消耗', used: usage.tokens, limit: usage.tokensLimit },
                      ]
                    : [
                        { label: '问答与生成次数', used: usage.chatRequests, limit: usage.chatRequestsLimit },
                        { label: '每日计算额度', used: usage.tokens, limit: usage.tokensLimit },
                      ]
                  ).map((item) => (
                    <div key={item.label}>
                      <div className="mb-1.5 flex items-center justify-between gap-2 text-sm">
                        <span>{item.label}</span>
                        <span className="tabular-nums text-muted">{item.used.toLocaleString()} / {item.limit === 0 ? '不限' : item.limit.toLocaleString()}</span>
                      </div>
                      <Progress value={item.limit > 0 ? Math.min(100, item.used / item.limit * 100) : 0} className="h-2" />
                    </div>
                  ))}
                  <p className="text-xs leading-5 text-muted">
                    {isAdmin
                      ? `按北京时间每日重置；来源摘要、概要与工作室生成也计入对话请求，所有能力均计入 Token。${usage.reservedTokens > 0 ? ` 当前处理中预留 ${usage.reservedTokens.toLocaleString()} Token。` : ''}`
                      : `按北京时间每日重置；智能问答、来源整理与工作室生成共享每日额度。${usage.reservedTokens > 0 ? ` 当前处理中占用 ${usage.reservedTokens.toLocaleString()} 额度。` : ''}`}
                  </p>
                </section>
              )}

              <section className="rounded-2xl border border-border/70 bg-surface-2 p-4">
                <div className="mb-2 flex items-center justify-between text-sm">
                  <h3 className="font-semibold">存储空间</h3>
                  <span className="tabular-nums text-muted">{formatBytes(s.storageUsed)} / {formatBytes(s.storageTotal)}</span>
                </div>
                <Progress value={s.storageTotal > 0 ? (s.storageUsed / s.storageTotal) * 100 : 0} className="h-2" />
                <p className="mt-2 text-xs text-muted">包含你上传的 PDF、Office 文档、音频等原始文件体积。</p>
              </section>
            </div>
          )}

          {tab === 'security' && (
            <div className="space-y-5">
              {user && (
                <section className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-border/70 bg-surface-2 p-4">
                  <div>
                    <p className="text-sm font-semibold">{user.nickname}</p>
                    <p className="mt-0.5 text-xs text-muted">{user.email}</p>
                  </div>
                  <div className="text-right text-xs text-muted">
                    <span className="rounded-full bg-primary-soft px-2.5 py-1 font-medium text-primary">
                      {user.role === 'admin' ? '管理员账户' : '标准账户'}
                    </span>
                    <p className="mt-1.5">注册于 {new Date(user.createdAt).toLocaleDateString('zh-CN')}</p>
                  </div>
                </section>
              )}

              <section>
                <h3 className="mb-1 text-sm font-semibold">最近登录记录</h3>
                <p className="mb-3 text-xs text-muted">最近 20 次登录或注册尝试，包含当前邮箱的失败记录。</p>
                {logsError ? (
                  <p className="text-sm text-danger">
                    {logsError}
                    <button className="ml-2 underline" onClick={() => setRetry((n) => n + 1)}>重试</button>
                  </p>
                ) : loginLogs.length === 0 ? (
                  <p className="rounded-2xl border border-border/70 bg-surface-2 px-4 py-6 text-center text-sm text-muted">暂无登录记录</p>
                ) : (
                  <div className="scroll-thin max-h-64 divide-y divide-border overflow-y-auto rounded-2xl border border-border">
                    {loginLogs.map((log) => (
                      <div key={log.id} className="px-4 py-3 text-xs">
                        <div className="flex items-center justify-between gap-3">
                          <time className="tabular-nums">{new Date(log.createdAt).toLocaleString('zh-CN')}</time>
                          <span className={cn('rounded-full px-2 py-0.5 font-medium', log.success ? 'bg-success/10 text-success' : 'bg-danger/10 text-danger')}>
                            {log.success ? '成功' : '失败'}
                          </span>
                        </div>
                        <p className="mt-1 break-all text-muted">{log.ip || '未知地址'} · {log.userAgent || '未知浏览器'}</p>
                      </div>
                    ))}
                  </div>
                )}
              </section>
            </div>
          )}
        </div>
      )}
    </Modal>
  )
}

export function HelpModal() {
  const { helpOpen, setHelpOpen } = useUIStore()
  const isMac = typeof navigator !== 'undefined' && /mac|iphone|ipad/i.test(navigator.platform || navigator.userAgent)
  const mod = isMac ? '⌘' : 'Ctrl'

  return (
    <Modal open={helpOpen} onClose={() => setHelpOpen(false)} title="使用指南与快捷键" description="快速掌握知页 KnowNote 的常用操作" size="md">
      <div className="space-y-5 pb-3">
        <section className="rounded-2xl border border-border/70 bg-surface-2 p-4">
          <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold">
            <Keyboard className="h-4 w-4 text-primary" /> 常用快捷键
          </h3>
          <div className="space-y-2.5 text-sm">
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted">首页快速搜索笔记本</span>
              <Kbd>{mod} + K</Kbd>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted">发送提问消息</span>
              <Kbd>Enter</Kbd>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted">输入框内换行</span>
              <Kbd>Shift + Enter</Kbd>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted">关闭弹窗 / 退出预览</span>
              <Kbd>Esc</Kbd>
            </div>
          </div>
        </section>

        <section className="space-y-2.5 text-sm">
          <h3 className="flex items-center gap-2 font-semibold">
            <Sparkles className="h-4 w-4 text-primary" /> 高效研读建议
          </h3>
          <ul className="space-y-2 text-xs leading-6 text-muted">
            <li className="flex gap-2"><FileText className="mt-1 h-3.5 w-3.5 shrink-0 text-primary" /><span>左侧来源列表支持自由勾选范围，AI 仅在已勾选的资料内检索回答。</span></li>
            <li className="flex gap-2"><FileText className="mt-1 h-3.5 w-3.5 shrink-0 text-primary" /><span>回答中的数字角标 <strong className="text-primary">[1]</strong> 可悬停查看原文片段，点击后直接在左侧定位并跳转到原件页码。</span></li>
            <li className="flex gap-2"><FileText className="mt-1 h-3.5 w-3.5 shrink-0 text-primary" /><span>在右侧工作室生成学习指南或手写笔记后，可点击「转换为来源」反向参与后续对话检索。</span></li>
          </ul>
        </section>
      </div>
    </Modal>
  )
}
