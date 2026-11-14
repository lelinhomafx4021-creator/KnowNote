import { useState, type FormEvent } from 'react'
import { Link, Navigate, useSearchParams } from 'react-router-dom'
import { ArrowRight, BookOpen, Check, FileStack, Quote, Sparkles, AlertCircle, Eye, EyeOff } from 'lucide-react'
import { authApi } from '@/api/auth'
import { USE_MOCK } from '@/api/client'
import { useAuthStore } from '@/store/useAuthStore'
import { Logo } from '@/components/layout/Logo'
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
  const field = 'h-12 w-full rounded-2xl border border-border bg-surface-2 px-4 text-sm outline-none transition focus:border-primary focus:ring-4 focus:ring-primary/10 disabled:opacity-60'

  return (
    <main className="min-h-full bg-bg p-3 sm:p-6 lg:flex lg:min-h-screen lg:gap-6">
      <section className="relative hidden w-[52%] flex-col overflow-hidden rounded-[32px] border border-primary/10 bg-gradient-to-br from-primary-soft via-surface to-primary-soft/50 p-10 lg:flex xl:p-14">
        <div className="pointer-events-none absolute -right-32 top-24 h-96 w-96 rounded-full bg-accent/10 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-40 -left-32 h-96 w-96 rounded-full bg-primary/15 blur-3xl" />
        <Logo />
        <div className="relative my-auto max-w-lg py-14">
          <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-primary/15 bg-surface/70 px-3 py-1.5 text-xs font-medium text-primary"><Sparkles className="h-3.5 w-3.5" /> 你的个人 AI 知识空间</div>
          <h1 className="text-4xl font-semibold leading-[1.35] tracking-tight xl:text-5xl">每一份资料，<br />都能成为<span className="text-gradient">新的灵感。</span></h1>
          <p className="mt-5 max-w-sm text-[15px] leading-7 text-muted">把散落的论文、网页与笔记汇聚在这里。与资料对话，让知识彼此连接。</p>
          <div className="relative mt-9 rotate-[-2deg] rounded-3xl border border-border/70 bg-surface/90 p-5 shadow-pop">
            <div className="flex items-center gap-2 text-xs text-muted"><span className="flex h-8 w-8 items-center justify-center rounded-xl bg-primary-soft text-primary"><BookOpen className="h-4 w-4" /></span> 我的研究笔记本 <span className="ml-auto rounded-full bg-success/10 px-2 py-1 text-success">已整理</span></div>
            <p className="mt-4 text-sm font-medium">这些资料之间，有什么联系？</p>
            <div className="mt-3 rounded-2xl bg-surface-2 p-3 text-[13px] leading-6 text-muted">从一个问题出发，发现资料中的共同线索。每个观点，都可以回到原文。<span className="ml-1 rounded bg-primary-soft px-1 text-primary">1</span><span className="ml-1 rounded bg-primary-soft px-1 text-primary">2</span></div>
          </div>
          <div className="mt-9 space-y-4">
            {[[FileStack, '多种资料，一个空间', '支持论文、网页、文档与音视频'], [Quote, '有据可查的回答', '基于你的来源提问，随时追溯引用'], [Sparkles, '让整理更轻松', '学习指南、思维导图与音频对谈']].map(([Icon, title, description]) => {
              const FeatureIcon = Icon as typeof Sparkles
              return <div key={String(title)} className="flex items-center gap-3"><span className="flex h-9 w-9 items-center justify-center rounded-xl bg-surface/80 text-primary"><FeatureIcon className="h-4 w-4" /></span><div><p className="text-sm font-medium">{String(title)}</p><p className="mt-0.5 text-xs text-muted">{String(description)}</p></div></div>
            })}
          </div>
        </div>
        <p className="relative flex items-center gap-2 text-xs text-muted"><Check className="h-3.5 w-3.5 text-primary" /> 从收集资料，到真正理解。</p>
      </section>
      <section className="relative flex min-h-[calc(100dvh-24px)] flex-1 flex-col rounded-[32px] bg-surface px-6 py-6 sm:min-h-[calc(100dvh-48px)] sm:px-10">
        <div className="flex items-center justify-between"><div className="lg:invisible"><Logo /></div><ThemeToggle /></div>
        <div className="mx-auto my-auto w-full max-w-sm py-12">
          <span className="mb-5 flex h-12 w-12 items-center justify-center rounded-2xl bg-primary-soft text-primary"><BookOpen className="h-6 w-6" /></span>
          <h2 className="text-3xl font-semibold tracking-tight">{register ? '开启你的知识之旅' : '欢迎回到知页'}</h2>
          <p className="mb-8 mt-3 text-sm leading-6 text-muted">{register ? '创建账户，给你的灵感一个生长的地方。' : '登录账户，继续探索你的知识世界。'}</p>
          {USE_MOCK && <p className="mb-5 rounded-2xl bg-primary-soft/60 px-4 py-3 text-xs leading-5 text-primary">当前为离线演示模式，填写任意账号与密码即可体验。</p>}
          <form onSubmit={submit} noValidate className="space-y-5">
            {register && <div><label htmlFor="nickname" className="mb-2 block text-sm font-medium">昵称</label><input id="nickname" name="nickname" autoComplete="nickname" maxLength={40} autoFocus value={nickname} onChange={(e) => setNickname(e.target.value)} disabled={loading} className={field} placeholder="希望我们如何称呼你" required /></div>}
            <div><label htmlFor="email" className="mb-2 block text-sm font-medium">{USE_MOCK ? '账号 / 邮箱' : '邮箱'}</label><input id="email" name="email" type={USE_MOCK ? 'text' : 'email'} autoComplete="username" autoFocus={!register} value={email} onChange={(e) => setEmail(e.target.value)} disabled={loading} className={field} placeholder="输入你的邮箱地址" required /></div>
            <div><label htmlFor="password" className="mb-2 block text-sm font-medium">密码</label><div className="relative"><input id="password" name="password" type={showPassword ? 'text' : 'password'} autoComplete={register ? 'new-password' : 'current-password'} value={password} onChange={(e) => setPassword(e.target.value)} disabled={loading} className={`${field} pr-12`} placeholder={register ? '设置密码，至少 6 位' : '输入你的密码'} required /><IconButton type="button" label={showPassword ? '隐藏密码' : '显示密码'} className="absolute right-1.5 top-1.5 h-9 w-9" onClick={() => setShowPassword(!showPassword)}>{showPassword ? <EyeOff /> : <Eye />}</IconButton></div></div>
            {error && <p role="alert" className="flex items-start gap-2 rounded-xl bg-danger/10 p-3 text-sm text-danger"><AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />{error}</p>}
            <Button type="submit" size="lg" className="w-full" loading={loading} icon={<ArrowRight className="h-4 w-4" />}>{loading ? (register ? '正在创建账户…' : '正在登录…') : (register ? '创建账户' : '登录')}</Button>
          </form>
          <p className="mt-7 text-center text-sm text-muted">{register ? '已经有账户？' : '还没有账户？'} <Link className="focus-ring rounded text-primary hover:underline" to={`${register ? '/login' : '/register'}?redirect=${encodeURIComponent(redirect)}`}>{register ? '立即登录' : '免费注册'}</Link></p>
        </div>
        <p className="text-center text-xs text-subtle">知页 KnowNote · 让知识触手可及</p>
      </section>
    </main>
  )
}
