import { useId } from 'react'
import { Link } from 'react-router-dom'
import { cn } from '@/lib/utils'

export function LogoMark({ className }: { className?: string }) {
  const id = useId()
  const bgId = `${id}-bg`
  const pageId = `${id}-page`
  return (
    <svg viewBox="0 0 36 36" fill="none" className={cn('h-8 w-8 shrink-0', className)} aria-hidden>
      <defs>
        <linearGradient id={bgId} x1="2" y1="2" x2="34" y2="34" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#0b57d0" />
          <stop offset="0.55" stopColor="#2563eb" />
          <stop offset="1" stopColor="#6366f1" />
        </linearGradient>
        <linearGradient id={pageId} x1="9" y1="8" x2="26" y2="28" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#ffffff" />
          <stop offset="1" stopColor="#e8f0fe" />
        </linearGradient>
      </defs>
      <rect width="36" height="36" rx="10" fill={`url(#${bgId})`} />
      <rect x="0.6" y="0.6" width="34.8" height="34.8" rx="9.4" stroke="#ffffff" strokeOpacity="0.24" strokeWidth="1.2" />
      <path d="M9.5 11.5A3.5 3.5 0 0 1 13 8h9.2L26.5 12.3V24.5A3.5 3.5 0 0 1 23 28H13a3.5 3.5 0 0 1-3.5-3.5v-13Z" fill={`url(#${pageId})`} />
      <path d="M22.2 8v3.1a1.2 1.2 0 0 0 1.2 1.2h3.1L22.2 8Z" fill="#93c5fd" />
      <path d="M13.5 15.5h8.5M13.5 19.5h6M13.5 23.5h4" stroke="#1d4ed8" strokeWidth="1.9" strokeLinecap="round" />
      <path d="M24.2 19.5c.35 1.4.95 2 2.35 2.35-1.4.35-2 .95-2.35 2.35-.35-1.4-.95-2-2.35-2.35 1.4-.35 2-.95 2.35-2.35Z" fill="#4f46e5" />
    </svg>
  )
}

export function Logo({ compact, to = '/' }: { compact?: boolean; to?: string }) {
  return (
    <Link to={to} className="focus-ring inline-flex items-center gap-2.5 rounded-xl py-0.5 pr-1">
      <LogoMark />
      {!compact && (
        <span className="flex items-baseline gap-1.5 text-[17px] font-semibold tracking-tight">
          <span>知页</span>
          <span className="text-[13px] font-medium text-muted">KnowNote</span>
        </span>
      )}
    </Link>
  )
}
