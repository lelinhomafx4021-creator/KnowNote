# KnowNote 接口契约（前后端唯一约定，修改需同步两端）

- 前缀：`/api`；后端监听 `:8080`，前端开发服务器把 `/api` 代理到这里。
- 字段命名：JSON 一律 **camelCase**；时间一律 RFC3339 字符串（如 `2026-11-14T10:00:00Z`）；ID 一律字符串（UUID）。
- 统一响应：`{ "code": 0, "data": <T>, "message": "ok" }`。出错时 `code != 0`，同时返回合适的 HTTP 状态码（400/401/403/404/409/500），`message` 为中文可读信息。
- 鉴权：除 `/api/auth/*` 和 `/api/health` 外全部需要 `Authorization: Bearer <JWT>`；所有资源按当前用户隔离，访问他人资源返回 404。
- 分页：本系统数据量小，列表接口一次返回全部，不分页。

## 数据模型（与 frontend/src/types/index.ts 一致）

```ts
User         { id, email, nickname, avatarUrl?, createdAt }
Notebook     { id, title, emoji, color(0-7), description?, sourceCount, createdAt, updatedAt }
Source       { id, notebookId, type: 'pdf'|'docx'|'web'|'text'|'markdown'|'audio'|'youtube',
               title, status: 'uploading'|'processing'|'ready'|'error', progress?, size?, url?,
               summary?, topics?: string[], content?(原文前 3000 字), wordCount?, error?, createdAt }
Citation     { index(从1开始), sourceId, sourceTitle, quote(引用的原文片段) }
Message      { id, role: 'user'|'assistant', content, citations?, feedback?: 'up'|'down', createdAt }
Note         { id, notebookId, title, content(Markdown), origin: 'manual'|'chat'|'studio', createdAt, updatedAt }
StudioArtifact { id, notebookId, kind: 'study-guide'|'briefing'|'faq'|'timeline'|'mindmap',
               title, content?(Markdown), mindmap?: MindMapNode, createdAt }
MindMapNode  { id, label, children?: MindMapNode[] }
AudioOverview { notebookId, status: 'idle'|'generating'|'ready'|'error', title?, durationSec?, url?, transcript? }
NotebookGuide { summary(Markdown), suggestedQuestions: string[] }
UserSettings { model, answerStyle: 'concise'|'detailed'|'learning', language: 'zh-CN'|'en-US', storageUsed, storageTotal }
```

## 接口列表

### 健康检查 / 认证
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/health` | - | `{ status: "ok", db: "ok" }` |
| POST | `/auth/register` | `{ email, password(≥6位), nickname }` | `{ token, user }`（邮箱重复返回 409） |
| POST | `/auth/login` | `{ email, password }` | `{ token, user }`（错误返回 401） |
| GET | `/me` | - | `User` |
| GET | `/me/settings` | - | `UserSettings` |
| PATCH | `/me/settings` | `Partial<UserSettings>`（storage* 只读） | `UserSettings` |

### 笔记本
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/notebooks` | - | `Notebook[]`（按 updatedAt 倒序） |
| POST | `/notebooks` | `{ title? }`（默认“未命名笔记本”，随机 emoji/color） | `Notebook` |
| GET | `/notebooks/:id` | - | `Notebook` |
| PATCH | `/notebooks/:id` | `{ title?, emoji?, color?, description? }` | `Notebook` |
| DELETE | `/notebooks/:id` | - | `null`（级联删除来源、分块、消息、笔记、产物、文件） |
| GET | `/notebooks/:id/guide` | - | `NotebookGuide`（无 ready 来源时返回空 summary 与空数组；结果缓存，来源变化后失效） |

### 来源
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/notebooks/:id/sources` | - | `Source[]`（按 createdAt 正序） |
| POST | `/notebooks/:id/sources/upload` | multipart，字段 `file`（≤200MB；pdf/doc/docx/txt/md/markdown/mp3/wav/m4a） | `Source`（status=processing，后台异步解析） |
| POST | `/notebooks/:id/sources` | `{ type: 'web'|'youtube'|'text', title?, url?, text? }` | `Source`（status=processing） |
| GET | `/sources/:id` | - | `Source`（前端每 1.5s 轮询直到 ready/error） |
| PATCH | `/sources/:id` | `{ title }` | `Source` |
| DELETE | `/sources/:id` | - | `null` |

后台处理流程：解析正文 → 清洗 → 切分（约 500 字，重叠 80 字）→ 批量向量化 → 写入 pgvector → LLM 生成 summary 与 3~6 个 topics → status=ready。任一步失败则 status=error 并写入 error。

### 对话
| 方法 | 路径 | 请求体 | 返回 |
|---|---|---|---|
| GET | `/notebooks/:id/messages` | - | `Message[]`（正序） |
| DELETE | `/notebooks/:id/messages` | - | `null` |
| POST | `/notebooks/:id/chat` | `{ question, sourceIds: string[] }` | **SSE 流**（见下） |
| POST | `/messages/:id/feedback` | `{ value: 'up'|'down'|null }` | `null` |

SSE：`Content-Type: text/event-stream`，每个事件一行 `data: <json>` 后跟空行，顺序为：

```
data: {"type":"citations","citations":[{"index":1,"sourceId":"...","sourceTitle":"...","quote":"..."}]}
data: {"type":"delta","text":"回答片段，引用处写 [1]"}      （多次）
data: {"type":"done","messageId":"<assistant 消息 id>"}
```

出错时发送 `{"type":"error","message":"..."}` 后结束。客户端断开连接时后端应停止生成，并保存已生成部分。
检索规则：只在 `sourceIds` 且属于该笔记本、status=ready 的来源里做余弦相似度 Top-K（默认 6）；`sourceIds` 为空时直接提示“请选择来源”，不调用模型。回答风格读取用户设置 answerStyle。

### 笔记
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/notebooks/:id/notes` | - | `Note[]`（updatedAt 倒序） |
| POST | `/notebooks/:id/notes` | `{ title, content, origin }` | `Note` |
| PATCH | `/notes/:id` | `{ title?, content? }` | `Note` |
| DELETE | `/notes/:id` | - | `null` |

### 工作室
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/notebooks/:id/artifacts` | - | `StudioArtifact[]`（createdAt 倒序） |
| POST | `/notebooks/:id/artifacts` | `{ kind }` | `StudioArtifact`（同步生成，可能耗时 10~60s；mindmap 由 LLM 输出 JSON 后校验） |
| DELETE | `/artifacts/:id` | - | `null` |
| GET | `/notebooks/:id/audio` | - | `AudioOverview`（从未生成时 status=idle） |
| POST | `/notebooks/:id/audio` | - | `AudioOverview`（立即返回 generating，后台生成两人对谈脚本；配置了 TTS 时合成音频并给出 url，否则只给 transcript） |
| GET | `/files/audio/:file` | - | 音频文件（需鉴权，也接受 `?token=` 以便 `<audio>` 标签播放） |
