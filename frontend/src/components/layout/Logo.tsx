import { useId } from 'react'
import { Link } from 'react-router-dom'
import { cn } from '@/lib/utils'

export function LogoMark({ className }: { className?: string }) {
  const gradientId = useId()
  return (
    <svg viewBox="0 0 32 32" className={cn('h-8 w-8', className)} aria-hidden>
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#0b57d0" />
          <stop offset="1" stopColor="#4c8df6" />
        </linearGradient>
      </defs>
      <rect width="32" height="32" rx="9" fill={`url(#${gradientId})`} />
      <path d="M10 9h8a4 4 0 0 1 4 4v10H14a4 4 0 0 1-4-4z" fill="#fff" opacity=".95" />
      <path d="M14 14h5M14 18h3" stroke="#0b57d0" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  )
}

export function Logo({ compact }: { compact?: boolean }) {
  return (
    <Link to="/" className="focus-ring flex items-center gap-2.5 rounded-xl">
      <LogoMark />
      {!compact && (
        <span className="text-[17px] font-semibold tracking-tight">
          知页 <span className="font-medium text-muted">KnowNote</span>
        </span>
      )}
    </Link>
  )
}
