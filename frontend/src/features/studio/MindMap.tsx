import { useMemo, useRef, useState, type PointerEvent } from 'react'
import { ZoomIn, ZoomOut, Maximize2, ChevronsDownUp, ChevronsUpDown } from 'lucide-react'
import type { MindMapNode } from '@/types'
import { IconButton } from '@/components/ui/Button'

const ROW_H = 44
const NODE_H = 32
const GAP_X = 56
const textW = (s: string) => [...s].reduce((w, ch) => w + (/[一-龥＀-￯]/.test(ch) ? 13 : 7.4), 0) + 28

interface Placed {
  node: MindMapNode
  depth: number
  x: number
  y: number
  w: number
  parent?: Placed
  hasChildren: boolean
  collapsed: boolean
}

function layout(root: MindMapNode, collapsed: Set<string>) {
  const placed: Placed[] = []
  const colW: number[] = []
  let row = 0
  const walk = (n: MindMapNode, depth: number, parent?: Placed): Placed => {
    const w = Math.min(textW(n.label), 260)
    colW[depth] = Math.max(colW[depth] ?? 0, w)
    const hasChildren = !!n.children?.length
    const isCollapsed = collapsed.has(n.id)
    const p: Placed = { node: n, depth, x: 0, y: 0, w, parent, hasChildren, collapsed: isCollapsed }
    placed.push(p)
    if (hasChildren && !isCollapsed) {
      const kids = n.children!.map((c) => walk(c, depth + 1, p))
      p.y = (kids[0].y + kids[kids.length - 1].y) / 2
    } else {
      p.y = row++ * ROW_H
    }
    return p
  }
  walk(root, 0)
  const colX = colW.reduce<number[]>((acc, _w, i) => (acc.push(i === 0 ? 0 : acc[i - 1] + colW[i - 1] + GAP_X), acc), [])
  placed.forEach((p) => (p.x = colX[p.depth]))
  const width = colX[colX.length - 1] + colW[colW.length - 1]
  const height = Math.max(row, 1) * ROW_H
  return { placed, width, height }
}

const PALETTE = ['#6366f1', '#0ea5e9', '#10b981', '#f59e0b', '#ec4899', '#8b5cf6']

export function MindMap({ data, className }: { data: MindMapNode; className?: string }) {
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const [scale, setScale] = useState(1)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const drag = useRef<{ x: number; y: number; px: number; py: number } | null>(null)
  const { placed, width, height } = useMemo(() => layout(data, collapsed), [data, collapsed])

  const branchColor = (p: Placed): string => {
    let cur = p
    while (cur.parent && cur.parent.parent) cur = cur.parent
    if (!cur.parent) return PALETTE[0]
    const idx = data.children?.findIndex((c) => c.id === cur.node.id) ?? 0
    return PALETTE[(idx + 1) % PALETTE.length]
  }

  const toggle = (id: string) =>
    setCollapsed((s) => {
      const n = new Set(s)
      n.has(id) ? n.delete(id) : n.add(id)
      return n
    })

  const collapseAll = () => setCollapsed(new Set(data.children?.filter((c) => c.children?.length).map((c) => c.id)))
  const reset = () => (setScale(1), setPan({ x: 0, y: 0 }))

  const onDown = (e: PointerEvent) => {
    if ((e.target as Element).closest('[data-node]')) return
    drag.current = { x: e.clientX, y: e.clientY, px: pan.x, py: pan.y }
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
  }
  const onMove = (e: PointerEvent) => {
    if (!drag.current) return
    setPan({ x: drag.current.px + e.clientX - drag.current.x, y: drag.current.py + e.clientY - drag.current.y })
  }

  const PAD = 24
  return (
    <div className={`relative overflow-hidden rounded-2xl border border-border/70 bg-surface-2 ${className ?? ''}`}>
      <div className="absolute right-2 top-2 z-10 flex gap-0.5 rounded-full border border-border/70 bg-surface/90 p-0.5 shadow-card backdrop-blur">
        <IconButton label="放大" size="sm" onClick={() => setScale((s) => Math.min(2, s + 0.15))}><ZoomIn /></IconButton>
        <IconButton label="缩小" size="sm" onClick={() => setScale((s) => Math.max(0.4, s - 0.15))}><ZoomOut /></IconButton>
        <IconButton label="全部展开" size="sm" onClick={() => setCollapsed(new Set())}><ChevronsUpDown /></IconButton>
        <IconButton label="全部收起" size="sm" onClick={collapseAll}><ChevronsDownUp /></IconButton>
        <IconButton label="重置视图" size="sm" onClick={reset}><Maximize2 /></IconButton>
      </div>
      <div
        className="h-full w-full cursor-grab touch-none overflow-hidden active:cursor-grabbing"
        onPointerDown={onDown}
        onPointerMove={onMove}
        onPointerUp={() => (drag.current = null)}
        onWheel={(e) => e.ctrlKey && setScale((s) => Math.max(0.4, Math.min(2, s - e.deltaY * 0.002)))}
      >
        <svg
          width={(width + PAD * 2) * scale}
          height={(height + PAD * 2) * scale}
          viewBox={`${-PAD} ${-PAD - NODE_H / 2} ${width + PAD * 2} ${height + PAD * 2}`}
          style={{ transform: `translate(${pan.x}px, ${pan.y}px)` }}
          className="mx-auto mt-10 block transition-[width,height] duration-200"
        >
          {placed.filter((p) => p.parent).map((p) => {
            const a = p.parent!
            const x1 = a.x + a.w
            const y1 = a.y
            const x2 = p.x
            const y2 = p.y
            const mx = (x1 + x2) / 2
            return <path key={'e' + p.node.id} d={`M${x1},${y1} C${mx},${y1} ${mx},${y2} ${x2},${y2}`} fill="none" stroke={branchColor(p)} strokeOpacity={0.55} strokeWidth={1.6} />
          })}
          {placed.map((p) => {
            const color = branchColor(p)
            const isRoot = p.depth === 0
            return (
              <g
                key={p.node.id}
                data-node
                transform={`translate(${p.x},${p.y - NODE_H / 2})`}
                onClick={() => p.hasChildren && toggle(p.node.id)}
                className={p.hasChildren ? 'cursor-pointer' : ''}
              >
                <rect
                  width={p.w}
                  height={NODE_H}
                  rx={NODE_H / 2}
                  fill={isRoot ? color : 'rgb(var(--surface))'}
                  stroke={color}
                  strokeOpacity={isRoot ? 1 : 0.5}
                  strokeWidth={1.4}
                />
                <text x={p.w / 2} y={NODE_H / 2 + 4.5} textAnchor="middle" fontSize={13} fontWeight={isRoot ? 600 : 500} fill={isRoot ? '#fff' : 'rgb(var(--fg))'}>
                  {p.node.label.length > 22 ? p.node.label.slice(0, 21) + '…' : p.node.label}
                </text>
                {p.hasChildren && !isRoot && (
                  <g transform={`translate(${p.w + 2},${NODE_H / 2})`}>
                    <circle r={8} fill="rgb(var(--surface))" stroke={color} strokeWidth={1.4} />
                    <text textAnchor="middle" y={4} fontSize={12} fontWeight={700} fill={color}>
                      {p.collapsed ? '+' : '−'}
                    </text>
                  </g>
                )}
              </g>
            )
          })}
        </svg>
      </div>
      <p className="pointer-events-none absolute bottom-2 left-3 text-[11px] text-subtle">点击节点展开/收起 · 拖拽平移 · Ctrl+滚轮缩放</p>
    </div>
  )
}
