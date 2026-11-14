import { useState } from 'react'
import { Trash2 } from 'lucide-react'
import type { Notebook, Tag } from '@/types'
import { notebooksApi, tagsApi } from '@/api'
import { reportError } from '@/api/client'
import { Button, IconButton } from '@/components/ui/Button'
import { Modal } from '@/components/ui/Modal'
import { confirmDialog } from '@/store/useUIStore'

export function TagBadge({ tag }: { tag: Tag }) {
  return <span className="inline-flex max-w-full items-center gap-1.5 rounded-lg px-2 py-0.5 text-xs" style={{ backgroundColor: `${tag.color}20` }}><span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: tag.color }} /><span className="truncate">{tag.name}</span></span>
}

export function EditTagsModal({ notebook, tags, onClose, onChanged, onSaved }: { notebook: Notebook; tags: Tag[]; onClose: () => void; onChanged: (deletedId?: string) => void; onSaved: (nb: Notebook) => void }) {
  const [items, setItems] = useState(tags)
  const [selected, setSelected] = useState(notebook.tags.map((t) => t.id))
  const [name, setName] = useState('')
  const [color, setColor] = useState('#6750A4')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const run = async (action: () => Promise<void>) => {
    if (busy) return
    setBusy(true); setError('')
    try { await action() } catch (e) { setError(reportError(e).message) } finally { setBusy(false) }
  }
  const add = () => run(async () => {
    const tag = await tagsApi.create(name.trim(), color)
    setItems((v) => [...v, tag]); setSelected((v) => [...v, tag.id]); setName(''); onChanged()
  })
  const remove = async (tag: Tag) => {
    if (!await confirmDialog({ title: `删除标签「${tag.name}」？`, description: '此标签会从所有笔记本中移除。', confirmText: '删除', danger: true })) return
    void run(async () => { await tagsApi.remove(tag.id); setItems((v) => v.filter((t) => t.id !== tag.id)); setSelected((v) => v.filter((id) => id !== tag.id)); onChanged(tag.id) })
  }
  return <Modal open onClose={() => !busy && onClose()} title="编辑标签" description={`为「${notebook.title}」整理分类`} footer={<><Button variant="ghost" disabled={busy} onClick={onClose}>取消</Button><Button loading={busy} onClick={() => run(async () => { const nb = await notebooksApi.setTags(notebook.id, selected); onSaved(nb) })}>保存</Button></>}>
    {error && <p role="alert" className="mb-3 text-sm text-danger">{error}</p>}
    <fieldset disabled={busy} className="space-y-2 disabled:opacity-60">
      {items.length === 0 && <p className="py-4 text-sm text-muted">还没有标签，在下方新建一个吧。</p>}
      {items.map((tag) => <div key={tag.id} className="flex items-center gap-3 rounded-xl bg-surface-2 px-3 py-2">
        <label className="flex min-w-0 flex-1 cursor-pointer items-center gap-3"><input type="checkbox" checked={selected.includes(tag.id)} onChange={(e) => setSelected((v) => e.target.checked ? [...v, tag.id] : v.filter((id) => id !== tag.id))} className="h-4 w-4 accent-primary" /><TagBadge tag={tag} /></label>
        <input type="color" aria-label={`修改「${tag.name}」颜色`} value={tag.color} onChange={(e) => { const color = e.target.value; void run(async () => { const updated = await tagsApi.update(tag.id, { color }); setItems((v) => v.map((t) => t.id === tag.id ? updated : t)); onChanged() }) }} className="h-7 w-7 cursor-pointer rounded border-0 bg-transparent" />
        <IconButton label={`删除标签「${tag.name}」`} size="sm" onClick={() => void remove(tag)}><Trash2 /></IconButton>
      </div>)}
      <div className="pt-4">
        <h3 className="mb-3 text-sm font-medium">新建标签</h3>
        <div className="flex items-center gap-2"><input aria-label="标签名称" maxLength={50} value={name} onChange={(e) => setName(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter' && name.trim()) void add() }} placeholder="例如：毕业设计" className="h-11 min-w-0 flex-1 rounded-xl border border-border bg-surface-2 px-3 text-sm outline-none focus:border-primary" /><input aria-label="标签颜色" type="color" value={color} onChange={(e) => setColor(e.target.value)} className="h-9 w-9 cursor-pointer border-0 bg-transparent" /><Button variant="secondary" disabled={!name.trim()} onClick={add}>新建</Button></div>
        <p className="mt-2 text-xs text-muted">新标签会自动勾选，保存后应用到此笔记本。</p>
      </div>
    </fieldset>
  </Modal>
}
