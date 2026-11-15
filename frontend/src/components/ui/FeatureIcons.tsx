import type { StudioKind } from '@/types'
import { cn } from '@/lib/utils'

export function StudioToolIcon({ kind, className }: { kind: StudioKind; className?: string }) {
  const box = cn('h-5 w-5', className)
  switch (kind) {
    case 'ppt':
      return (
        <svg viewBox="0 0 24 24" fill="none" className={box} aria-hidden>
          <rect x="2.5" y="4" width="19" height="13.5" rx="2.8" fill="currentColor" fillOpacity="0.16" stroke="currentColor" strokeWidth="1.6" />
          <path d="M8 20.5h8M12 17.5v3" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
          <path d="M7.5 13v-3.5M12 13V8.2M16.5 13v-2" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" />
        </svg>
      )
    case 'word':
      return (
        <svg viewBox="0 0 24 24" fill="none" className={box} aria-hidden>
          <path d="M6 3.5h8.2L18.5 7.8V19a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V5.5a2 2 0 0 1 2-2Z" fill="currentColor" fillOpacity="0.14" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
          <path d="M14 3.5V8h4.5" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
          <path d="M7.5 11.5h9M7.5 14.8h9M7.5 18h5.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
        </svg>
      )
    case 'pdf':
      return (
        <svg viewBox="0 0 24 24" fill="none" className={box} aria-hidden>
          <path d="M6 3.5h8.2L18.5 7.8V19a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V5.5a2 2 0 0 1 2-2Z" fill="currentColor" fillOpacity="0.14" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
          <path d="M14 3.5V8h4.5" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
          <rect x="6.5" y="12.2" width="11" height="5.2" rx="1.5" fill="currentColor" fillOpacity="0.22" stroke="currentColor" strokeWidth="1.2" />
          <path d="M8.8 14.8h6.4" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
        </svg>
      )
    case 'mindmap':
      return (
        <svg viewBox="0 0 24 24" fill="none" className={box} aria-hidden>
          <rect x="2.5" y="9" width="6.5" height="6" rx="2" fill="currentColor" fillOpacity="0.2" stroke="currentColor" strokeWidth="1.6" />
          <rect x="15" y="3.5" width="6.5" height="5" rx="1.8" fill="currentColor" fillOpacity="0.2" stroke="currentColor" strokeWidth="1.6" />
          <rect x="15" y="15.5" width="6.5" height="5" rx="1.8" fill="currentColor" fillOpacity="0.2" stroke="currentColor" strokeWidth="1.6" />
          <path d="M9 12h3c1.5 0 1.5-6 3-6M9 12h3c1.5 0 1.5 6 3 6" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
        </svg>
      )
    case 'study-guide':
      return (
        <svg viewBox="0 0 24 24" fill="none" className={box} aria-hidden>
          <path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H12v17H6.5A2.5 2.5 0 0 0 4 22V5.5Z" fill="currentColor" fillOpacity="0.16" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
          <path d="M20 5.5A2.5 2.5 0 0 0 17.5 3H12v17h5.5A2.5 2.5 0 0 1 20 22V5.5Z" fill="currentColor" fillOpacity="0.08" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
          <path d="M7 8h2.5M7 11.5h2.5M14.5 8H17M14.5 11.5H17" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      )
    case 'briefing':
      return (
        <svg viewBox="0 0 24 24" fill="none" className={box} aria-hidden>
          <rect x="4" y="3" width="16" height="18" rx="3" fill="currentColor" fillOpacity="0.15" stroke="currentColor" strokeWidth="1.6" />
          <path d="M8 14v-3.5M12 14V8M16 14v-2" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          <path d="M8 17.5h8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
        </svg>
      )
    case 'faq':
      return (
        <svg viewBox="0 0 24 24" fill="none" className={box} aria-hidden>
          <path d="M4 6.5A3.5 3.5 0 0 1 7.5 3h9A3.5 3.5 0 0 1 20 6.5v6A3.5 3.5 0 0 1 16.5 16H11l-4.5 4v-4A3.5 3.5 0 0 1 4 12.5v-6Z" fill="currentColor" fillOpacity="0.16" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
          <path d="M9.8 8.3a2.2 2.2 0 1 1 3.3 1.9c-.8.45-1.1.9-1.1 1.6" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
          <circle cx="12" cy="14" r="0.9" fill="currentColor" />
        </svg>
      )
    case 'timeline':
      return (
        <svg viewBox="0 0 24 24" fill="none" className={box} aria-hidden>
          <path d="M6.5 4.5v15" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
          <circle cx="6.5" cy="6.5" r="2.2" fill="currentColor" />
          <circle cx="6.5" cy="12" r="2.2" fill="currentColor" fillOpacity="0.35" stroke="currentColor" strokeWidth="1.5" />
          <circle cx="6.5" cy="17.5" r="2.2" fill="currentColor" />
          <rect x="11" y="4.8" width="9.5" height="3.4" rx="1.7" fill="currentColor" fillOpacity="0.18" />
          <rect x="11" y="10.3" width="7" height="3.4" rx="1.7" fill="currentColor" fillOpacity="0.18" />
          <rect x="11" y="15.8" width="9.5" height="3.4" rx="1.7" fill="currentColor" fillOpacity="0.18" />
        </svg>
      )
  }
}

export function UploadIllustration({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 64 64" fill="none" className={cn('h-16 w-16', className)} aria-hidden>
      <rect x="10" y="14" width="32" height="40" rx="7" fill="currentColor" fillOpacity="0.1" stroke="currentColor" strokeOpacity="0.35" strokeWidth="1.8" transform="rotate(-6 10 14)" />
      <rect x="20" y="10" width="34" height="42" rx="8" fill="rgb(var(--surface))" stroke="currentColor" strokeWidth="2" />
      <path d="M37 22v14m0-14-5 5m5-5 5 5" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M29 41h16" stroke="currentColor" strokeOpacity="0.45" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}
