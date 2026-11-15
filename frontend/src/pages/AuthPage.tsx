import { useMemo, useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom'
import {
  AlertCircle, ArrowLeft, ArrowRight, Eye, EyeOff,
  Lock, Mail, Sparkles, User as UserIcon,
} from 'lucide-react'
import { authApi } from '@/api/auth'
import { USE_MOCK } from '@/api/client'
import { useAuthStore } from '@/store/useAuthStore'
import { Logo, LogoMark } from '@/components/layout/Logo'
import { ThemeToggle } from '@/components/layout/UserMenu'
import { Button, IconButton } from '@/components/ui/Button'
import { SourceIcon } from '@/components/ui/SourceIcon'
import { cn } from '@/lib/utils'

function safeRedirect(value: string | null) {
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) return '/'
  const path = new URL(value, window.location.origin)
  return path.origin === window.location.origin && !/^\/(login|register)(\/|$)/.test(path.pathname)
    ? path.pathname + path.search + path.hash
    : '/'
}

function passwordStrength(pwd: string): { score: number; label: string; color: string } {
  if (!pwd) return { score: 0, label: '', color: '' }
  let score = 0
  if (pwd.length >= 6) score++
  if (pwd.length >= 10) score++
  if (/[A-Z]/.test(pwd) && /[a-z]/.test(pwd)) score++
  if (/\d/.test(pwd) || /[^A-Za-z0-9]/.test(pwd)) score++
  if (score <= 1) return { score: 1, label: '较弱', color: 'bg-danger' }
  if (score <= 2) return { score: 2, label: '一般', color: 'bg-warning' }
  return { score: 3, label: '较强', color: 'bg-success' }
}

export default function AuthPage({ register = false }: { register?: boolean }) {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { user, accept } = useAuthStore()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [nickname, setNickname] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [touched, setTouched] = useState<{ email?: boolean; password?: boolean; confirm?: boolean; nickname?: boolean }>({})
  const [loading, setLoading] = useState(false)
  const [demoLoading, setDemoLoading] = useState(false)
  const [error, setError] = useState('')

  const redirect = safeRedirect(params.get('redirect'))
  const strength = useMemo(() => passwordStrength(password), [password])

  if (user) return <Navigate to={redirect} replace />

  const emailInvalid = !USE_MOCK && touched.email && email.trim() !== '' && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())
  const pwdTooShort = !USE_MOCK && register && touched.password && password.length > 0 && password.length < 6
  const pwdHasEdgeSpace = password.length > 0 && password !== password.trim()
  const pwdMismatch = register && touched.confirm && confirmPassword.length > 0 && confirmPassword !== password

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (loading || demoLoading) return
    setTouched({ email: true, password: true, confirm: true, nickname: true })

    if (!email.trim() || !password || (register && !nickname.trim())) {
      setError('请填写完整的账户信息')
      return
    }
    if (!USE_MOCK && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
      setError('请输入有效的邮箱地址')
      return
    }
    if (!USE_MOCK && register && password.length < 6) {
      setError('密码至少需要 6 个字符')
      return
    }
    if (new TextEncoder().encode(password).length > 72) {
      setError('密码长度不能超过 72 字节')
      return
    }
    if (register && password !== confirmPassword) {
      setError('两次输入的密码不一致')
      return
    }

    setError('')
    setLoading(true)
    try {
      const input = { email: email.trim(), password }
      const result = register
        ? await authApi.register({ ...input, nickname: nickname.trim() })
        : await authApi.login(input)
      accept(result)
      navigate(redirect, { replace: true })
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const demoLogin = async () => {
    if (loading || demoLoading) return
    setDemoLoading(true)
    setError('')
    try {
      const result = await authApi.login({ email: 'demo@knownote.local', password: 'demo-preview' })
      accept(result)
      navigate(redirect, { replace: true })
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setDemoLoading(false)
    }
  }

  const switchQuery = redirect !== '/' ? `?redirect=${encodeURIComponent(redirect)}` : ''

  return (
    <div className="relative flex min-h-full flex-col bg-surface dark:bg-bg">
      {/* NotebookLM 通透粉彩微光背景 */}
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute -top-28 left-1/2 h-[420px] w-[780px] -translate-x-1/2 rounded-full bg-gradient-to-tr from-[#0b57d0]/12 via-[#10b981]/10 to-[#6366f1]/12 blur-[110px]" />
      </div>

      {/* 顶部栏 */}
      <header className="relative z-10 mx-auto flex h-16 w-full max-w-[1100px] items-center justify-between px-4 sm:px-8">
        <Logo to="/" />
        <div className="flex items-center gap-2">
          <Link
            to="/"
            className="focus-ring inline-flex items-center gap-1.5 rounded-full border border-border bg-surface/90 px-3.5 py-1.5 text-xs font-medium text-muted backdrop-blur transition-colors hover:text-fg sm:text-sm"
          >
            <ArrowLeft className="h-3.5 w-3.5" /> 了解知页
          </Link>
          <ThemeToggle />
        </div>
      </header>

      {/* 主容器 */}
      <main className="relative z-10 mx-auto flex w-full max-w-[1040px] flex-1 items-center justify-center px-4 py-6 sm:px-8 sm:py-10">
        <div className="grid w-full overflow-hidden rounded-[28px] border border-border bg-surface shadow-pop lg:grid-cols-[1fr_1.05fr]">
          {/* 左侧：NotebookLM 风格浅彩研究卡片预览 */}
          <aside className="hidden flex-col justify-between bg-[#f4f7fc] p-9 dark:bg-surface-2/80 lg:flex">
            <div>
              <div className="flex items-center gap-3">
                <LogoMark className="h-10 w-10" />
                <div>
                  <p className="text-sm font-semibold">知页 KnowNote</p>
                  <p className="text-xs text-muted">AI 个人资料与研究工作台</p>
                </div>
              </div>
              <h2 className="mt-7 text-[28px] font-medium leading-snug tracking-tight">
                理解一切，
                <span className="bg-gradient-to-r from-[#0b57d0] via-[#0ea5e9] to-[#10b981] bg-clip-text text-transparent">
                  更有深度
                </span>
              </h2>
              <p className="mt-3 text-sm leading-6 text-muted">
                把论文、课件、表格与网页汇入专属笔记本，享受句句带原文角标的问答与一键思维导图沉淀。
              </p>

              {/* 模拟来源与引用卡片 */}
              <div className="mt-7 space-y-2.5">
                <div className="flex items-center gap-2.5 rounded-2xl border border-border/70 bg-surface p-3 shadow-card">
                  <SourceIcon type="pdf" size="sm" />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-xs font-medium">RAG 阅读与引用指南.pdf</p>
                    <p className="text-[11px] text-subtle">PDF · 支持原件逐页定位</p>
                  </div>
                  <span className="rounded-full bg-[#e6f4ea] px-2 py-0.5 text-[11px] font-medium text-[#146c2e] dark:bg-[#223229] dark:text-[#6dd58c]">
                    已就绪
                  </span>
                </div>
                <div className="rounded-2xl border border-border/70 bg-surface p-3.5 shadow-card">
                  <div className="flex items-center gap-1.5 text-xs font-medium text-primary">
                    <Sparkles className="h-3.5 w-3.5" /> 循证回答示例
                  </div>
                  <p className="mt-1.5 text-xs leading-6 text-muted">
                    “系统按 500 字切分并保留重叠上下文，点击角标即可跳转原件具体页面核对
                    <span className="mx-1 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-primary-soft px-1 text-[10px] font-semibold text-primary">1</span>
                    。”
                  </p>
                </div>
              </div>
            </div>

            <div className="mt-8 flex flex-wrap gap-2">
              {(['pdf', 'pptx', 'xlsx', 'web', 'audio'] as const).map((type) => (
                <SourceIcon key={type} type={type} size="sm" />
              ))}
              <span className="ml-1 self-center text-xs text-muted">支持 10 种常见资料格式</span>
            </div>
          </aside>

          {/* 右侧：登录/注册表单 */}
          <section className="flex flex-col justify-center p-6 sm:p-10">
            <div className="mb-7 flex rounded-full bg-surface-3 p-1" role="tablist" aria-label="认证方式切换">
              <Link
                to={`/login${switchQuery}`}
                replace
                role="tab"
                aria-selected={!register}
                className={cn(
                  'focus-ring flex-1 rounded-full py-2 text-center text-sm font-medium transition-all',
                  !register ? 'bg-surface text-fg shadow-card' : 'text-muted hover:text-fg',
                )}
              >
                登录
              </Link>
              <Link
                to={`/register${switchQuery}`}
                replace
                role="tab"
                aria-selected={register}
                className={cn(
                  'focus-ring flex-1 rounded-full py-2 text-center text-sm font-medium transition-all',
                  register ? 'bg-surface text-fg shadow-card' : 'text-muted hover:text-fg',
                )}
              >
                创建账户
              </Link>
            </div>

            <div>
              <h1 className="text-2xl font-medium tracking-tight">{register ? '开始使用知页' : '欢迎回来'}</h1>
              <p className="mt-1.5 text-sm text-muted">
                {register ? '创建一个个人账户，管理你的资料与笔记本' : '登录你的账户，继续研读与整理'}
              </p>
            </div>

            {USE_MOCK && (
              <div className="mt-5 flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-primary/20 bg-primary-soft/45 px-4 py-3">
                <div className="min-w-0 flex-1 text-xs leading-5">
                  <p className="font-medium text-primary dark:text-fg">当前为离线演示模式</p>
                  <p className="text-muted">可直接点击按钮免密进入工作台体验。</p>
                </div>
                <Button type="button" size="sm" loading={demoLoading} onClick={() => void demoLogin()}>
                  一键体验
                </Button>
              </div>
            )}

            <form onSubmit={submit} noValidate className="mt-6 space-y-4">
              {register && (
                <AuthInput
                  id="nickname"
                  label="昵称"
                  placeholder="例如：张同学"
                  value={nickname}
                  onChange={setNickname}
                  onBlur={() => setTouched((t) => ({ ...t, nickname: true }))}
                  disabled={loading}
                  autoComplete="nickname"
                  autoFocus
                  maxLength={100}
                  icon={<UserIcon className="h-4 w-4" />}
                  hint={touched.nickname && !nickname.trim() ? '请输入昵称' : undefined}
                  invalid={touched.nickname && !nickname.trim()}
                />
              )}

              <AuthInput
                id="email"
                label={USE_MOCK ? '账号或邮箱' : '邮箱地址'}
                placeholder={USE_MOCK ? '输入任意账号或邮箱' : 'name@example.com'}
                type={USE_MOCK ? 'text' : 'email'}
                value={email}
                onChange={setEmail}
                onBlur={() => setTouched((t) => ({ ...t, email: true }))}
                disabled={loading}
                autoComplete="username"
                autoFocus={!register}
                icon={<Mail className="h-4 w-4" />}
                hint={emailInvalid ? '邮箱格式似乎不正确，请检查后再试' : undefined}
                invalid={emailInvalid}
              />

              <div>
                <AuthInput
                  id="password"
                  label={register ? '设置密码' : '密码'}
                  placeholder={register ? '至少 6 位字符' : '输入登录密码'}
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={setPassword}
                  onBlur={() => setTouched((t) => ({ ...t, password: true }))}
                  disabled={loading}
                  autoComplete={register ? 'new-password' : 'current-password'}
                  icon={<Lock className="h-4 w-4" />}
                  hint={pwdTooShort ? '密码至少需要 6 个字符' : pwdHasEdgeSpace ? '密码首尾包含空格，请检查是否多复制了空格' : undefined}
                  invalid={pwdTooShort}
                  trailing={
                    <IconButton
                      type="button"
                      size="sm"
                      label={showPassword ? '隐藏密码' : '显示密码'}
                      onClick={() => setShowPassword(!showPassword)}
                    >
                      {showPassword ? <EyeOff /> : <Eye />}
                    </IconButton>
                  }
                />
                {register && password && (
                  <div className="mt-2 flex items-center gap-2">
                    <div className="grid flex-1 grid-cols-3 gap-1">
                      {[1, 2, 3].map((idx) => (
                        <span
                          key={idx}
                          className={cn(
                            'h-1 rounded-full transition-colors',
                            strength.score >= idx ? strength.color : 'bg-surface-3',
                          )}
                        />
                      ))}
                    </div>
                    <span className="text-[11px] text-muted">强度：{strength.label}</span>
                  </div>
                )}
              </div>

              {register && (
                <AuthInput
                  id="confirm-password"
                  label="确认密码"
                  placeholder="再次输入密码"
                  type={showPassword ? 'text' : 'password'}
                  value={confirmPassword}
                  onChange={setConfirmPassword}
                  onBlur={() => setTouched((t) => ({ ...t, confirm: true }))}
                  disabled={loading}
                  autoComplete="new-password"
                  icon={<Lock className="h-4 w-4" />}
                  hint={pwdMismatch ? '两次输入的密码不一致' : undefined}
                  invalid={pwdMismatch}
                />
              )}

              {error && (
                <div role="alert" className="flex items-start gap-2 rounded-xl border border-danger/25 bg-danger/10 px-3.5 py-2.5 text-sm text-danger">
                  <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              <Button
                type="submit"
                size="lg"
                loading={loading}
                className="mt-2 w-full bg-fg text-bg hover:opacity-90 dark:bg-fg dark:text-bg"
              >
                {loading
                  ? register ? '正在创建账户…' : '正在登录…'
                  : register ? '创建账户并开始' : '登录知页'}
                {!loading && <ArrowRight className="h-4 w-4" />}
              </Button>
            </form>

            <p className="mt-6 text-center text-xs leading-5 text-subtle">
              {register ? '注册' : '登录'}即表示你理解本系统仅用于个人学习与知识管理演示用途。
            </p>
          </section>
        </div>
      </main>
    </div>
  )
}

function AuthInput({
  id,
  label,
  value,
  onChange,
  onBlur,
  placeholder,
  type = 'text',
  disabled,
  autoComplete,
  autoFocus,
  maxLength,
  icon,
  trailing,
  hint,
  invalid,
}: {
  id: string
  label: string
  value: string
  onChange: (v: string) => void
  onBlur?: () => void
  placeholder?: string
  type?: string
  disabled?: boolean
  autoComplete?: string
  autoFocus?: boolean
  maxLength?: number
  icon?: React.ReactNode
  trailing?: React.ReactNode
  hint?: string
  invalid?: boolean
}) {
  return (
    <div>
      <label htmlFor={id} className="mb-1.5 block text-[13px] font-medium text-fg">
        {label}
      </label>
      <div className="relative">
        {icon && (
          <span className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-subtle">
            {icon}
          </span>
        )}
        <input
          id={id}
          name={id}
          type={type}
          value={value}
          placeholder={placeholder}
          disabled={disabled}
          autoComplete={autoComplete}
          autoFocus={autoFocus}
          maxLength={maxLength}
          onChange={(e) => onChange(e.target.value)}
          onBlur={onBlur}
          className={cn(
            'h-11 w-full rounded-xl border bg-surface-2 text-sm outline-none transition-all placeholder:text-subtle/75 focus:bg-surface focus:ring-4 disabled:opacity-60',
            icon ? 'pl-10' : 'pl-3.5',
            trailing ? 'pr-11' : 'pr-3.5',
            invalid
              ? 'border-danger focus:border-danger focus:ring-danger/15'
              : 'border-border focus:border-primary focus:ring-primary/15',
          )}
        />
        {trailing && <span className="absolute right-2 top-1/2 -translate-y-1/2">{trailing}</span>}
      </div>
      {hint && <p className="mt-1.5 text-xs text-danger">{hint}</p>}
    </div>
  )
}
