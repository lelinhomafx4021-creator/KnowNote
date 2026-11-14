import { CheckCircle2, AlertCircle, Info } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useUIStore } from '@/store/useUIStore'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Progress, Segmented } from '@/components/ui/misc'
import { reportError } from '@/api/client'
import { useAuthStore } from '@/store/useAuthStore'
import { settingsApi, modelsApi } from '@/api'
import type { AnswerStyle, UserSettings, ModelCatalog, DailyUsage } from '@/types'
import { cn, formatBytes } from '@/lib/utils'

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

export function SettingsModal() {
  const { settingsOpen, setSettingsOpen, theme, toggleTheme } = useUIStore()
  const userId = useAuthStore((state) => state.user?.id)
  const [s, setS] = useState<UserSettings | null>(null)
  const [models, setModels] = useState<ModelCatalog | null>(null)
  const [usage, setUsage] = useState<DailyUsage | null>(null)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const savingRef = useRef(false)
  const generation = useRef(0)
  const [retry, setRetry] = useState(0)
  useEffect(() => {
    const version = ++generation.current
    setS(null); setModels(null); setUsage(null); setError(''); setSaving(false); savingRef.current = false
    if (settingsOpen && userId) void Promise.all([settingsApi.get(), modelsApi.get(), modelsApi.usage()]).then(([settings, catalog, daily]) => { if (generation.current === version) { setS(settings); setModels(catalog); setUsage(daily) } }).catch((e) => { if (generation.current === version) setError(reportError(e).message) })
    return () => { generation.current++ }
  }, [settingsOpen, userId, retry])
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
    <Modal open={settingsOpen} onClose={() => setSettingsOpen(false)} title="设置" description="个性化你的 AI 助手与界面偏好" size="lg">
      {!s ? (
        <div className="py-10 text-center text-sm text-muted">{error || '加载中…'}{error && <Button className="ml-3" size="sm" onClick={() => setRetry((n) => n + 1)}>重试</Button>}</div>
      ) : (
        <fieldset disabled={saving} className="space-y-7 pb-4 disabled:opacity-70">
          {error && <p role="alert" className="text-sm text-danger">{error}</p>}
          {saving && <p role="status" className="text-xs text-primary">正在保存设置…</p>}
          <section>
            <h3 className="mb-3 text-sm font-semibold">对话模型</h3>
            <div className="grid gap-2 sm:grid-cols-2">
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
            <p className="mt-2 text-xs text-muted">学习引导模式下，AI 会通过提问帮助你自己找到答案。</p>
          </section>
          <section>
            <h3 className="mb-3 text-sm font-semibold">回答语言</h3>
            <Segmented value={s.language} onChange={(language) => void update({ language })} options={[{ value: 'zh-CN', label: '中文' }, { value: 'en-US', label: '英语' }]} />
          </section>
          <section className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold">外观</h3>
              <p className="text-xs text-muted">在浅色与深色主题之间切换</p>
            </div>
            <Segmented value={theme} onChange={(v) => v !== theme && toggleTheme()} options={[{ value: 'light', label: '浅色' }, { value: 'dark', label: '深色' }]} />
          </section>
          {usage && <section className="space-y-3">
            <h3 className="text-sm font-semibold">今日用量 <span className="font-normal text-muted">（{usage.date}）</span></h3>
            {[
              { label: '对话请求', used: usage.chatRequests, limit: usage.chatRequestsLimit },
              { label: 'Token', used: usage.tokens, limit: usage.tokensLimit },
            ].map((item) => <div key={item.label}>
              <div className="mb-2 flex items-center justify-between gap-2 text-sm">
                <span>{item.label}</span><span className="text-muted">{item.used.toLocaleString()} / {item.limit === 0 ? '不限' : item.limit.toLocaleString()}</span>
              </div>
              <Progress value={item.limit > 0 ? Math.min(100, item.used / item.limit * 100) : 0} className="h-2" />
            </div>)}
            <p className="text-xs text-muted">按北京时间每日重置；摘要与工作室生成也计入对话请求，所有模型能力计入 Token。{usage.reservedTokens > 0 ? `当前处理中预留 ${usage.reservedTokens.toLocaleString()} Token。` : ''}</p>
          </section>}
          <section>
            <div className="mb-2 flex items-center justify-between text-sm">
              <h3 className="font-semibold">存储空间</h3>
              <span className="text-muted">{formatBytes(s.storageUsed)} / {formatBytes(s.storageTotal)}</span>
            </div>
            <Progress value={s.storageTotal > 0 ? (s.storageUsed / s.storageTotal) * 100 : 0} className="h-2" />
          </section>
        </fieldset>
      )}
    </Modal>
  )
}
