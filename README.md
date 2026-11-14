# 知页 KnowNote

知页是一个基于大语言模型的个人资料管理系统，也是仿 NotebookLM 的毕业设计实现。它将资料收集、来源问答与学习内容整理放在同一个笔记本中：上传论文和文档、添加网页或文本后，系统异步解析并建立向量索引；回答通过流式输出呈现，并可点击引用核对来源片段。

## 已实现功能

- 邮箱注册登录、JWT 认证、用户数据隔离、笔记本管理、主题与回答风格设置。
- PDF、DOCX、TXT、Markdown、公开网页和粘贴文本接入，处理状态、摘要、主题与原文预览。
- 在选中来源内进行 RAG 检索问答，SSE 流式回答、引用跳转、历史记录与赞踩反馈。
- 回答保存为笔记、Markdown 笔记编辑；学习指南、简报、FAQ、时间线、思维导图五种工作室产物。
- 双主持音频概览文稿；配置兼容 TTS 后可合成音频并鉴权播放。
- 无 API Key 时自动使用 Fake 模型，完整业务链仍可演示和测试。公开网页导入本身仍需联网。

## 功能截图位置说明

以下是论文与展示材料的截图位置规划，仓库未放置占位图片或伪造截图。运行系统后可按表取景。

| 截图位 | 页面与建议内容 |
|---|---|
| 图 1：认证入口 | `/register` 或 `/login`，品牌区与中文表单 |
| 图 2：资料工作台 | 三栏工作台，展示 PDF/网页/文本 ready、概要和推荐问题 |
| 图 3：可追溯问答 | 回答引用角标、左侧引用片段和原文高亮同时显示 |
| 图 4：学习整理 | 工作室五种生成内容及保存的对话笔记 |
| 图 5：思维导图 | 打开思维导图或全屏视图 |
| 图 6：音频概览 | Fake 下展开双主持文稿；有真实 TTS 时显示播放器 |

## 环境要求

- Go 1.27+、Node.js 24+ 与 npm。
- Docker Desktop 或兼容 Docker 的本地运行环境，支持 `docker compose`。
- PostgreSQL 16 + pgvector 由 Compose 提供，默认使用 5432；后端 8080、前端 5173。
- macOS/Linux 可直接运行 Bash 脚本；Windows 建议使用 WSL2。
- 自动化端到端脚本使用 Node.js 内置 fetch，无需额外浏览器驱动。

## 快速开始

在仓库根目录执行：

```bash
# 首次安装前端项目依赖（dev.sh 在依赖目录不存在时也会执行 npm ci）
npm --prefix frontend ci

# 启动数据库，等待就绪，然后同时运行真实后端与真实前端
./scripts/dev.sh
```

打开 `http://localhost:5173`，注册一个测试账户，新建笔记本并添加资料。默认不需要模型密钥；后端自动迁移数据库并启用 Fake。按 **Ctrl-C** 会一起停止前后端，数据库容器和数据卷保留。脚本不读取任何 `.env`，配置从启动终端的环境变量取得；`backend/.env.example` 与 `frontend/.env.example` 仅作说明。

5173 已被其他服务使用时：

```bash
FRONTEND_PORT=5176 ./scripts/dev.sh
```

脚本会同步设置默认 CORS 来源，并以 `--strictPort` 启动 Vite；端口占用时明确退出，避免连到别的项目。当前开发代理固定指向 8080，因此该脚本要求后端使用 8080。若显式设置 `CORS_ORIGINS`，应包含实际的前端地址，使用浏览器时保持 `localhost` 域名一致。

也可分别启动（在三个终端中）：

```bash
docker compose up -d
```

```bash
cd backend
go run ./cmd/server
```

```bash
cd frontend
VITE_USE_MOCK=false npm run dev
```

仅演示前端 UI 时，可在 `frontend/` 执行 `VITE_USE_MOCK=true npm run dev`，使用任意非空账号密码进入 mock 数据。该模式与后端 Fake 模型不同：Fake 仍执行真实数据库、解析、向量检索和接口流程。

### 配置 OpenAI 兼容模型

所有模型配置只在后端设置，不能使用 `VITE_` 变量保存密钥。未设置 `LLM_API_KEY` 时为 Fake；设置后不会在网络错误时偷偷退回 Fake。开发环境未设置 `JWT_SECRET` 时会生成临时密钥，重启会让已有登录失效；持久使用和生产部署应注入稳定的随机密钥，生产至少 32 字节。

下面以百炼北京地域为例，聊天与向量化共用一个兼容地址和密钥。`text-embedding-v2` 原生为 1536 维，与本项目默认数据库一致；客户端对 `text-embedding-v*` 每批最多发送 10 条。模型名称、地域及开通权限以供应商控制台为准。接口地址与维度依据[百炼 Chat 文档](https://help.aliyun.com/zh/model-studio/qwen-api-via-openai-chat-completions/)和[百炼 Embedding 文档](https://help.aliyun.com/zh/model-studio/embedding-interfaces-compatible-with-openai)。

```bash
# 先在当前终端或密钥管理工具中安全注入 DASHSCOPE_API_KEY
export LLM_API_KEY="$DASHSCOPE_API_KEY"
export LLM_BASE_URL='https://你的业务空间ID.cn-beijing.maas.aliyuncs.com/compatible-mode/v1'
export LLM_CHAT_MODEL='qwen-plus'
export LLM_EMBED_MODEL='text-embedding-v2'
export EMBEDDING_DIM=1536
./scripts/dev.sh
```

百炼原地址 `https://dashscope.aliyuncs.com/compatible-mode/v1` 仍可使用，新业务空间建议采用控制台提供的专属地址。[官方地址说明](https://help.aliyun.com/zh/model-studio/embedding-interfaces-compatible-with-openai)。

DeepSeek 直连的对话配置示意如下；当前官方模型示例为 `deepseek-flash`，不要把界面历史预设的 `deepseek-v3` 直接当作官方模型名。[DeepSeek 官方调用文档](https://api-docs.deepseek.com/en/)。

```bash
export LLM_BASE_URL='https://api.deepseek.com'
export LLM_API_KEY="$DEEPSEEK_API_KEY"
export LLM_CHAT_MODEL='deepseek-flash'
```

**完整 RAG 还必须提供 `/embeddings`。** 当前客户端的聊天、Embedding、TTS 共用 `LLM_BASE_URL` 和 `LLM_API_KEY`，不能分别直连两个供应商。上述 DeepSeek 片段只说明对话参数；完整部署应使用同时路由聊天与向量化的 OpenAI 兼容网关，将 `LLM_BASE_URL` 改为网关地址，`LLM_API_KEY` 改为网关密钥，`LLM_CHAT_MODEL` 与 `LLM_EMBED_MODEL` 填网关实际映射的模型名。不要把 DeepSeek 聊天模型填入向量模型字段。

已有用户的 `settings.model` 会覆盖启动时的聊天默认模型；更换服务后，可通过 `PATCH /api/me/settings` 更新 `model`，或注册新测试账户。界面中的模型预设也须由当前服务支持。切换 Embedding 模型后应重新导入资料，即使维度相同也不能混用不同向量空间；更换维度还需独立数据库或正式迁移，应用会拒绝维度不一致的启动。HNSW 向量列支持本项目配置范围 1～2000 维。

如服务支持 OpenAI 兼容 `/audio/speech`，可设置 `LLM_TTS_MODEL` 与 `LLM_TTS_VOICES`（两个逗号分隔音色）。不配置 TTS 时音频概览只提供文稿，Fake 始终仅提供文稿。

### 验证与复现

```bash
# 数据库启动后，集中完成 Go 构建/vet/测试、前端传输状态测试与生产构建
./scripts/check.sh

# 保持 dev.sh 运行，在另一终端执行完整端到端验收
cd frontend
E2E_BASE_URL=http://localhost:5173 npm run test:e2e
```

端到端脚本经 Vite 代理使用真实前端 API 和工作台状态，覆盖注册登录、三类来源、引用 SSE、笔记、五种产物、音频文稿、删除和退出；PDF multipart 使用 Node fetch 上传，浏览器 XHR 上传另由传输测试与实际 Chrome 验证。脚本需公网访问 `https://example.com`，按无 Key/Fake 模式验收并要求无音频 URL。脚本成功或失败均尝试清理自己的笔记本；测试用户保留，因为产品没有用户删除接口。数据库不可用时 Go 集成测试会标明 Skip，因此正式验收须先确认数据库健康且没有 Skip。

## 目录结构

```text
KnowNote/
├── frontend/               React + TypeScript + Vite，API、状态和三栏工作台
│   ├── src/api/            统一请求、上传、SSE 与可选 mock
│   ├── src/store/          登录和工作台状态、任务取消
│   ├── src/features/       来源、对话、工作室
│   └── scripts/            传输状态测试与真实端到端验收
├── backend/
│   ├── cmd/server/         启动、迁移、依赖组装与优雅退出
│   ├── internal/           配置、模型、数据库、HTTP、业务、解析、RAG、LLM、存储
│   └── pkg/response/       统一响应与错误码
├── scripts/                dev.sh 一键启动、check.sh 集中检查
├── docker-compose.yml      PostgreSQL 16 + pgvector 与持久卷
└── docs/                   接口契约、架构设计、测试报告、计划与 T1–T6 交付记录
```

进一步阅读：[接口契约](docs/api.md)、[架构设计](docs/架构设计.md)、[测试报告](docs/测试报告.md)、[T6 交付及遗留处理](docs/progress/T6.md)。

## 当前限制

扫描 PDF 需先 OCR，旧 DOC 需先转换；音视频与 YouTube/B站暂不自动转写。网页抓取不执行 JavaScript，不支持登录/反爬页面或内网地址。上传不超过 200MiB，提取正文另有 20MiB 限制，网页响应限制 10MiB。来源与音频队列面向单进程；错误来源需删除重加。Fake 用于流程演示，不能代替真实模型的语义能力；真实付费模型与真实 TTS 本轮未调用。完整验收证据与其他边界见测试报告。
