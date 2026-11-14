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
  const field = 'h-11 w-full rounded-xl border border-border bg-surface px-3.5 text-sm outline-none transition placeholder:text-subtle hover:border-subtle/60 focus:border-primary focus:ring-4 focus:ring-primary/10 disabled:opacity-60'

  return (
    <main className="min-h-full bg-bg p-3 sm:p-6 lg:flex lg:min-h-screen lg:gap-6">
      <section className="relative hidden w-[50%] flex-col overflow-hidden rounded-[28px] bg-[#0d1024] p-10 text-white lg:flex xl:p-14">
        <div className="pointer-events-none absolute inset-0 opacity-[0.35] [background-image:linear-gradient(rgba(255,255,255,.06)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,.06)_1px,transparent_1px)] [background-size:44px_44px] [mask-image:radial-gradient(ellipse_at_30%_40%,black_20%,transparent_75%)]" />
        <div className="pointer-events-none absolute -left-24 top-1/4 h-[420px] w-[420px] rounded-full bg-[#3d5afe]/35 blur-[110px]" />
        <div className="pointer-events-none absolute -bottom-32 right-0 h-[360px] w-[360px] rounded-full bg-[#7c5cfc]/30 blur-[110px]" />
        <div className="relative flex items-center gap-2.5 text-[17px] font-semibold tracking-tight">
          <span className="flex h-8 w-8 items-center justify-center rounded-[9px] bg-white text-[#3d5afe]"><BookOpen className="h-4 w-4" /></span>
          知页 <span className="font-medium text-white/50">KnowNote</span>
        </div>
        <div className="relative my-auto max-w-[460px] py-12">
          <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/5 px-3 py-1.5 text-xs font-medium text-white/80 backdrop-blur"><Sparkles className="h-3.5 w-3.5" /> 基于大语言模型的个人知识库</div>
          <h1 className="text-[40px] font-semibold leading-[1.25] tracking-tight xl:text-[46px]">每一份资料，<br />都值得被<span className="bg-gradient-to-r from-[#8fa2ff] to-[#c3b4ff] bg-clip-text text-transparent">真正理解</span>。</h1>
          <p className="mt-5 max-w-sm text-[15px] leading-7 text-white/60">把散落的论文、网页与笔记汇聚在一起，与资料对话，让每个观点都能回到原文。</p>
          <div className="mt-10 rounded-2xl border border-white/10 bg-white/[0.06] p-5 shadow-2xl shadow-black/30 backdrop-blur-md">
            <div className="flex items-center gap-2.5 text-xs text-white/60">
              <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-white/10"><BookOpen className="h-3.5 w-3.5" /></span>
              毕业设计 · 文献综述
              <span className="ml-auto rounded-full bg-emerald-400/15 px-2 py-0.5 text-emerald-300">12 个来源</span>
            </div>
            <p className="mt-4 text-sm font-medium text-white/90">RAG 和模型微调有什么区别？</p>
            <p className="mt-2.5 text-[13px] leading-6 text-white/60">
              RAG 不修改模型参数，而是在回答前检索外部资料作为上下文<span className="mx-0.5 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-[#8fa2ff]/25 px-1 text-[10px] font-semibold text-[#c7d0ff]">1</span>，因此知识可以随时更新并给出出处<span className="mx-0.5 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-[#8fa2ff]/25 px-1 text-[10px] font-semibold text-[#c7d0ff]">2</span>。
            </p>
          </div>
          <div className="mt-10 grid grid-cols-3 gap-4">
            {[[FileStack, '多源资料', 'PDF、网页、文档'], [Quote, '有据可查', '回答附原文引用'], [Sparkles, '自动整理', '导图、指南、对谈']].map(([Icon, title, description]) => {
              const FeatureIcon = Icon as typeof Sparkles
              return <div key={String(title)}><FeatureIcon className="h-4 w-4 text-[#8fa2ff]" /><p className="mt-2 text-sm font-medium text-white/90">{String(title)}</p><p className="mt-0.5 text-xs text-white/45">{String(description)}</p></div>
            })}
          </div>
        </div>
        <p className="relative flex items-center gap-2 text-xs text-white/40"><Check className="h-3.5 w-3.5" /> 从收集资料，到真正理解。</p>
      </section>
      <section className="relative flex min-h-[calc(100dvh-24px)] flex-1 flex-col rounded-[28px] border border-border/70 bg-surface px-6 py-6 shadow-card sm:min-h-[calc(100dvh-48px)] sm:px-10">
        <div className="flex items-center justify-between"><div className="lg:invisible"><Logo /></div><ThemeToggle /></div>
        <div className="mx-auto my-auto w-full max-w-sm py-12">
          
          <h2 className="text-3xl font-semibold tracking-tight">{register ? '开启你的知识之旅' : '欢迎回到知页'}</h2>
          <p className="mb-8 mt-3 text-sm leading-6 text-muted">{register ? '创建账户，给你的灵感一个生长的地方。' : '登录账户，继续探索你的知识世界。'}</p>
          {USE_MOCK && <p className="mb-6 rounded-xl border border-primary/15 bg-primary-soft/50 px-3.5 py-2.5 text-xs leading-5 text-primary">当前为离线演示模式，填写任意账号与密码即可体验。</p>}
          <form onSubmit={submit} noValidate className="space-y-4">
            {register && <div><label htmlFor="nickname" className="mb-1.5 block text-[13px] font-medium">昵称</label><input id="nickname" name="nickname" autoComplete="nickname" maxLength={40} autoFocus value={nickname} onChange={(e) => setNickname(e.target.value)} disabled={loading} className={field} placeholder="希望我们如何称呼你" required /></div>}
            <div><label htmlFor="email" className="mb-1.5 block text-[13px] font-medium">{USE_MOCK ? '账号 / 邮箱' : '邮箱'}</label><input id="email" name="email" type={USE_MOCK ? 'text' : 'email'} autoComplete="username" autoFocus={!register} value={email} onChange={(e) => setEmail(e.target.value)} disabled={loading} className={field} placeholder="输入你的邮箱地址" required /></div>
            <div><label htmlFor="password" className="mb-1.5 block text-[13px] font-medium">密码</label><div className="relative"><input id="password" name="password" type={showPassword ? 'text' : 'password'} autoComplete={register ? 'new-password' : 'current-password'} value={password} onChange={(e) => setPassword(e.target.value)} disabled={loading} className={`${field} pr-12`} placeholder={register ? '设置密码，至少 6 位' : '输入你的密码'} required /><IconButton type="button" label={showPassword ? '隐藏密码' : '显示密码'} className="absolute right-1 top-1 h-9 w-9" onClick={() => setShowPassword(!showPassword)}>{showPassword ? <EyeOff /> : <Eye />}</IconButton></div></div>
            {error && <p role="alert" className="flex items-start gap-2 rounded-xl bg-danger/10 p-3 text-sm text-danger"><AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />{error}</p>}
            <Button type="submit" size="lg" className="!mt-6 h-11 w-full rounded-xl" loading={loading} icon={<ArrowRight className="h-4 w-4" />}>{loading ? (register ? '正在创建账户…' : '正在登录…') : (register ? '创建账户' : '登录')}</Button>
          </form>
          <p className="mt-7 text-center text-sm text-muted">{register ? '已经有账户？' : '还没有账户？'} <Link className="focus-ring rounded text-primary hover:underline" to={`${register ? '/login' : '/register'}?redirect=${encodeURIComponent(redirect)}`}>{register ? '立即登录' : '免费注册'}</Link></p>
        </div>
        <p className="text-center text-xs text-subtle">知页 KnowNote · 让知识触手可及</p>
      </section>
    </main>
  )
}
