# KnowNote 接口契约（前后端唯一约定，修改需同步两端）

- 前缀：`/api`；后端监听 `:8080`，前端开发服务器把 `/api` 代理到这里。
- 字段命名：JSON 一律 **camelCase**；时间一律 RFC3339 字符串（如 `2026-11-14T10:00:00Z`）；ID 一律字符串（UUID）。
- 统一响应：`{ "code": 0, "data": <T>, "message": "ok" }`。出错时 `code != 0`，同时返回合适的 HTTP 状态码（400/401/403/404/409/429/500），`message` 为中文可读信息。
- 鉴权：除 `/api/auth/*` 和 `/api/health` 外全部需要 `Authorization: Bearer <JWT>`；所有资源按当前用户隔离，访问他人资源返回 404。
- 分页：本系统数据量小，除管理员用户列表外，列表接口一次返回全部，不分页。

## 数据模型（与 frontend/src/types/index.ts 一致）

```ts
User         { id, email, nickname, role: 'user'|'admin', avatarUrl?, createdAt }
Notebook     { id, title, emoji, color(0-7), description?, sourceCount, tags: Tag[], createdAt, updatedAt }
Tag          { id, name, color(#RRGGBB), createdAt }
LoginLog     { id, email, success, ip, userAgent, createdAt }
Source       { id, notebookId, type: 'pdf'|'docx'|'pptx'|'xlsx'|'csv'|'web'|'text'|'markdown'|'audio'|'youtube',
               title, status: 'uploading'|'processing'|'ready'|'error', progress?, size?, url?,
               summary?, topics?: string[], content?(原文前 3000 字), wordCount?, error?, createdAt,
               pageCount?(PDF 页数 / 幻灯片张数 / 工作表数), sections?: SourceSection[],
               previewable?(true 表示可调用 /sources/:id/preview 查看原件 PDF), originalExt?(如 'pptx'、'doc') }
SourceSection { page(从1开始：PDF 页码 / 幻灯片序号 / 工作表序号), label('第 3 页' | '第 5 张幻灯片：标题' | '工作表：Sheet1'), title?, excerpt(该页前 120 字) }
Citation     { index(从1开始), sourceId, sourceTitle, quote(引用的原文片段), page?(片段所在页/幻灯片/工作表序号), locationLabel?(如 '第 3 页'、'第 5 张幻灯片') }
Message      { id, role: 'user'|'assistant', content, citations?, feedback?: 'up'|'down', createdAt }
Note         { id, notebookId, title, content(Markdown), origin: 'manual'|'chat'|'studio', createdAt, updatedAt }
StudioArtifact { id, notebookId, kind: 'ppt'|'word'|'pdf'|'study-guide'|'briefing'|'faq'|'timeline'|'mindmap',
               title, format?: 'pptx'|'docx'|'pdf', preset?, prompt?, retrievalMode?: 'rag'|'sources',
               content?(Markdown), mindmap?: MindMapNode, slides?: SlideDeck, createdAt }
SlideDeck    { title, subtitle?, slides: { title, subtitle?, bullets: string[], notes? }[] }
MindMapNode  { id, label, children?: MindMapNode[] }
AudioOverview { notebookId, status: 'idle'|'generating'|'ready'|'error', title?, durationSec?, url?, transcript?, error? }
NotebookGuide { summary(Markdown), suggestedQuestions: string[] }
UserSettings { model(仅管理员返回), answerStyle: 'concise'|'detailed'|'learning', language: 'zh-CN'|'en-US', storageUsed, storageTotal }
```

## 接口列表

### 健康检查 / 认证
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/health` | - | `{ status: "ok", db: "ok" }` |
| POST | `/auth/register` | `{ email, password(≥6位), nickname }` | `{ token, user }`（邮箱重复返回 409） |
| POST | `/auth/login` | `{ email, password }` | `{ token, user }`（错误返回 401） |
| GET | `/me/login-logs` | - | `LoginLog[]`（最近 20 条，时间倒序） |
| GET | `/me` | - | `User` |
| GET | `/me/settings` | - | `UserSettings`（普通用户响应省略 `model` 字段） |
| PATCH | `/me/settings` | `Partial<UserSettings>`（storage* 只读；`model` 仅管理员可修改，普通用户返回 403） | `UserSettings`（普通用户响应省略 `model` 字段） |

认证审计与限流：登录/注册成功和失败均写 `login_logs`，失败时 user_id 为空，邮箱先去除首尾空白并转小写；不记录密码、token 或请求体。邮箱在过去 10 分钟已有至少 5 次失败时，后续登录和注册返回 HTTP 429、code=42901、“尝试次数过多，请稍后再试”（第五次错误认证本身仍为原业务错误，第六次起限流；被限流的尝试也记失败）。成功不清除窗口内失败计数。参数错误也记录；无法解析的请求保留原 400，无法取得邮箱时记空字符串。IP 使用直连地址，不信任任意代理头。

`GET /me/login-logs` 只返回当前 user_id 的成功记录，以及当前邮箱、user_id 为空的失败记录，不接受邮箱或用户筛选参数；最多 20 条，createdAt 倒序。用户删除级联清理已关联的成功日志，匿名失败日志按审计记录保留，不建立他人账号关联。

### 笔记本
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/notebooks` | - | `Notebook[]`（按 updatedAt 倒序；可选 `?tag=<标签UUID>`，越权/不存在标签返回 404） |
| POST | `/notebooks` | `{ title? }`（默认“未命名笔记本”，随机 emoji/color） | `Notebook` |
| GET | `/notebooks/:id` | - | `Notebook` |
| PATCH | `/notebooks/:id` | `{ title?, emoji?, color?, description? }` | `Notebook` |
| DELETE | `/notebooks/:id` | - | `null`（级联删除来源、分块、消息、笔记、产物、文件） |
| GET | `/notebooks/:id/guide` | - | `NotebookGuide`（无 ready 来源时返回空 summary 与空数组；结果缓存，来源变化后失效） |

### 标签
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/tags` | - | `Tag[]`（当前用户，createdAt 正序） |
| POST | `/tags` | `{ name, color? }` | `Tag` |
| PATCH | `/tags/:id` | `{ name?, color? }` | `Tag` |
| DELETE | `/tags/:id` | - | `null`（移除全部笔记本关联，保留笔记本） |
| PUT | `/notebooks/:id/tags` | `{ tagIds: string[] }` | `Notebook`（事务整体替换；空数组清空，重复 ID 去重，最多 100 个） |

标签名称去除首尾空白后为 1～50 字，同一用户名称精确匹配唯一，重名返回 409、code=40901、“该标签名称已存在”。颜色使用 `#RRGGBB`，默认 `#6750A4`，响应大写。笔记本所有返回均包含 `tags`，未标记时为 `[]`。关联时笔记本与所有标签均须属于当前用户，非法/越权标签不改变原关联，返回 404。

### 来源
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/notebooks/:id/sources` | - | `Source[]`（按 createdAt 正序） |
| POST | `/notebooks/:id/sources/upload` | multipart，字段 `file`（≤200MB；pdf / doc / docx / ppt / pptx / xls / xlsx / csv / txt / md / markdown / mp3 / wav / m4a） | `Source`（status=processing，后台异步解析） |
| POST | `/notebooks/:id/sources` | `{ type: 'web'|'youtube'|'text', title?, url?, text? }` | `Source`（status=processing） |
| GET | `/sources/:id` | - | `Source`（前端每 1.5s 轮询直到 ready/error） |
| PATCH | `/sources/:id` | `{ title }` | `Source` |
| DELETE | `/sources/:id` | - | `null` |
| GET | `/sources/:id/preview` | - | 原件的 PDF 预览（`Content-Type: application/pdf`，`Content-Disposition: inline`，支持 Range）。PDF 直接返回原文件；doc/docx/ppt/pptx 由 LibreOffice 转出的 PDF；不可预览时 404。需鉴权，也接受 `?token=`。使用 OSS 时 302 跳转到 10 分钟有效的签名 URL。前端用 `<iframe src=".../preview?token=…#page=N">` 打开到指定页 |
| GET | `/sources/:id/file` | - | 下载原始文件（`Content-Disposition: attachment`），鉴权方式同上 |

类型识别与解析：
- `.pdf`：按页提取文字（每页一个 section），扫描件无文字时 status=error，error="未能从 PDF 中提取文字，可能是扫描件（本系统暂不支持 OCR）"。
- `.pptx`：按幻灯片提取标题、正文、表格与演讲者备注（每张幻灯片一个 section，label 带幻灯片标题）；`.ppt` 先用 LibreOffice 转 pptx 再解析，type 记为 'pptx'，originalExt='ppt'。
- `.docx`：保留标题层级（转为 Markdown 标题）与表格（转为 Markdown 表格）；`.doc` 先转 docx，type 记为 'docx'。
- `.xlsx` / `.xls` / `.csv`：每个工作表一个 section，按行转成「列名：值」文本，超大表只取前 2000 行并在 error 外的 summary 中说明；`.xls` 先转 xlsx。
- 切分时 chunk 不跨页/幻灯片/工作表，并记录 page，供引用定位。
- 未安装 LibreOffice 时，旧版格式返回 status=error，error="需要安装 LibreOffice 才能解析旧版 Office 文件"；新版格式与 PDF 不依赖 LibreOffice。

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
默认仅使用相似度 ≥0.2 的非空片段；全部低于阈值时，保留相似度最高的至多 3 个非空片段作为上下文并记录日志。相邻块合并后引用数量可能减少。兜底只提供可引用的资料，资料不足时仍须明确说明，不能据此推断问题一定可回答。

### 笔记
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/notebooks/:id/notes` | - | `Note[]`（updatedAt 倒序） |
| POST | `/notebooks/:id/notes` | `{ title, content, origin }` | `Note` |
| PATCH | `/notes/:id` | `{ title?, content? }` | `Note` |
| DELETE | `/notes/:id` | - | `null` |
| GET | `/notes/:id/export` | 查询 `?format=docx\|pdf\|pptx` | 导出对应二进制文件（attachment 下载） |

### 工作室
| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/notebooks/:id/artifacts` | - | `StudioArtifact[]`（createdAt 倒序） |
| POST | `/notebooks/:id/artifacts` | `{ kind, topic?, preset?, sourceIds? }` | `StudioArtifact`（携带 `topic` 时先执行向量 RAG 检索；支持 `ppt`/`word`/`pdf` 及既有类型） |
| GET | `/artifacts/:id/export` | 查询 `?format=pptx\|docx\|pdf&inline=1` | 导出或预览对应 `.pptx`/`.docx`/`.pdf` 二进制文件 |
| DELETE | `/artifacts/:id` | - | `null` |
| GET | `/notebooks/:id/audio` | - | `AudioOverview`（从未生成时 status=idle） |
| POST | `/notebooks/:id/audio` | - | `AudioOverview`（立即返回 generating，后台生成两人对谈脚本；配置了 TTS 时合成音频并给出 url，否则只给 transcript） |
| GET | `/files/audio/:file` | - | 音频文件（需鉴权，也接受 `?token=` 以便 `<audio>` 标签播放；使用 OSS 时 302 跳转签名 URL） |

## 文件存储

上传原件、转换出的预览 PDF、音频概览统一经存储层读写，`STORAGE_DRIVER=local`（默认，写入 `DATA_DIR`）或 `oss`（阿里云 OSS）。OSS 配置：`OSS_REGION`（如 cn-hangzhou）、`OSS_ENDPOINT`（可选，自定义/内网域名）、`OSS_BUCKET`、`OSS_ACCESS_KEY_ID`、`OSS_ACCESS_KEY_SECRET`、`OSS_PREFIX`（对象键前缀，默认 `knownote/`）。对象键：`<prefix>uploads/<userID>/<sourceID>/original.<ext>`、`.../preview.pdf`、`<prefix>audio/<userID>/<notebookID>.mp3`。Bucket 保持私有，浏览器只拿签名 URL。

## 模型能力与共享额度（T9）

以下接口均需 JWT，与用户设置一样挂在 `/api` 下。`provider` 只包含服务域名（无协议、路径、凭据），未启用真实能力时为 `fake`；图像的 provider 为配置值 `openai` / `dashscope` / 空字符串。

| 方法 | 路径 | 返回 data |
|---|---|---|
| GET | `/models` | `{ chat: [{ id, name, provider }], embed: { model, dimension, provider }, tts: { enabled }, image: { enabled, provider, model } }`（仅管理员可用，普通用户返回 403） |
| GET | `/me/usage` | `{ date: "2026-11-14", chatRequests: 12, tokens: 8400, reservedTokens: 4096, chatRequestsLimit: 200, tokensLimit: 200000 }` |

- `PATCH /me/settings` 的 `model` 必须属于 `/models.chat` 的 id，非法值返回 HTTP 400、code=40001、“请选择可用的对话模型”。空 `CHAT_MODELS` 时仅开放默认 `CHAT_MODEL`，配置列表时也始终包含默认模型。旧用户已保存的不可用模型按当前默认模型读取和执行，再次选择后持久化新值。
- chat 模型共用 `CHAT_BASE_URL/API_KEY`，用于用户对话、摘要、概要与工作室。只有向量服务配置 Key 时，对话仍使用 Fake。tts.enabled 表示能合成真实语音，false 时仅提供文稿；image.enabled 表示配置了提供方，缺 Key 时内部调用生成 Fake PNG；本版本不开放图像业务路由。
- `date` 使用北京时间（UTC+8）每日边界；`tokens` 为各能力输入与输出 Token 合计，优先使用上游 usage，缺失时按 Unicode 字符数估算。流式最终 usage 包含推理 Token 时也计入额度，但 reasoning_content 不发送给前端。`reservedTokens` 是处理中预留，不重复计入 tokens。
- 每日默认 200 次对话生成、200000 Token；摘要、概要、工作室脚本及格式重试计入对话次数，向量与语音只计入 Token。失败与取消的实际调用同样记录。两项 limit 为 0 表示不限；额度不等同云端计费账单。
- 调用前在数据库内原子预留输入字符数与最大输出预算，完成后结算，因此剩余额度不足请求预算时会提前拒绝；超额 HTTP 429、code=42901、message=`今日额度已用完，明天再来`。SSE 通过 `{"type":"error","message":"今日额度已用完，明天再来"}` 后结束。
- 每用户每分钟最多 20 次对话生成，超出提示“提问太频繁，请一分钟后再试”；单进程所有能力默认最多 8 个并发调用，忙时提示“当前使用人数较多，请稍后重试”，普通请求均为 429，SSE 发 error。缓存命中、没有所选来源等未调用模型的情况不计量。
- 上游 429/5xx 有限重试，流建立后不重复请求；返回友好中文，不返回上游原始报错或密钥。来源后台失败在 Source.error 展示，异步音频失败在 AudioOverview.error 展示。

## 引用持久化（T10，接口兼容）

SSE 与 Message.citations 的字段和编号规则保持不变。新回答在事务中保存 messages 与 message_citations，读取历史只从引用表组装；不再写 messages.citations JSONB，旧列保留用于兼容与核查。首次启动迁移按 `schema_migrations` 的 `t10_message_citations` 版本执行一次，将旧引用的编号、来源标题、摘录和位置完整搬迁；可定位且同用户同笔记本的 source_id 建立外键，无法确定的分块关联留空。迁移失败整体回滚。

每个引用编号对应一行；相邻分块合并时 chunk_id 指向组内首个原文分块。来源/分块删除时引用外键 SET NULL，sourceRef/sourceTitle/quote 保留快照，对外 sourceId 仍是原引用标识，前端可显示已失效来源。删除消息或笔记本会 CASCADE 清理引用行。详细实体、约束与索引见 [数据库设计](数据库设计.md)。


## 管理员后台（T11）

`/api/admin/*` 全部要求管理员。JWT 校验后每次查数据库确认账号存在、disabled=false；管理员接口再确认 role=admin。普通用户返回 HTTP 403、code=40301、“仅管理员可访问”。停用账号登录及已有 token 的业务请求返回 403、“账号已被停用”（认证失败窗口限流仍按认证规则生效）；不会中断已经开始执行的请求。GET /me、注册/登录的 user 均新增 role。

管理员认定：`ADMIN_EMAILS` 中的邮箱去首尾空白、转小写，在启动/注册/登录时提升为 admin；列表不为空时不做首用户回退。未配置且库内没有管理员时，将最早注册用户（created_at、id 排序）提升为管理员。管理员后台不能停用自己、撤销自己，也不能移除最后一位可用管理员；并发变更在数据库事务锁内串行校验。环境名单中的用户被手工撤销后，下次登录/启动仍会按名单提升，永久撤销需同步调整部署环境。

### 配置与连接测试

| 方法 | 路径 | 请求体 | 返回 data |
|---|---|---|---|
| GET | `/admin/settings` | — | `AdminSettings` |
| PATCH | `/admin/settings` | `{ values: { [配置键]: string } }` | `AdminSettings`；测试失败 400 且不保存 |
| POST | `/admin/settings/test` | `{ values?: { [配置键]: string }, capability?: 'chat'|'embed' }` | `{ ok, latencyMs, message }`；连接失败正常返回 ok=false，参数错误 400 |

```ts
AdminSetting { key: string, value: string, source: 'database'|'env'|'default' }
AdminSettings {
  chat: AdminSetting[], embed: AdminSetting[], quota: AdminSetting[],
  reindexRequired: boolean, message: string
}
```

可写键仅为下面九项，其他键（含 EMBEDDING_DIM）一律 400：

| 分组 | 配置键 | 值 |
|---|---|---|
| 对话 chat | CHAT_BASE_URL、CHAT_API_KEY、CHAT_MODEL、CHAT_MODELS | 字符串；CHAT_MODELS 为逗号分隔模型名，始终包含默认模型 |
| 向量 embed | EMBED_BASE_URL、EMBED_API_KEY、EMBED_MODEL | 字符串；返回向量维度必须等于固定 EMBEDDING_DIM |
| 额度 quota | QUOTA_DAILY_CHAT_REQUESTS、QUOTA_DAILY_TOKENS | 非负整数字符串，0 为不限 |

读取优先级为数据库覆盖值 → 启动时环境配置（含 dev.sh 注入及旧 LLM_* 回退）→ 内置默认。来源指实际使用的覆盖层；密文无法解密时警告并回退环境/默认，其来源也随回退改变。未单独设置 CHAT_MODELS 时列表跟随当前默认模型。

`values` 部分更新示例：`{"values":{"CHAT_BASE_URL":"http://localhost:18102","CHAT_API_KEY":"仅供示例的占位值","QUOTA_DAILY_TOKENS":"0"}}`。省略或空字符串均不修改；`__CLEAR__` 删除该键的数据库值并回退环境/默认。密钥始终返回前 3 位 + `****` + 后 4 位；长度不足 8 时仅返回 `****`，未配置返回空。前端以掩码作占位，绝不将掩码自动提交为新密钥。API Key 使用带随机 nonce 的 AES-GCM 密文入库，主密钥取 SETTINGS_ENCRYPTION_KEY 的 SHA-256，留空则使用 JWT_SECRET 的 SHA-256；接口、审计和日志均不返回明文。

保存串行执行：合并候选配置、校验格式 → 构造客户端 → 对受影响能力测试（对话短消息最多 8 输出 Token；向量短文本并核对维度；总超时 15 秒）→ 数据库事务写配置与脱敏审计 → Provider 原子替换。额度单独更新不请求模型；测试接口 capability 可只测一个能力，未指定时测试涉及的能力，空 values 则测试两者。未配置密钥的能力沿用 Fake，测试不会访问外部服务。连接测试不计用户用量，也不保存上游原始错误。正在进行的流使用旧客户端完成，新调用及模型目录、默认模型回退、额度读取立即更新。

EMBED_MODEL 或 EMBED_BASE_URL 实际变化时，响应 `reindexRequired=true`，message 为“向量模型已更换，旧资料需要重建向量才能被准确检索”。向量重建期间拒绝修改向量组，允许修改对话与额度。重建提示与进度为单进程内存状态，重启后重置；配置与审计持久化，不自动重建。

### 用户、用量与审计

| 方法 | 路径 | 请求体 / 参数 | 返回 data |
|---|---|---|---|
| GET | `/admin/users` | `query?` 搜索邮箱/昵称；`page=1`、`pageSize=20`（1～100） | `{ items: AdminUser[], total, page, pageSize }` |
| PATCH | `/admin/users/:id` | `{ disabled?: boolean, role?: 'user'|'admin' }`（至少一项） | `null`；自操作/最后管理员约束 400，不存在 404 |
| GET | `/admin/usage` | — | `{ days: UsageDay[], topUsers: UsageTop[] }` |
| GET | `/admin/audit-logs` | — | `AuditLog[]`，最近 50 条，按 createdAt/id 倒序 |

```ts
AdminUser extends User {
  disabled: boolean, notebookCount: number, todayRequests: number,
  todayTokens: number, weekTokens: number
}
UsageDay { date: 'YYYY-MM-DD', requests: number, tokens: number }
UsageTop { id, email, nickname, requests: number, tokens: number }
AuditLog { id, userId: string|null, action, target, detail, createdAt }
```

所有日期边界均为北京时间；近 7 天含今天，days 固定返回七天并补零。用户列表 todayRequests 为计入每日额度的对话生成次数（含后台摘要等），用量页 requests 为全部能力的实际调用次数；失败调用也统计，未结算 Token 尚不计入 tokens。Top 10 按今日 Tokens 降序。系统配置、用户和全站统计是管理员明确授权的跨用户视图，普通业务资源仍按 user_id 隔离。

修改配置审计只记录变更键名；账号操作记录目标 UUID 与状态/角色；启动重建记录固定中文说明。审计不存请求体、密钥或上游响应，用户删除后 userId 置空，记录保留。

### 异步重建

| 方法 | 路径 | 返回 data |
|---|---|---|
| POST | `/admin/reindex` | `ReindexProgress`，任务已启动；已有任务返回 409 |
| GET | `/admin/reindex` | `ReindexProgress` |

```ts
ReindexProgress { total: number, completed: number, failed: number, running: boolean, message: string }
```

总数与进度以已有分块为单位；处理中 completed 表示已处理的暂存分块数，事务提交后才持久生效。失败时整次回滚，completed 归零，failed 表示本次未提交的分块总数（无法统计总数的失败至少记 1）。复用 T9 重建事务保留分块 ID、原文、页码、引用链接与文件，期间 sources/chunks 的读写会等待；向量调用按原资料用户记用量并使用当前额度。任务固定启动时的向量客户端，配置写锁与单任务状态防止重建期间更换向量空间。进程退出取消并等待重建回滚；不做多实例任务协调。
