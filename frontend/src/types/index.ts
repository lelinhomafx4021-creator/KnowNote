/** ========== 领域模型（与后端接口约定保持一致） ========== */

export type SourceType = 'pdf' | 'docx' | 'pptx' | 'xlsx' | 'csv' | 'web' | 'text' | 'markdown' | 'audio' | 'youtube'
export type SourceStatus = 'uploading' | 'processing' | 'ready' | 'error'

export interface User {
  role: 'user' | 'admin'
  id: string
  email: string
  nickname: string
  avatarUrl?: string
  createdAt: string
}

export interface Tag { id: string; name: string; color: string; createdAt: string }
export interface LoginLog { id: string; email: string; success: boolean; ip: string; userAgent: string; createdAt: string }

export interface SampleNotebook {
  slug: string
  title: string
  emoji: string
  description: string
  tone: number
  sourceCount: number
  ready: boolean
  addedNotebookId?: string
}

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

export type StudioKind = 'ppt' | 'word' | 'pdf' | 'study-guide' | 'briefing' | 'faq' | 'timeline' | 'mindmap'
export type ExportFormat = 'pptx' | 'docx' | 'pdf'

export interface MindMapNode {
  id: string
  label: string
  children?: MindMapNode[]
}

export interface Slide {
  title: string
  subtitle?: string
  bullets: string[]
  notes?: string
}

export interface SlideDeck {
  title: string
  subtitle?: string
  slides: Slide[]
}

export interface StudioArtifact {
  id: string
  notebookId: string
  kind: StudioKind
  title: string
  format?: ExportFormat
  preset?: string
  prompt?: string
  retrievalMode?: 'rag' | 'sources' | ''
  /** Markdown 正文（mindmap 时为空） */
  content?: string
  mindmap?: MindMapNode
  slides?: SlideDeck
  createdAt: string
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

export type SettingKey =
  | 'CHAT_BASE_URL'
  | 'CHAT_API_KEY'
  | 'CHAT_MODEL'
  | 'CHAT_MODELS'
  | 'EMBED_BASE_URL'
  | 'EMBED_API_KEY'
  | 'EMBED_MODEL'
  | 'STORAGE_DRIVER'
  | 'OSS_REGION'
  | 'OSS_ENDPOINT'
  | 'OSS_BUCKET'
  | 'OSS_ACCESS_KEY_ID'
  | 'OSS_ACCESS_KEY_SECRET'
  | 'OSS_PREFIX'
  | 'QUOTA_DAILY_CHAT_REQUESTS'
  | 'QUOTA_DAILY_TOKENS'
export interface AdminSetting { key: SettingKey; value: string; source: 'database' | 'env' | 'default' }
export interface AdminSettings { chat: AdminSetting[]; embed: AdminSetting[]; storage: AdminSetting[]; quota: AdminSetting[]; reindexRequired: boolean; message: string }
export interface AdminUser extends User { disabled: boolean; notebookCount: number; todayRequests: number; todayTokens: number; weekTokens: number }
export interface AdminUserPage { items: AdminUser[]; total: number; page: number; pageSize: number }
export interface AdminUsage { days: { date: string; requests: number; tokens: number }[]; topUsers: { id: string; email: string; nickname: string; requests: number; tokens: number }[] }
export interface AuditLog { id: string; userId: string | null; action: string; target: string; detail: string; createdAt: string }
export interface ReindexProgress { total: number; completed: number; failed: number; running: boolean; message: string }
export interface ConnectionTest { ok: boolean; latencyMs: number; message: string }

/** 语音输入配置与识别结果 */
export interface SpeechConfig {
  enabled: boolean
  maxSeconds: number
}

export interface SpeechResult {
  text: string
}
