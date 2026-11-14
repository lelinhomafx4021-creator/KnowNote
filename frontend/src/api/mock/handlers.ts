/**
 * 模拟后端：内存数据 + 随机延迟。
 * 由 USE_MOCK 选择执行，与真实接口保持相同的领域类型。
 */
import type {
  Notebook, Source, Note, StudioArtifact, StudioKind, Message, Citation,
  ChatStreamEvent, NotebookGuide, AudioOverview, UserSettings, MindMapNode, SourceType,
} from '@/types'
import { sleep, uid } from '@/lib/utils'
import * as db from './data'

const now = () => new Date().toISOString()
const delay = (min = 150, max = 450) => sleep(min + Math.random() * (max - min))
const clone = <T,>(v: T): T => JSON.parse(JSON.stringify(v))

function touch(notebookId: string) {
  const nb = db.notebooks.find((n) => n.id === notebookId)
  if (nb) {
    nb.updatedAt = now()
    nb.sourceCount = db.sources.filter((s) => s.notebookId === notebookId).length
  }
}

/* ---------------- 笔记本 ---------------- */
export async function listNotebooks(): Promise<Notebook[]> {
  await delay()
  return clone(db.notebooks)
}
export async function getNotebook(id: string): Promise<Notebook> {
  await delay(80, 200)
  const nb = db.notebooks.find((n) => n.id === id)
  if (!nb) throw new Error('笔记本不存在')
  return clone(nb)
}
const EMOJIS = ['📘', '🧪', '🎯', '🌱', '🧩', '🚀', '📝', '🔭']
export async function createNotebook(title = '未命名笔记本'): Promise<Notebook> {
  await delay()
  const nb: Notebook = {
    id: uid('nb'), title, emoji: EMOJIS[Math.floor(Math.random() * EMOJIS.length)],
    color: Math.floor(Math.random() * 8), sourceCount: 0, createdAt: now(), updatedAt: now(),
  }
  db.notebooks.unshift(nb)
  db.messages[nb.id] = []
  return clone(nb)
}
export async function updateNotebook(id: string, patch: Partial<Notebook>): Promise<Notebook> {
  await delay(80, 200)
  const nb = db.notebooks.find((n) => n.id === id)
  if (!nb) throw new Error('笔记本不存在')
  Object.assign(nb, patch, { updatedAt: now() })
  return clone(nb)
}
export async function deleteNotebook(id: string): Promise<void> {
  await delay()
  const i = db.notebooks.findIndex((n) => n.id === id)
  if (i >= 0) db.notebooks.splice(i, 1)
}

/* ---------------- 来源 ---------------- */
export async function listSources(notebookId: string): Promise<Source[]> {
  await delay()
  return clone(db.sources.filter((s) => s.notebookId === notebookId))
}

export interface AddSourceInput {
  type: SourceType
  title: string
  size?: number
  url?: string
  text?: string
}

export async function getSource(id: string): Promise<Source> {
  const source = db.sources.find((s) => s.id === id)
  if (!source) throw new Error('来源不存在')
  return clone(source)
}

/** 模拟上传 + 解析：通过 onProgress 回调推送状态变化 */
export async function addSource(
  notebookId: string,
  input: AddSourceInput,
  onProgress?: (s: Source) => void,
  signal?: AbortSignal,
): Promise<Source> {
  const src: Source = {
    id: uid('src'), notebookId, type: input.type, title: input.title, size: input.size, url: input.url,
    status: input.type === 'web' || input.type === 'youtube' || input.type === 'text' ? 'processing' : 'uploading',
    progress: 0, createdAt: now(),
  }
  db.sources.push(src)
  touch(notebookId)
  if (!signal?.aborted) onProgress?.(clone(src))

  if (src.status === 'uploading') {
    for (let p = 10; p <= 100; p += 15 + Math.random() * 15) {
      await sleep(180)
      src.progress = Math.min(100, Math.round(p))
      if (!signal?.aborted) onProgress?.(clone(src))
    }
    src.status = 'processing'
    src.progress = 100
    if (!signal?.aborted) onProgress?.(clone(src))
  }
  await sleep(1200 + Math.random() * 800)
  const text = input.text?.trim()
  Object.assign(src, {
    status: 'ready',
    wordCount: text ? text.length : 1500 + Math.floor(Math.random() * 6000),
    summary: text
      ? `这段文本主要讨论了：${text.slice(0, 60)}${text.length > 60 ? '……' : ''}。系统已完成分段与向量化，可在对话中引用。`
      : `系统已完成对「${input.title}」的解析：抽取正文、切分为语义片段并生成向量索引。该资料涉及的核心概念已整理为下方关键主题，可在对话中直接提问。`,
    topics: ['核心概念', '方法论', '实践案例', '结论'],
    content: text || '（原文预览）已解析的正文内容将在此显示。接入后端后，这里展示文档切分后的前若干段落。',
  })
  touch(notebookId)
  if (!signal?.aborted) onProgress?.(clone(src))
  return clone(src)
}

export async function renameSource(id: string, title: string): Promise<void> {
  await delay(60, 120)
  const s = db.sources.find((x) => x.id === id)
  if (s) s.title = title
}
export async function deleteSource(id: string): Promise<void> {
  await delay(80, 200)
  const i = db.sources.findIndex((s) => s.id === id)
  if (i >= 0) {
    const nbId = db.sources[i].notebookId
    db.sources.splice(i, 1)
    touch(nbId)
  }
}

/* ---------------- 笔记本概要（Notebook Guide） ---------------- */
export async function getGuide(notebookId: string): Promise<NotebookGuide> {
  await delay(300, 700)
  const srcs = db.sources.filter((s) => s.notebookId === notebookId && s.status === 'ready')
  if (!srcs.length) return { summary: '', suggestedQuestions: [] }
  if (notebookId === 'nb_1') {
    return {
      summary:
        '这些资料围绕**检索增强生成（RAG）**展开：从 Lewis 等人的原始论文出发，介绍了将参数化记忆与非参数化记忆结合的方法；工程文章补充了切分、向量化、检索与重排序的完整流水线；开题报告则将其落地为一个**基于大语言模型的个人资料管理系统**。Karpathy 的讲座提供了理解 LLM 训练与安全问题的宏观背景。',
      suggestedQuestions: ['RAG 和模型微调有什么区别？', '我的系统应该如何设计文档切分策略？', '总结一下开题报告的技术路线'],
    }
  }
  return {
    summary: `本笔记本包含 ${srcs.length} 份资料，主要涉及 ${[...new Set(srcs.flatMap((s) => s.topics ?? []))].slice(0, 5).join('、')} 等主题。`,
    suggestedQuestions: ['帮我总结这些资料的核心观点', '这些资料之间有什么联系？', '列出需要重点掌握的概念'],
  }
}

/* ---------------- 对话（流式） ---------------- */
export async function listMessages(notebookId: string): Promise<Message[]> {
  await delay(100, 250)
  return clone(db.messages[notebookId] ?? [])
}
export async function clearMessages(notebookId: string): Promise<void> {
  await delay(80, 150)
  db.messages[notebookId] = []
}

function cite(index: number, s: Source, quote?: string): Citation {
  const q = quote ?? (s.content ?? s.summary ?? '').replace(/\n+/g, ' ').slice(0, 120) + '…'
  return { index, sourceId: s.id, sourceTitle: s.title, quote: q }
}

function composeAnswer(question: string, srcs: Source[]): { text: string; citations: Citation[] } {
  if (!srcs.length) {
    return { text: '当前没有选中任何来源。请在左侧勾选至少一个来源，我会只根据你的资料来回答，并标注引用出处。', citations: [] }
  }
  const [a, b = srcs[0], c = srcs[srcs.length - 1]] = srcs
  const citations = [cite(1, a), cite(2, b), cite(3, c)]

  if (/微调|区别|fine/i.test(question)) {
    return {
      citations,
      text: `根据你的资料，**RAG** 与 **模型微调** 是两种向大模型注入知识的不同思路：

## 核心区别
| 维度 | RAG（检索增强生成） | 微调（Fine-tuning） |
|---|---|---|
| 知识存放 | 外部向量索引（非参数化记忆） | 模型权重（参数化记忆） |
| 知识更新 | 增删文档即可，实时生效 | 需重新训练 |
| 可解释性 | 可给出引用来源 | 难以追溯 |

RAG 将预训练 seq2seq 模型与维基百科稠密向量索引结合，在知识密集型任务上取得了更具体、更符合事实的生成效果 [1]。在工程上，它依赖“切分 → 向量化 → 检索 → 生成”的流水线 [2]。

## 对你毕设的启示
你的开题报告将系统定位为**个人资料管理**，资料会被频繁增删，因此选择 RAG 而非微调更合理 [3]。`,
    }
  }
  if (/切分|chunk|分块/i.test(question)) {
    return {
      citations,
      text: `关于文档切分（Chunking），资料中给出了以下建议：

1. **片段长度**：一般将文档切分为 300~800 字的片段，过短会丢失上下文，过长会稀释语义 [2]。
2. **重叠窗口**：相邻片段保留 10%~20% 的重叠，减少关键句被切断的概率 [2]。
3. **按结构切分**：优先依据标题、段落等文档结构进行切分，再对超长段落做二次切分。
4. **检索粒度与生成配合**：原始 RAG 论文中，每个检索单元是 100 词的维基百科段落 [1]。

> 建议在系统中把切分参数做成可配置项，并在论文实验部分对比不同参数下的召回率 [3]。`,
    }
  }
  return {
    citations,
    text: `结合你选中的 ${srcs.length} 份资料，针对「${question}」整理如下：

**要点概括**
- 「${a.title.replace(/\.(pdf|docx|md)$/, '')}」指出：${(a.summary ?? '').slice(0, 70)}… [1]
- 另一份资料补充了实践层面的细节：${(b.summary ?? '').slice(0, 60)}… [2]
- 综合来看，这些内容可以相互印证，形成从理论到落地的完整链条 [3]。

**下一步建议**
1. 将上述要点保存为笔记，便于后续复习；
2. 在右侧「工作室」中生成**思维导图**，梳理概念之间的层级关系。`,
  }
}

/**
 * 流式问答。真实实现应调用 POST /api/notebooks/:id/chat（SSE），
 * 逐条产出 delta / citations / done 事件。
 */
export async function* streamChat(
  notebookId: string,
  question: string,
  sourceIds: string[],
  signal?: AbortSignal,
): AsyncGenerator<ChatStreamEvent> {
  const list = (db.messages[notebookId] ??= [])
  list.push({ id: uid('msg'), role: 'user', content: question, createdAt: now() })
  const srcs = db.sources.filter((s) => sourceIds.includes(s.id) && s.status === 'ready')
  const { text, citations } = composeAnswer(question, srcs)

  await sleep(600)
  if (signal?.aborted) return
  yield { type: 'citations', citations }
  let out = ''
  const chunks = text.match(/[\s\S]{1,3}/g) ?? []
  for (const ch of chunks) {
    if (signal?.aborted) break
    await sleep(12 + Math.random() * 18)
    if (signal?.aborted) break
    out += ch
    yield { type: 'delta', text: ch }
  }
  const id = uid('msg')
  list.push({ id, role: 'assistant', content: out, citations, createdAt: now() })
  yield { type: 'done', messageId: id }
}

export async function feedbackMessage(id: string, value: 'up' | 'down' | null): Promise<void> {
  for (const list of Object.values(db.messages)) {
    const message = list.find((m) => m.id === id)
    if (message) message.feedback = value
  }
}

/* ---------------- 笔记 ---------------- */
export async function listNotes(notebookId: string): Promise<Note[]> {
  await delay()
  return clone(db.notes.filter((n) => n.notebookId === notebookId).sort((a, b) => b.updatedAt.localeCompare(a.updatedAt)))
}
export async function createNote(notebookId: string, data: Pick<Note, 'title' | 'content' | 'origin'>): Promise<Note> {
  await delay(80, 200)
  const n: Note = { id: uid('note'), notebookId, ...data, createdAt: now(), updatedAt: now() }
  db.notes.push(n)
  return clone(n)
}
export async function updateNote(id: string, patch: Partial<Pick<Note, 'title' | 'content'>>): Promise<Note> {
  await delay(60, 150)
  const n = db.notes.find((x) => x.id === id)
  if (!n) throw new Error('笔记不存在')
  Object.assign(n, patch, { updatedAt: now() })
  return clone(n)
}
export async function deleteNote(id: string): Promise<void> {
  await delay(60, 150)
  const i = db.notes.findIndex((n) => n.id === id)
  if (i >= 0) db.notes.splice(i, 1)
}

/* ---------------- 工作室 ---------------- */
export const STUDIO_TITLES: Record<StudioKind, string> = {
  'study-guide': '学习指南', briefing: '简报文档', faq: '常见问题', timeline: '时间线', mindmap: '思维导图',
}

export async function listArtifacts(notebookId: string): Promise<StudioArtifact[]> {
  await delay()
  return clone(db.artifacts.filter((a) => a.notebookId === notebookId))
}

function buildMindmap(notebookId: string): MindMapNode {
  const nb = db.notebooks.find((n) => n.id === notebookId)
  const srcs = db.sources.filter((s) => s.notebookId === notebookId && s.status === 'ready')
  if (notebookId === 'nb_1') {
    return {
      id: 'root', label: 'RAG 个人资料管理系统',
      children: [
        { id: 'a', label: 'RAG 原理', children: [
          { id: 'a1', label: '参数化记忆 (seq2seq)' }, { id: 'a2', label: '非参数化记忆 (向量索引)' },
          { id: 'a3', label: 'RAG-Sequence / RAG-Token' } ] },
        { id: 'b', label: '工程流水线', children: [
          { id: 'b1', label: '文档切分' }, { id: 'b2', label: 'Embedding' }, { id: 'b3', label: '向量数据库' },
          { id: 'b4', label: '检索与重排序' } ] },
        { id: 'c', label: '系统功能', children: [
          { id: 'c1', label: '多格式资料导入' }, { id: 'c2', label: '带引用的智能问答' },
          { id: 'c3', label: '摘要 / FAQ / 思维导图' } ] },
        { id: 'd', label: 'LLM 背景', children: [
          { id: 'd1', label: '预训练与 RLHF' }, { id: 'd2', label: '提示注入与安全' } ] },
      ],
    }
  }
  return {
    id: 'root', label: nb?.title ?? '主题',
    children: srcs.map((s) => ({
      id: s.id, label: s.title.replace(/\.(pdf|docx|md)$/, '').slice(0, 18),
      children: (s.topics ?? []).slice(0, 4).map((t, i) => ({ id: `${s.id}_${i}`, label: t })),
    })),
  }
}

function buildContent(kind: StudioKind, srcs: Source[]): string {
  const topics = [...new Set(srcs.flatMap((s) => s.topics ?? []))]
  switch (kind) {
    case 'study-guide':
      return `## 学习目标\n${topics.slice(0, 4).map((t) => `- 理解并能解释 **${t}**`).join('\n')}\n\n## 简答题\n${topics.slice(0, 5).map((t, i) => `${i + 1}. 请简述「${t}」的含义及其作用。`).join('\n')}\n\n## 论述题\n1. 结合资料，论述各核心概念之间的关系。\n2. 如果要将这些方法应用到实际系统中，需要注意哪些问题？\n\n## 术语表\n${topics.map((t) => `- **${t}**：见相关来源中的定义。`).join('\n')}`
    case 'briefing':
      return `## 执行摘要\n本简报基于 ${srcs.length} 份资料生成，概括了其中的核心主题与关键结论。\n\n## 核心主题\n${srcs.map((s) => `### ${s.title.replace(/\.(pdf|docx|md)$/, '')}\n${s.summary ?? ''}`).join('\n\n')}\n\n## 结论与建议\n- 各资料在核心观点上相互印证；\n- 建议优先关注：${topics.slice(0, 3).join('、')}。`
    case 'faq':
      return topics.slice(0, 6).map((t) => `### ${t}是什么？\n根据资料，${t}是本笔记本中反复出现的关键概念，相关内容可参见对应来源的摘要与原文。`).join('\n\n')
    case 'timeline':
      return `## 时间线\n| 时间 | 事件 |\n|---|---|\n| 2017 | Transformer 架构提出 |\n| 2019 | DPR 稠密段落检索被提出 |\n| 2020 | RAG 论文发表，提出检索增强生成框架 |\n| 2022 | ChatGPT 发布，LLM 进入大众视野 |\n| 2023 | 向量数据库与 RAG 工程化快速普及 |\n| 2026 | 本毕业设计：基于 LLM 的个人资料管理系统 |\n\n## 人物表\n- **Patrick Lewis**：RAG 论文第一作者\n- **Andrej Karpathy**：《Intro to LLMs》讲者`
    default:
      return ''
  }
}

export async function generateArtifact(notebookId: string, kind: StudioKind): Promise<StudioArtifact> {
  await sleep(1800 + Math.random() * 1200)
  const srcs = db.sources.filter((s) => s.notebookId === notebookId && s.status === 'ready')
  const a: StudioArtifact = {
    id: uid('art'), notebookId, kind, title: STUDIO_TITLES[kind], createdAt: now(),
    ...(kind === 'mindmap' ? { mindmap: buildMindmap(notebookId) } : { content: buildContent(kind, srcs) }),
  }
  db.artifacts.unshift(a)
  return clone(a)
}
export async function deleteArtifact(id: string): Promise<void> {
  await delay(60, 120)
  const i = db.artifacts.findIndex((a) => a.id === id)
  if (i >= 0) db.artifacts.splice(i, 1)
}

/* ---------------- 音频概览 ---------------- */
const audios: Record<string, AudioOverview> = {}
export async function getAudio(notebookId: string): Promise<AudioOverview> {
  await delay(80, 160)
  return clone(audios[notebookId] ?? { notebookId, status: 'idle' })
}
export async function generateAudio(notebookId: string): Promise<AudioOverview> {
  audios[notebookId] = { notebookId, status: 'generating' }
  await sleep(3500)
  const nb = db.notebooks.find((n) => n.id === notebookId)
  audios[notebookId] = { notebookId, status: 'ready', title: `深度对谈：${nb?.title ?? ''}`, transcript: `主持人甲：欢迎来到知页。今天我们一起聊聊「${nb?.title ?? '学习资料'}」。\n\n主持人乙：我们可以先梳理核心概念，再结合来源中的例子理解它们。\n\n主持人甲：带着问题回到原文，能够帮助我们建立自己的知识结构。` }
  return clone(audios[notebookId])
}

/* ---------------- 用户设置 ---------------- */
export async function getSettings(): Promise<UserSettings> {
  await delay(80, 160)
  return clone(db.settings)
}
export async function updateSettings(patch: Partial<UserSettings>): Promise<UserSettings> {
  await delay(80, 160)
  Object.assign(db.settings, patch)
  return clone(db.settings)
}
