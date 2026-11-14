/**
 * 笔记本工作台状态：来源 / 对话 / 笔记 / 工作室产物
 */
import { create } from 'zustand'
import type { AudioOverview, Message, Note, Notebook, NotebookGuide, Source, StudioArtifact, StudioKind } from '@/types'
import { chatApi, notebooksApi, notesApi, sourcesApi, studioApi, type AddSourceInput } from '@/api'
import { uid } from '@/lib/utils'
import { toast } from './useUIStore'

type StudioView = { type: 'list' } | { type: 'note'; id: string } | { type: 'artifact'; id: string }

interface WorkspaceState {
  notebookId: string | null
  notebook: Notebook | null
  loading: boolean
  error: string | null

  sources: Source[]
  selectedIds: string[]
  openSourceId: string | null
  highlight: string | null

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
  openSource: (id: string | null, highlight?: string) => void
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

export const useWorkspace = create<WorkspaceState>((set, get) => ({
  notebookId: null,
  notebook: null,
  loading: true,
  error: null,
  sources: [],
  selectedIds: [],
  openSourceId: null,
  highlight: null,
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
    get().abort?.abort()
    set({
      notebookId: id, loading: true, error: null, notebook: null, sources: [], messages: [], notes: [], artifacts: [],
      openSourceId: null, studioView: { type: 'list' }, guide: null, generating: [], streaming: false,
    })
    try {
      const [notebook, sources, messages, notes, artifacts, audio] = await Promise.all([
        notebooksApi.get(id), sourcesApi.list(id), chatApi.history(id), notesApi.list(id), studioApi.list(id), studioApi.getAudio(id),
      ])
      if (get().notebookId !== id) return
      set({
        notebook, sources, messages, notes, artifacts, audio, loading: false,
        selectedIds: sources.filter((s) => s.status === 'ready').map((s) => s.id),
      })
      get().refreshGuide()
    } catch (e) {
      set({ loading: false, error: (e as Error).message })
    }
  },

  async renameNotebook(title) {
    const { notebookId } = get()
    if (!notebookId || !title.trim()) return
    const nb = await notebooksApi.update(notebookId, { title: title.trim() })
    set({ notebook: nb })
  },

  toggleSource: (id) =>
    set((s) => ({ selectedIds: s.selectedIds.includes(id) ? s.selectedIds.filter((x) => x !== id) : [...s.selectedIds, id] })),
  selectAll: (v) => set((s) => ({ selectedIds: v ? s.sources.filter((x) => x.status === 'ready').map((x) => x.id) : [] })),
  openSource: (id, highlight) => set({ openSourceId: id, highlight: highlight ?? null }),

  async addSource(input) {
    const nbId = get().notebookId
    if (!nbId) return
    let tempId: string | null = null
    try {
      const final = await sourcesApi.add(nbId, input, (src) => {
        if (get().notebookId !== nbId) return
        set((s) => {
          const exists = s.sources.some((x) => x.id === src.id || (src.id === 'pending' && x.id === tempId))
          if (!exists) {
            tempId = src.id
            return { sources: [...s.sources, src] }
          }
          return { sources: s.sources.map((x) => (x.id === src.id || x.id === tempId ? src : x)) }
        })
      })
      if (get().notebookId !== nbId) return
      set((s) => ({ selectedIds: [...new Set([...s.selectedIds, final.id])] }))
      toast(`「${final.title}」已就绪`, 'success')
      get().refreshGuide()
    } catch (e) {
      toast((e as Error).message, 'error')
    }
  },

  async renameSource(id, title) {
    await sourcesApi.rename(id, title)
    set((s) => ({ sources: s.sources.map((x) => (x.id === id ? { ...x, title } : x)) }))
  },

  async removeSource(id) {
    await sourcesApi.remove(id)
    set((s) => ({
      sources: s.sources.filter((x) => x.id !== id),
      selectedIds: s.selectedIds.filter((x) => x !== id),
      openSourceId: s.openSourceId === id ? null : s.openSourceId,
    }))
    toast('来源已删除')
    get().refreshGuide()
  },

  async refreshGuide() {
    const id = get().notebookId
    if (!id) return
    set({ guideLoading: true })
    const guide = await notebooksApi.guide(id)
    if (get().notebookId === id) set({ guide, guideLoading: false })
  },

  async ask(question) {
    const { notebookId, selectedIds, streaming } = get()
    if (!notebookId || streaming || !question.trim()) return
    const ctrl = new AbortController()
    const userMsg: Message = { id: uid('m'), role: 'user', content: question.trim(), createdAt: new Date().toISOString() }
    const aiId = uid('m')
    const aiMsg: Message = { id: aiId, role: 'assistant', content: '', streaming: true, createdAt: new Date().toISOString() }
    set((s) => ({ messages: [...s.messages, userMsg, aiMsg], streaming: true, abort: ctrl }))
    const patch = (fn: (m: Message) => Message) =>
      set((s) => ({ messages: s.messages.map((m) => (m.id === aiId ? fn(m) : m)) }))
    try {
      for await (const ev of chatApi.stream(notebookId, question.trim(), selectedIds, ctrl.signal)) {
        if (ev.type === 'delta') patch((m) => ({ ...m, content: m.content + ev.text }))
        else if (ev.type === 'citations') patch((m) => ({ ...m, citations: ev.citations }))
        else if (ev.type === 'done') patch((m) => ({ ...m, id: ev.messageId }))
      }
    } catch (e) {
      if ((e as Error).name !== 'AbortError') patch((m) => ({ ...m, content: m.content + '\n\n> ⚠️ 生成失败：' + (e as Error).message }))
    } finally {
      set((s) => ({
        streaming: false, abort: null,
        messages: s.messages.map((m) => (m.streaming ? { ...m, streaming: false } : m)),
      }))
    }
  },

  stop() {
    get().abort?.abort()
  },

  async clearChat() {
    const id = get().notebookId
    if (!id) return
    get().abort?.abort()
    await chatApi.clear(id)
    set({ messages: [] })
    toast('对话记录已清空')
  },

  feedback(id, v) {
    set((s) => ({ messages: s.messages.map((m) => (m.id === id ? { ...m, feedback: m.feedback === v ? undefined : v } : m)) }))
    chatApi.feedback(id, v)
  },

  setStudioView: (v) => set({ studioView: v }),

  async createNote(data, open = true) {
    const id = get().notebookId
    if (!id) return
    const note = await notesApi.create(id, { title: data?.title ?? '新建笔记', content: data?.content ?? '', origin: data?.origin ?? 'manual' })
    set((s) => ({ notes: [note, ...s.notes], ...(open ? { studioView: { type: 'note', id: note.id } } : {}) }))
    return note
  },

  async updateNote(id, patch) {
    set((s) => ({ notes: s.notes.map((n) => (n.id === id ? { ...n, ...patch, updatedAt: new Date().toISOString() } : n)) }))
    await notesApi.update(id, patch)
  },

  async removeNote(id) {
    await notesApi.remove(id)
    set((s) => ({ notes: s.notes.filter((n) => n.id !== id), studioView: { type: 'list' } }))
    toast('笔记已删除')
  },

  async generate(kind) {
    const id = get().notebookId
    if (!id || get().generating.includes(kind)) return
    if (!get().sources.some((s) => s.status === 'ready')) {
      toast('请先添加至少一个来源', 'error')
      return
    }
    set((s) => ({ generating: [...s.generating, kind] }))
    try {
      const art = await studioApi.generate(id, kind)
      if (get().notebookId !== id) return
      set((s) => ({ artifacts: [art, ...s.artifacts] }))
      toast(`${art.title}已生成`, 'success')
    } finally {
      set((s) => ({ generating: s.generating.filter((k) => k !== kind) }))
    }
  },

  async removeArtifact(id) {
    await studioApi.remove(id)
    set((s) => ({ artifacts: s.artifacts.filter((a) => a.id !== id), studioView: { type: 'list' } }))
  },

  async generateAudio() {
    const id = get().notebookId
    if (!id) return
    if (!get().sources.some((s) => s.status === 'ready')) {
      toast('请先添加至少一个来源', 'error')
      return
    }
    set({ audio: { notebookId: id, status: 'generating' } })
    const audio = await studioApi.generateAudio(id)
    if (get().notebookId === id) set({ audio })
  },
}))
