import { useState, type FormEvent } from 'react'
import { Link, Navigate, useSearchParams } from 'react-router-dom'
import { FileStack, Quote, Sparkles, AlertCircle, Eye, EyeOff } from 'lucide-react'
import { authApi } from '@/api/auth'
import { USE_MOCK } from '@/api/client'
import { useAuthStore } from '@/store/useAuthStore'
import { LogoMark } from '@/components/layout/Logo'
import { ThemeToggle } from '@/components/layout/UserMenu'
import { Button, IconButton } from '@/components/ui/Button'

function safeRedirect(value: string | null) {
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) return '/'
  const path = new URL(value, window.location.origin)
  return path.origin === window.location.origin && !/^\/(login|register)(\/|$)/.test(path.pathname) ? path.pathname + path.search + path.hash : '/'
}

export default function AuthPage({ register = false }: { register?: boolean }) {
  const [params] = useSearchParams()
  const { user, accept } = useAuthStore()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [nickname, setNickname] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [destination, setDestination] = useState('/')
  const redirect = safeRedirect(params.get('redirect'))
  if (user) return <Navigate to={destination} replace />

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (loading) return
    if (!email.trim() || !password || (register && !nickname.trim())) { setError('请填写完整的账户信息'); return }
    if (!USE_MOCK && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) { setError('请输入有效的邮箱地址'); return }
    if (!USE_MOCK && register && password.length < 6) { setError('密码至少需要 6 位'); return }
    setError('')
    setLoading(true)
    try {
      const input = { email: email.trim(), password }
      const result = register ? await authApi.register({ ...input, nickname: nickname.trim() }) : await authApi.login(input)
      setDestination(redirect)
      accept(result)
    } catch (e) { setError((e as Error).message) }
    finally { setLoading(false) }
  }
  const footerLink = `${register ? '/login' : '/register'}?redirect=${encodeURIComponent(redirect)}`

  return (
    <main className="flex min-h-full flex-col items-center justify-start bg-surface px-0 py-0 sm:justify-center sm:bg-bg sm:px-6 sm:py-10">
      <div className="w-full max-w-[1040px] bg-surface px-6 py-9 sm:rounded-[28px] sm:px-10 sm:py-10 md:px-12">
        <div className="grid gap-x-12 gap-y-8 md:grid-cols-2">
          {/* 左侧：品牌与标题 */}
          <div>
            <LogoMark className="h-12 w-12" />
            <h1 className="mt-6 text-[32px] font-normal leading-tight tracking-tight sm:text-[36px]">{register ? '创建账户' : '登录'}</h1>
            <p className="mt-4 text-base text-fg/90">{register ? '只需一分钟，开始整理你的知识库' : '继续使用知页 KnowNote'}</p>
            <ul className="mt-10 hidden space-y-5 md:block">
              {[
                [FileStack, '汇集多种资料', 'PDF、Word、网页与文本，一处管理'],
                [Quote, '有据可查的回答', '每个观点都标注出处，可回到原文'],
                [Sparkles, '自动整理笔记', '一键生成学习指南、思维导图与音频对谈'],
              ].map(([Icon, title, description]) => {
                const I = Icon as typeof Sparkles
                return (
                  <li key={String(title)} className="flex items-start gap-4">
                    <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary-soft text-primary dark:text-fg"><I className="h-5 w-5" /></span>
                    <div>
                      <p className="text-[15px] font-medium">{String(title)}</p>
                      <p className="mt-0.5 text-sm text-muted">{String(description)}</p>
                    </div>
                  </li>
                )
              })}
            </ul>
          </div>

          {/* 右侧：表单 */}
          <form onSubmit={submit} noValidate className="flex flex-col md:pt-[72px]">
            {USE_MOCK && <p className="mb-6 rounded-xl bg-primary-soft/60 px-4 py-3 text-[13px] leading-5 text-primary dark:text-fg">当前为离线演示模式，填写任意账号与密码即可体验。</p>}
            <div className="space-y-5">
              {register && <Field id="nickname" label="昵称" value={nickname} onChange={setNickname} disabled={loading} autoComplete="nickname" autoFocus maxLength={40} />}
              <Field id="email" label={USE_MOCK ? '账号或邮箱' : '邮箱地址'} type={USE_MOCK ? 'text' : 'email'} value={email} onChange={setEmail} disabled={loading} autoComplete="username" autoFocus={!register} />
              <Field
                id="password"
                label={register ? '设置密码（至少 6 位）' : '密码'}
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={setPassword}
                disabled={loading}
                autoComplete={register ? 'new-password' : 'current-password'}
                trailing={<IconButton type="button" label={showPassword ? '隐藏密码' : '显示密码'} onClick={() => setShowPassword(!showPassword)}>{showPassword ? <EyeOff /> : <Eye />}</IconButton>}
              />
            </div>
            {error && <p role="alert" className="mt-4 flex items-start gap-2 text-sm text-danger"><AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />{error}</p>}
            <p className="mt-6 text-sm text-muted">登录即表示你同意本系统仅用于学习与演示用途。</p>
            <div className="mt-10 flex items-center justify-end gap-2">
              <Link to={footerLink} className="focus-ring rounded-full px-4 py-2.5 text-sm font-medium text-primary transition-colors hover:bg-primary-soft/50">
                {register ? '已有账户？登录' : '创建账户'}
              </Link>
              <Button type="submit" loading={loading} className="h-10 min-w-[96px] px-6">
                {loading ? (register ? '创建中…' : '登录中…') : register ? '创建' : '下一步'}
              </Button>
            </div>
          </form>
        </div>
      </div>
      <div className="flex w-full max-w-[1040px] items-center justify-between px-6 py-4 text-xs text-muted sm:px-2">
        <span>简体中文</span>
        <div className="flex items-center gap-1">
          <span className="px-2">知页 KnowNote · 毕业设计</span>
          <ThemeToggle />
        </div>
      </div>
    </main>
  )
}

function Field({ id, label, value, onChange, type = 'text', disabled, autoComplete, autoFocus, maxLength, trailing }: {
  id: string; label: string; value: string; onChange: (v: string) => void; type?: string; disabled?: boolean
  autoComplete?: string; autoFocus?: boolean; maxLength?: number; trailing?: React.ReactNode
}) {
  return (
    <div className="relative">
      <input
        id={id}
        name={id}
        type={type}
        value={value}
        placeholder=" "
        disabled={disabled}
        autoComplete={autoComplete}
        autoFocus={autoFocus}
        maxLength={maxLength}
        onChange={(e) => onChange(e.target.value)}
        className={`peer h-14 w-full rounded-[4px] border border-subtle/70 bg-transparent px-4 pt-1 text-base outline-none transition-colors hover:border-fg focus:border-2 focus:border-primary focus:px-[15px] disabled:opacity-60 ${trailing ? 'pr-14' : ''}`}
      />
      <label
        htmlFor={id}
        className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 bg-surface px-1 text-base text-muted transition-all peer-focus:top-0 peer-focus:text-xs peer-focus:text-primary peer-[:not(:placeholder-shown)]:top-0 peer-[:not(:placeholder-shown)]:text-xs"
      >
        {label}
      </label>
      {trailing && <span className="absolute right-1.5 top-1/2 -translate-y-1/2">{trailing}</span>}
    </div>
  )
}
