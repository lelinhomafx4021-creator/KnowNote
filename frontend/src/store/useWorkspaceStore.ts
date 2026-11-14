/**
 * 笔记本工作台状态：来源 / 对话 / 笔记 / 工作室产物
 */
import { create } from 'zustand'
import type { AudioOverview, Citation, Message, Note, Notebook, NotebookGuide, Source, StudioArtifact, StudioKind } from '@/types'
import { chatApi, notebooksApi, notesApi, sourcesApi, studioApi, type AddSourceInput } from '@/api'
import { isAbort, reportError } from '@/api/client'
import { uid } from '@/lib/utils'
import { toast } from './useUIStore'

type StudioView = { type: 'list' } | { type: 'note'; id: string } | { type: 'artifact'; id: string }

interface WorkspaceState {
  session: AbortController | null
  moduleErrors: string[]
  reset: () => void
  notebookId: string | null
  notebook: Notebook | null
  loading: boolean
  error: string | null

  sources: Source[]
  selectedIds: string[]
  openSourceId: string | null
  highlight: string | null
  citationLocation: Pick<Citation, 'page' | 'locationLabel'> | null
  preview: { sourceId: string; page: number } | null

  guide: NotebookGuide | null
  guideLoading: boolean

  messages: Message[]
  streaming: boolean
  abort: AbortController | null

  notes: Note[]
  artifacts: StudioArtifact[]
  generating: StudioKind[]
  audio: AudioOverview
  studioView: StudioView

  load: (id: string) => Promise<void>
  renameNotebook: (title: string) => Promise<void>

  toggleSource: (id: string) => void
  selectAll: (v: boolean) => void
  openSource: (id: string | null, highlight?: string, location?: Pick<Citation, 'page' | 'locationLabel'>) => void
  openPreview: (id: string, page?: number) => void
  closePreview: () => void
  addSource: (input: AddSourceInput & { file?: File }) => Promise<void>
  renameSource: (id: string, title: string) => Promise<void>
  removeSource: (id: string) => Promise<void>
  refreshGuide: () => Promise<void>

  ask: (question: string) => Promise<void>
  stop: () => void
  clearChat: () => Promise<void>
  feedback: (id: string, v: 'up' | 'down') => void

  setStudioView: (v: StudioView) => void
  createNote: (data?: Partial<Pick<Note, 'title' | 'content' | 'origin'>>, open?: boolean) => Promise<Note | undefined>
  updateNote: (id: string, patch: Partial<Pick<Note, 'title' | 'content'>>) => Promise<void>
  removeNote: (id: string) => Promise<void>
  generate: (kind: StudioKind) => Promise<void>
  removeArtifact: (id: string) => Promise<void>
  generateAudio: () => Promise<void>
}

const sourceTasks = new Map<string, AbortController>()

export const useWorkspace = create<WorkspaceState>((set, get) => ({
  session: null,
  moduleErrors: [],
  reset() {
    get().session?.abort()
    get().abort?.abort()
    sourceTasks.forEach((task) => task.abort())
    sourceTasks.clear()
    set({ session: null, notebookId: null, notebook: null, sources: [], selectedIds: [], openSourceId: null, highlight: null, citationLocation: null, preview: null, messages: [], streaming: false, abort: null, notes: [], artifacts: [], generating: [], audio: { notebookId: '', status: 'idle' }, guide: null, guideLoading: false, loading: false, error: null, moduleErrors: [], studioView: { type: 'list' } })
  },
  notebookId: null,
  notebook: null,
  loading: true,
  error: null,
  sources: [],
  selectedIds: [],
  openSourceId: null,
  highlight: null,
  citationLocation: null,
  preview: null,
  guide: null,
  guideLoading: false,
  messages: [],
  streaming: false,
  abort: null,
  notes: [],
  artifacts: [],
  generating: [],
  audio: { notebookId: '', status: 'idle' },
  studioView: { type: 'list' },

  async load(id) {
    get().reset()
    const session = new AbortController()
    const active = () => get().session === session && !session.signal.aborted
    set({ session, notebookId: id, loading: true, error: null, audio: { notebookId: id, status: 'idle' } })
    try {
      const [notebook, results] = await Promise.all([
        notebooksApi.get(id),
        Promise.allSettled([sourcesApi.list(id, session.signal), chatApi.history(id), notesApi.list(id), studioApi.list(id), studioApi.getAudio(id, session.signal)]),
      ])
      if (!active()) return
      const [sourceResult, messageResult, noteResult, artifactResult, audioResult] = results
      const sources = sourceResult.status === 'fulfilled' ? sourceResult.value : []
      const audio: AudioOverview = audioResult.status === 'fulfilled' ? audioResult.value : { notebookId: id, status: 'error' }
      const labels = ['来源', '对话记录', '笔记', '工作室内容', '音频概览']
      set({ notebook, sources, messages: messageResult.status === 'fulfilled' ? messageResult.value : [], notes: noteResult.status === 'fulfilled' ? noteResult.value : [], artifacts: artifactResult.status === 'fulfilled' ? artifactResult.value : [], audio, loading: false, selectedIds: sources.filter((s) => s.status === 'ready').map((s) => s.id), moduleErrors: results.flatMap((r, i) => r.status === 'rejected' ? [labels[i]] : []) })
      void get().refreshGuide()
      for (const source of sources.filter((s) => s.status === 'processing' || s.status === 'uploading')) {
        const task = new AbortController()
        sourceTasks.set(source.id, task)
        const abort = () => task.abort()
        session.signal.addEventListener('abort', abort, { once: true })
        void sourcesApi.poll(source, (updated) => {
          if (!active() || task.signal.aborted) return
          set((s) => ({ sources: s.sources.map((x) => x.id === updated.id ? updated : x), selectedIds: updated.status === 'ready' ? [...new Set([...s.selectedIds, updated.id])] : s.selectedIds }))
          if (updated.status === 'ready') void get().refreshGuide()
        }, task.signal).catch((e) => {
          if (active() && !isAbort(e)) {
            reportError(e)
            set((s) => ({ sources: s.sources.map((x) => x.id === source.id ? { ...x, status: 'error', error: (e as Error).message } : x) }))
          }
        }).finally(() => { session.signal.removeEventListener('abort', abort); if (sourceTasks.get(source.id) === task) sourceTasks.delete(source.id) })
      }
      if (audio.status === 'generating') void studioApi.pollAudio(audio, (updated) => { if (active()) set({ audio: updated }) }, session.signal).catch((e) => {
        if (active() && !isAbort(e)) { reportError(e); set({ audio: { notebookId: id, status: 'error' } }) }
      })
    } catch (e) {
      if (active()) set({ loading: false, error: reportError(e).message })
    }
  },

  async renameNotebook(title) {
    const { notebookId } = get()
    if (!notebookId || !title.trim()) return
    try {
      const nb = await notebooksApi.update(notebookId, { title: title.trim() })
      if (get().notebookId === notebookId) set({ notebook: nb })
    } catch (e) { reportError(e) }
  },

  toggleSource: (id) =>
    set((s) => ({ selectedIds: s.selectedIds.includes(id) ? s.selectedIds.filter((x) => x !== id) : [...s.selectedIds, id] })),
  selectAll: (v) => set((s) => ({ selectedIds: v ? s.sources.filter((x) => x.status === 'ready').map((x) => x.id) : [] })),
  openSource: (id, highlight, location) => set({ openSourceId: id, highlight: highlight ?? null, citationLocation: location ?? null, preview: null }),
  openPreview: (id, page = 1) => {
    const source = get().sources.find((s) => s.id === id)
    if (!source?.previewable) return
    const target = Math.max(1, Math.min(source.pageCount || Infinity, Math.floor(page) || 1))
    set({ preview: { sourceId: id, page: target } })
  },
  closePreview: () => set({ preview: null }),

  async addSource(input) {
    const { notebookId: nbId, session } = get()
    if (!nbId || !session) return
    const active = () => get().session === session && !session.signal.aborted
    const task = new AbortController()
    const abort = () => task.abort()
    session.signal.addEventListener('abort', abort, { once: true })
    let rowId = uid('pending')
    sourceTasks.set(rowId, task)
    const temporary: Source = { id: rowId, notebookId: nbId, type: input.type, title: input.title, size: input.size, status: input.file ? 'uploading' : 'processing', progress: 0, createdAt: new Date().toISOString() }
    set((s) => ({ sources: [...s.sources, temporary] }))
    const update = (source: Source) => {
      if (!active() || task.signal.aborted) return
      const next = source.id === 'pending' ? { ...source, id: rowId } : source
      set((s) => ({ sources: s.sources.map((x) => x.id === rowId ? next : x) }))
      sourceTasks.delete(rowId)
      rowId = next.id
      sourceTasks.set(rowId, task)
    }
    try {
      const final = await sourcesApi.add(nbId, input, update, task.signal)
      if (!active() || task.signal.aborted) return
      update(final)
      if (final.status === 'ready') {
        set((s) => ({ selectedIds: [...new Set([...s.selectedIds, final.id])] }))
        toast(`「${final.title}」已就绪`, 'success')
        void get().refreshGuide()
      } else if (final.status === 'error') toast(final.error || '来源处理失败，可删除后重试', 'error')
    } catch (e) {
      if (active() && !isAbort(e)) {
        const error = reportError(e)
        set((s) => ({ sources: s.sources.map((x) => x.id === rowId ? { ...x, status: 'error', error: error.message } : x) }))
      }
    } finally {
      session.signal.removeEventListener('abort', abort)
      if (sourceTasks.get(rowId) === task) sourceTasks.delete(rowId)
    }
  },

  async renameSource(id, title) {
    try {
      await sourcesApi.rename(id, title)
      set((s) => ({ sources: s.sources.map((x) => x.id === id ? { ...x, title } : x) }))
    } catch (e) { reportError(e) }
  },

  async removeSource(id) {
    try {
      if (!id.startsWith('pending_')) await sourcesApi.remove(id)
      sourceTasks.get(id)?.abort()
      sourceTasks.delete(id)
      set((s) => ({ sources: s.sources.filter((x) => x.id !== id), selectedIds: s.selectedIds.filter((x) => x !== id), openSourceId: s.openSourceId === id ? null : s.openSourceId, ...(s.openSourceId === id ? { highlight: null, citationLocation: null } : {}), preview: s.preview?.sourceId === id ? null : s.preview }))
      toast('来源已删除')
      void get().refreshGuide()
    } catch (e) { reportError(e) }
  },

  async refreshGuide() {
    const { notebookId: id, session } = get()
    if (!id) return
    set({ guideLoading: true })
    try {
      const guide = await notebooksApi.guide(id)
      if (get().session === session) set({ guide, guideLoading: false })
    } catch (e) {
      if (get().session === session) { reportError(e); set({ guide: null, guideLoading: false }) }
    }
  },

  async ask(question) {
    const { notebookId, selectedIds, streaming } = get()
    if (!notebookId || streaming || !question.trim()) return
    if (!selectedIds.length) { toast('请先选择至少一个已就绪来源', 'error'); return }
    const ctrl = new AbortController()
    const userMsg: Message = { id: uid('m'), role: 'user', content: question.trim(), createdAt: new Date().toISOString() }
    let aiId = uid('m')
    const aiMsg: Message = { id: aiId, role: 'assistant', content: '', streaming: true, createdAt: new Date().toISOString() }
    set((s) => ({ messages: [...s.messages, userMsg, aiMsg], streaming: true, abort: ctrl }))
    const patch = (fn: (m: Message) => Message) => {
      if (get().abort === ctrl && !ctrl.signal.aborted) set((s) => ({ messages: s.messages.map((m) => m.id === aiId ? fn(m) : m) }))
    }
    try {
      for await (const ev of chatApi.stream(notebookId, question.trim(), selectedIds, ctrl.signal)) {
        if (ctrl.signal.aborted || get().abort !== ctrl) break
        if (ev.type === 'delta') patch((m) => ({ ...m, content: m.content + ev.text }))
        else if (ev.type === 'citations') patch((m) => ({ ...m, citations: ev.citations }))
        else if (ev.type === 'done') { patch((m) => ({ ...m, id: ev.messageId })); aiId = ev.messageId }
        else if (ev.type === 'error') throw new Error(ev.message)
      }
    } catch (e) {
      if (!isAbort(e) && get().abort === ctrl) {
        reportError(e)
        patch((m) => ({ ...m, content: m.content + '\n\n> 生成失败：' + (e as Error).message }))
      }
    } finally {
      if (get().abort === ctrl) set((s) => ({ streaming: false, abort: null, messages: s.messages.map((m) => m.id === aiId ? { ...m, streaming: false } : m) }))
    }
  },

  stop() {
    get().abort?.abort()
    set((s) => ({ streaming: false, abort: null, messages: s.messages.map((m) => m.streaming ? { ...m, streaming: false, content: m.content || '已停止生成。' } : m) }))
  },

  async clearChat() {
    const id = get().notebookId
    if (!id) return
    get().stop()
    try { await chatApi.clear(id); if (get().notebookId === id) set({ messages: [] }); toast('对话记录已清空') }
    catch (e) { reportError(e) }
  },

  feedback(id, v) {
    const message = get().messages.find((m) => m.id === id)
    if (!message) return
    if (id.startsWith('m_')) { toast('请刷新对话后再反馈这条回答'); return }
    const value = message.feedback === v ? null : v
    set((s) => ({ messages: s.messages.map((m) => m.id === id ? { ...m, feedback: value } : m) }))
    void chatApi.feedback(id, value).catch((e) => {
      reportError(e)
      set((s) => ({ messages: s.messages.map((m) => m.id === id && m.feedback === value ? { ...m, feedback: message.feedback } : m) }))
    })
  },

  setStudioView: (v) => set({ studioView: v }),

  async createNote(data, open = true) {
    const { notebookId: id, session } = get()
    if (!id) return
    try {
      const note = await notesApi.create(id, { title: data?.title ?? '新建笔记', content: data?.content ?? '', origin: data?.origin ?? 'manual' })
      if (get().session !== session) return
      set((s) => ({ notes: [note, ...s.notes], ...(open ? { studioView: { type: 'note', id: note.id } as StudioView } : {}) }))
      return note
    } catch (e) { reportError(e) }
  },

  async updateNote(id, patch) {
    const note = await notesApi.update(id, patch)
    set((s) => ({ notes: s.notes.map((n) => n.id === id ? note : n) }))
  },

  async removeNote(id) {
    try {
      await notesApi.remove(id)
      set((s) => ({ notes: s.notes.filter((n) => n.id !== id), studioView: { type: 'list' } }))
      toast('笔记已删除')
    } catch (e) { reportError(e) }
  },

  async generate(kind) {
    const { notebookId: id, session } = get()
    if (!id || get().generating.includes(kind)) return
    if (!get().sources.some((s) => s.status === 'ready')) {
      toast('请先添加至少一个来源', 'error')
      return
    }
    set((s) => ({ generating: [...s.generating, kind] }))
    try {
      const art = await studioApi.generate(id, kind)
      if (get().session !== session) return
      set((s) => ({ artifacts: [art, ...s.artifacts] }))
      toast(`${art.title}已生成`, 'success')
    } catch (e) { reportError(e) } finally {
      if (get().session === session) set((s) => ({ generating: s.generating.filter((k) => k !== kind) }))
    }
  },

  async removeArtifact(id) {
    try {
      await studioApi.remove(id)
      set((s) => ({ artifacts: s.artifacts.filter((a) => a.id !== id), studioView: { type: 'list' } }))
    } catch (e) { reportError(e) }
  },

  async generateAudio() {
    const { notebookId: id, session, audio } = get()
    if (!id || !session || audio.status === 'generating') return
    if (!get().sources.some((s) => s.status === 'ready')) { toast('请先添加至少一个来源', 'error'); return }
    const active = () => get().session === session && !session.signal.aborted
    set({ audio: { notebookId: id, status: 'generating' } })
    try {
      const audio = await studioApi.generateAudio(id, (updated) => { if (active()) set({ audio: updated }) }, session.signal)
      if (active() && audio.status === 'error') toast('音频概览生成失败，请重试', 'error')
    } catch (e) {
      if (active() && !isAbort(e)) { reportError(e); set({ audio: { notebookId: id, status: 'error' } }) }
    }
  },
}))
