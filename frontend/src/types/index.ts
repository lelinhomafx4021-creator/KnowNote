/** ========== 领域模型（与后端接口约定保持一致） ========== */

export type SourceType = 'pdf' | 'docx' | 'pptx' | 'xlsx' | 'csv' | 'web' | 'text' | 'markdown' | 'audio' | 'youtube'
export type SourceStatus = 'uploading' | 'processing' | 'ready' | 'error'

export interface User {
  id: string
  email: string
  nickname: string
  avatarUrl?: string
  createdAt: string
}

export interface Tag { id: string; name: string; color: string; createdAt: string }
export interface LoginLog { id: string; email: string; success: boolean; ip: string; userAgent: string; createdAt: string }

export interface Notebook {
  id: string
  title: string
  emoji: string
  /** 封面色调，0-7 */
  color: number
  description?: string
  tags: Tag[]
  sourceCount: number
  createdAt: string
  updatedAt: string
}

export interface Source {
  id: string
  notebookId: string
  type: SourceType
  title: string
  status: SourceStatus
  /** 0-100，上传 / 解析进度 */
  progress?: number
  size?: number
  url?: string
  /** AI 生成的来源摘要 */
  summary?: string
  /** AI 提取的关键主题 */
  topics?: string[]
  /** 原文预览（前若干段） */
  content?: string
  error?: string
  wordCount?: number
  /** PDF 页数 / 幻灯片张数 / 工作表数 */
  pageCount?: number
  sections?: SourceSection[]
  /** 后端已提供原件的 PDF 预览 */
  previewable?: boolean
  originalExt?: string
  createdAt: string
}

export interface SourceSection {
  /** 页码 / 幻灯片序号 / 工作表序号，从 1 开始 */
  page: number
  label: string
  title?: string
  excerpt: string
}

export interface Citation {
  /** 回答中的角标序号，从 1 开始 */
  index: number
  sourceId: string
  sourceTitle: string
  /** 引用的原文片段 */
  quote: string
  page?: number
  locationLabel?: string
}

export type Role = 'user' | 'assistant'

export interface Message {
  id: string
  role: Role
  content: string
  citations?: Citation[]
  createdAt: string
  feedback?: 'up' | 'down' | null
  /** 前端状态：是否仍在流式输出 */
  streaming?: boolean
}

export interface Note {
  id: string
  notebookId: string
  title: string
  content: string
  /** 来源：手写 / 对话保存 / 工作室生成 */
  origin: 'manual' | 'chat' | 'studio'
  createdAt: string
  updatedAt: string
}

export type StudioKind = 'study-guide' | 'briefing' | 'faq' | 'timeline' | 'mindmap'

export interface MindMapNode {
  id: string
  label: string
  children?: MindMapNode[]
}

export interface StudioArtifact {
  id: string
  notebookId: string
  kind: StudioKind
  title: string
  /** Markdown 正文（mindmap 时为空） */
  content?: string
  mindmap?: MindMapNode
  createdAt: string
}

export interface AudioOverview {
  notebookId: string
  status: 'idle' | 'generating' | 'ready' | 'error'
  title?: string
  transcript?: string
  durationSec?: number
  url?: string
}

export interface NotebookGuide {
  summary: string
  suggestedQuestions: string[]
}

export type AnswerStyle = 'concise' | 'detailed' | 'learning'

export interface UserSettings {
  model: string
  answerStyle: AnswerStyle
  language: 'zh-CN' | 'en-US'
  storageUsed: number
  storageTotal: number
}

/** 流式对话事件（对应后端 SSE） */
export type ChatStreamEvent =
  | { type: 'delta'; text: string }
  | { type: 'citations'; citations: Citation[] }
  | { type: 'done'; messageId: string }
  | { type: 'error'; message: string }

export interface ModelCatalog {
  chat: { id: string; name: string; provider: string }[]
  embed: { model: string; dimension: number; provider: string }
  tts: { enabled: boolean }
  image: { enabled: boolean; provider: string; model: string }
}
export interface DailyUsage {
  date: string
  chatRequests: number
  tokens: number
  reservedTokens: number
  chatRequestsLimit: number
  tokensLimit: number
}
