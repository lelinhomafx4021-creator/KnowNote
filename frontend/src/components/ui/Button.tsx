import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { Loader2 } from 'lucide-react'
import { cn } from '@/lib/utils'

type Variant = 'primary' | 'secondary' | 'ghost' | 'outline' | 'danger' | 'soft'
type Size = 'sm' | 'md' | 'lg'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  loading?: boolean
  icon?: ReactNode
}

const variants: Record<Variant, string> = {
  primary: 'bg-primary text-primary-fg hover:bg-primary/90 shadow-sm',
  secondary: 'bg-surface-3 text-fg hover:bg-surface-3/70',
  soft: 'bg-primary-soft text-primary hover:bg-primary/15',
  ghost: 'text-muted hover:bg-surface-3 hover:text-fg',
  outline: 'border border-border bg-surface text-fg hover:bg-surface-2',
  danger: 'bg-danger text-white hover:bg-danger/90',
}
const sizes: Record<Size, string> = {
  sm: 'h-8 px-3 text-[13px] gap-1.5 rounded-full',
  md: 'h-10 px-4 text-sm gap-2 rounded-full',
  lg: 'h-12 px-6 text-[15px] gap-2 rounded-full',
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading, icon, className, children, disabled, ...rest }, ref) => (
    <button
      ref={ref}
      disabled={disabled || loading}
      className={cn(
        'focus-ring inline-flex shrink-0 items-center justify-center font-medium transition-colors disabled:pointer-events-none disabled:opacity-50',
        variants[variant], sizes[size], className,
      )}
      {...rest}
    >
      {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : icon}
      {children}
    </button>
  ),
)
Button.displayName = 'Button'

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  label: string
  size?: 'sm' | 'md'
  active?: boolean
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ label, size = 'md', active, className, children, ...rest }, ref) => (
    <button
      ref={ref}
      aria-label={label}
      title={label}
      className={cn(
        'focus-ring inline-flex shrink-0 items-center justify-center rounded-full text-muted transition-colors hover:bg-surface-3 hover:text-fg disabled:opacity-40 disabled:pointer-events-none',
        size === 'sm' ? 'h-8 w-8 [&_svg]:h-4 [&_svg]:w-4' : 'h-10 w-10 [&_svg]:h-5 [&_svg]:w-5',
        active && 'bg-primary-soft text-primary hover:bg-primary-soft hover:text-primary',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  ),
)
IconButton.displayName = 'IconButton'
