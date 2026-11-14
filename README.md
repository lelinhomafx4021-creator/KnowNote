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

打开 `http://localhost:5173`，注册一个测试账户，新建笔记本并添加资料。默认不需要模型密钥；后端自动迁移数据库并启用 Fake。按 **Ctrl-C** 会一起停止前后端，数据库容器和数据卷保留。模型密钥等配置写在 `backend/.env`（从 `backend/.env.example` 复制，已被 Git 忽略），`scripts/dev.sh` 启动时会按 `KEY=VALUE` 读取其中的 `JWT_SECRET`、`APP_ENV`、`DATA_DIR`、`DATABASE_URL`、`EMBEDDING_DIM` 与 `LLM_*`；启动终端里已 `export` 的同名变量优先。端口与 CORS 由脚本统一设置。

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

### 配置真实模型

所有密钥只在后端通过进程环境注入。对话、向量、语音分别判断是否有密钥；未配置的能力使用 Fake，网络错误不会自动切成 Fake。`backend/.env.example` 提供完整的空值模板。旧 `LLM_BASE_URL/API_KEY/CHAT_MODEL/EMBED_MODEL/TTS_MODEL/TTS_VOICES` 继续作为对应新变量的回退值；独立接入时请保持旧 `LLM_API_KEY` 为空，以免一个供应商的凭据被回退到另一能力。

“DeepSeek 对话 + 百炼向量”需填写：

| 变量 | 值 |
|---|---|
| `CHAT_BASE_URL` | `https://api.deepseek.com` |
| `CHAT_API_KEY` | 安全注入 DeepSeek Key |
| `CHAT_MODEL` | `deepseek-flash` |
| `CHAT_MODELS` | `deepseek-flash,deepseek-v4-pro`，可选，供设置弹窗切换 |
| `EMBED_BASE_URL` | `https://你的业务空间ID.cn-beijing.maas.aliyuncs.com/compatible-mode/v1` |
| `EMBED_API_KEY` | 安全注入与地域匹配的百炼 Key |
| `EMBED_MODEL` | `text-embedding-v4` |
| `EMBEDDING_DIM` | `1536`，与已有数据库保持一致 |

按[DeepSeek 官方更新记录](https://api-docs.deepseek.com/updates/)，`deepseek-flash` 当前对应较新的 V4.1 Flash，面向快速推理并支持视觉理解；`deepseek-v4-pro` 是上一代 V4 Pro，官方按用户需求继续提供服务。本项目推荐 Flash 作为日常文本问答默认值，保留 Pro 供既有工作流比较；不发送图片输入，也不承诺具体资料上的准确性差异。流式只发送正式回答，忽略 `reasoning_content`。摘要、概要与思维导图优先使用[JSON Output](https://api-docs.deepseek.com/guides/json_mode/)，不支持时回退文本 JSON 解析。

[百炼官方向量接口](https://help.aliyun.com/zh/model-studio/text-embedding-synchronous-api/)列出 v4 的维度为 2048、1536、1024（原生默认）、768、512、256、128、64；每批最多 10 条、每条最多 8192 Token。客户端显式传入 `dimensions=EMBEDDING_DIM`，示例采用 v4 的 1536 维以兼容现有库。官方另已提供 qwen3.7-text-embedding；配置其他模型前须核对可用地域与维度。本项目 HNSW 索引限制为 1～2000 维，暂不支持 2048 及以上。旧公共地址 `https://dashscope.aliyuncs.com/compatible-mode/v1` 的兼容说明见[官方文档](https://help.aliyun.com/zh/model-studio/embedding-interfaces-compatible-with-openai)。

真实向量能力启动前会用一条短文本探测维度，再检查数据库列；不同即拒绝启动并给出重建方法。**切换向量模型即使维度相同也必须重建**：先停止服务，在当前终端注入目标 `EMBED_*` 与 `EMBEDDING_DIM`，执行 `./scripts/reindex.sh`，成功后重启。命令不读取 `.env`，保留原文、文件、分块 ID 与引用位置；中途失败整体回滚。重建遍历所有用户并计入各自 Token 额度，管理员可在这次维护进程临时设置 `QUOTA_DAILY_TOKENS=0`，不要同时运行正常服务。

设置弹窗从 `/api/models` 获取可选模型，保存到用户设置并用于提问；旧设置中的不可用模型自动按当前默认值执行。可独立配置 OpenAI 兼容语音的 `TTS_BASE_URL/API_KEY/MODEL/VOICES`，未启用时音频概览仅有文稿。图像配置与存储适配已预留，尚未开放业务接口和界面。

共享 Key 保护：默认每用户每日 200 次对话生成、200000 Token，每分钟最多 20 次对话生成；摘要、概要、工作室也计入对话次数，向量与语音计入 Token。`QUOTA_DAILY_CHAT_REQUESTS/TOKENS=0` 表示不限，`LLM_MAX_CONCURRENCY` 默认 8（单进程）。调用前按输入字符数与最大输出预算预留，完成后按上游 usage 或字符估算结算，因此剩余额度不足一次请求预算时会提前拒绝。设置中展示今日已用与处理中预留。北京时间每日重置；用量不等同供应商账单，失败/取消请求也记录。

开发环境未设置 `JWT_SECRET` 时会生成临时密钥；持久使用和生产部署应注入稳定随机密钥，生产至少 32 字节。密钥不能写入 `VITE_` 变量。

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
