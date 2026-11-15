import { useRef, useState, type DragEvent } from 'react'
import { UploadCloud, Link2, ClipboardType, FileText } from 'lucide-react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Segmented, Progress } from '@/components/ui/misc'
import { SourceIcon } from '@/components/ui/SourceIcon'
import { UploadIllustration } from '@/components/ui/FeatureIcons'
import { useWorkspace } from '@/store/useWorkspaceStore'
import type { SourceType } from '@/types'
import { toast } from '@/store/useUIStore'
import { cn } from '@/lib/utils'

type Tab = 'upload' | 'link' | 'text'
const MAX_SOURCES = 50

function detectType(name: string): SourceType {
  const ext = name.split('.').pop()?.toLowerCase()
  if (ext === 'pdf') return 'pdf'
  if (ext === 'doc' || ext === 'docx') return 'docx'
  if (ext === 'ppt' || ext === 'pptx') return 'pptx'
  if (ext === 'xls' || ext === 'xlsx') return 'xlsx'
  if (ext === 'csv') return 'csv'
  if (ext === 'md' || ext === 'markdown') return 'markdown'
  if (['mp3', 'wav', 'm4a'].includes(ext ?? '')) return 'audio'
  return 'text'
}

export function AddSourceModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [tab, setTab] = useState<Tab>('upload')
  const [drag, setDrag] = useState(false)
  const [url, setUrl] = useState('')
  const [title, setTitle] = useState('')
  const [text, setText] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const { addSource, sources } = useWorkspace()

  const reset = () => {
    setUrl('')
    setTitle('')
    setText('')
    setDrag(false)
    setTab('upload')
  }
  const close = () => {
    reset()
    onClose()
  }

  const handleFiles = (files: FileList | null) => {
    if (!files?.length) return
    let added = false
    Array.from(files).forEach((f) => {
      if (!/\.(pdf|doc|docx|ppt|pptx|xls|xlsx|csv|txt|md|markdown|mp3|wav|m4a)$/i.test(f.name)) { toast(`「${f.name}」的文件格式暂不支持`, 'error'); return }
      if (f.size > 200 * 1024 * 1024) { toast(`「${f.name}」超过 200MB`, 'error'); return }
      void addSource({ type: detectType(f.name), title: f.name, size: f.size, file: f })
      added = true
    })
    if (inputRef.current) inputRef.current.value = ''
    if (added) close()
  }

  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setDrag(false)
    handleFiles(e.dataTransfer.files)
  }

  const urlValid = /^https?:\/\/\S+\.\S+/.test(url.trim())
  const isYoutube = /youtube\.com|youtu\.be|bilibili\.com/.test(url)

  const submitLink = () => {
    if (!urlValid) return
    const u = url.trim()
    let host = u
    try {
      host = new URL(u).hostname
    } catch {
      /* ignore */
    }
    addSource({ type: isYoutube ? 'youtube' : 'web', title: isYoutube ? `视频：${host}` : `网页：${host}`, url: u })
    close()
  }
  const submitText = () => {
    if (!text.trim()) return
    addSource({ type: 'text', title: title.trim() || text.trim().slice(0, 20), text })
    close()
  }

  return (
    <Modal
      open={open}
      onClose={close}
      size="lg"
      title="添加来源"
      description="汇集你的资料，获得有据可查的回答。上传后会自动整理来源概览与目录。"
    >
      <Segmented
        className="mb-5 max-w-full [&_button]:px-2.5 [&_button]:text-xs sm:[&_button]:px-3 sm:[&_button]:text-[13px]"
        value={tab}
        onChange={setTab}
        options={[
          { value: 'upload', label: '上传文件', icon: <UploadCloud /> },
          { value: 'link', label: '网址链接', icon: <Link2 /> },
          { value: 'text', label: '粘贴文本', icon: <ClipboardType /> },
        ]}
      />

      {tab === 'upload' && (
        <div
          onDragOver={(e) => (e.preventDefault(), setDrag(true))}
          onDragLeave={() => setDrag(false)}
          onDrop={onDrop}
          onClick={() => inputRef.current?.click()}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); inputRef.current?.click() } }}
          aria-label="选择或拖入多个来源文件"
          className={cn(
            'group flex cursor-pointer flex-col items-center justify-center rounded-3xl border-2 border-dashed px-6 py-12 text-center transition-colors',
            drag ? 'border-primary bg-primary-soft/60' : 'border-border bg-surface-2 hover:border-primary/40',
          )}
        >
          <div className="mb-4 text-primary transition-transform group-hover:scale-105">
            <UploadIllustration className="h-16 w-16" />
          </div>
          <p className="font-medium">拖拽文件到这里，或 <span className="text-primary">点击选择</span></p>
          <p className="mt-2 text-sm leading-6 text-muted">支持一次添加多个文件，单个文件不超过 200MB。</p>
          <div className="mt-5 flex flex-wrap justify-center gap-2 text-xs text-muted">
            {['PDF', 'PPT / PPTX', 'DOC / DOCX', 'XLS / XLSX', 'CSV', 'TXT / MD', 'MP3 / WAV / M4A'].map((t) => (
              <span key={t} className="rounded-full border border-border bg-surface px-2.5 py-1">{t}</span>
            ))}
          </div>
          <p className="mt-4 text-xs leading-5 text-subtle">演示文稿按幻灯片整理，表格按工作表整理。旧版 Office 文件会自动转换；PDF 与可预览的 Office 文件支持回到原件查看。</p>
          <input ref={inputRef} type="file" multiple hidden accept=".pdf,.ppt,.pptx,.doc,.docx,.xls,.xlsx,.csv,.txt,.md,.markdown,.mp3,.wav,.m4a" onChange={(e) => handleFiles(e.target.files)} />
        </div>
      )}

      {tab === 'link' && (
        <div className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2">
            <Hint icon={<SourceIcon type="web" size="md" />} title="网页" text="自动抓取正文，过滤广告和导航" />
            <Hint icon={<SourceIcon type="youtube" size="md" />} title="视频" text="支持 YouTube / B 站链接；暂不自动转写，可粘贴字幕文本" />
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium">粘贴网址</label>
            <input
              autoFocus
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && submitLink()}
              placeholder="https://"
              className="h-11 w-full rounded-xl border border-border bg-surface-2 px-3.5 text-sm outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
            />
            {url && !urlValid && <p className="mt-1.5 text-xs text-danger">请输入有效的网址</p>}
          </div>
          <div className="flex justify-end">
            <Button onClick={submitLink} disabled={!urlValid}>添加</Button>
          </div>
        </div>
      )}

      {tab === 'text' && (
        <div className="space-y-3">
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="标题（可选）"
            className="h-11 w-full rounded-xl border border-border bg-surface-2 px-3.5 text-sm outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
          />
          <textarea
            autoFocus
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="在此粘贴文本内容…"
            rows={8}
            className="scroll-thin w-full resize-none rounded-xl border border-border bg-surface-2 p-3.5 text-sm leading-6 outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
          />
          <div className="flex items-center justify-between">
            <span className="text-xs text-muted">{text.length} 字</span>
            <Button onClick={submitText} disabled={!text.trim()} icon={<FileText className="h-4 w-4" />}>插入</Button>
          </div>
        </div>
      )}

      <div className="mt-6 flex items-center gap-3 border-t border-border/60 pt-4 pb-2">
        <span className="shrink-0 text-xs text-muted">来源上限</span>
        <Progress value={(sources.length / MAX_SOURCES) * 100} />
        <span className="shrink-0 text-xs tabular-nums text-muted">{sources.length} / {MAX_SOURCES}</span>
      </div>
    </Modal>
  )
}

function Hint({ icon, title, text }: { icon: React.ReactNode; title: string; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-2xl bg-surface-2 p-3.5">
      <span className="shrink-0">{icon}</span>
      <div>
        <div className="text-sm font-medium">{title}</div>
        <div className="text-xs text-muted">{text}</div>
      </div>
    </div>
  )
}
