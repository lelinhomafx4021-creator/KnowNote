import { CheckCircle2, AlertCircle, Info } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useUIStore } from '@/store/useUIStore'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Progress, Segmented } from '@/components/ui/misc'
import { settingsApi } from '@/api'
import type { AnswerStyle, UserSettings } from '@/types'
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

const MODELS = [
  { id: 'qwen-max', name: '通义千问 Max', desc: '综合能力强，适合长文档问答' },
  { id: 'deepseek-v3', name: 'DeepSeek V3', desc: '推理能力出色，性价比高' },
  { id: 'glm-4', name: 'GLM-4', desc: '中文理解好，响应速度快' },
  { id: 'local-llama', name: '本地模型（Ollama）', desc: '数据不出本机，隐私优先' },
]

export function SettingsModal() {
  const { settingsOpen, setSettingsOpen, theme, toggleTheme } = useUIStore()
  const [s, setS] = useState<UserSettings | null>(null)
  useEffect(() => {
    if (settingsOpen) settingsApi.get().then(setS)
  }, [settingsOpen])
  const update = (patch: Partial<UserSettings>) => {
    setS((p) => (p ? { ...p, ...patch } : p))
    settingsApi.update(patch)
  }
  return (
    <Modal open={settingsOpen} onClose={() => setSettingsOpen(false)} title="设置" description="个性化你的 AI 助手与界面偏好" size="lg">
      {!s ? (
        <div className="py-10 text-center text-sm text-muted">加载中…</div>
      ) : (
        <div className="space-y-7 pb-4">
          <section>
            <h3 className="mb-3 text-sm font-semibold">对话模型</h3>
            <div className="grid gap-2 sm:grid-cols-2">
              {MODELS.map((m) => (
                <button
                  key={m.id}
                  onClick={() => update({ model: m.id })}
                  className={cn(
                    'focus-ring rounded-2xl border p-3.5 text-left transition-colors',
                    s.model === m.id ? 'border-primary bg-primary-soft/60' : 'border-border hover:bg-surface-2',
                  )}
                >
                  <div className="text-sm font-medium">{m.name}</div>
                  <div className="mt-0.5 text-xs text-muted">{m.desc}</div>
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
          <section className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold">外观</h3>
              <p className="text-xs text-muted">在浅色与深色主题之间切换</p>
            </div>
            <Segmented value={theme} onChange={(v) => v !== theme && toggleTheme()} options={[{ value: 'light', label: '浅色' }, { value: 'dark', label: '深色' }]} />
          </section>
          <section>
            <div className="mb-2 flex items-center justify-between text-sm">
              <h3 className="font-semibold">存储空间</h3>
              <span className="text-muted">{formatBytes(s.storageUsed)} / {formatBytes(s.storageTotal)}</span>
            </div>
            <Progress value={(s.storageUsed / s.storageTotal) * 100} className="h-2" />
          </section>
        </div>
      )}
    </Modal>
  )
}
