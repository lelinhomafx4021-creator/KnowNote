# 知页 KnowNote

知页是一个基于大语言模型的个人资料管理系统，也是仿 NotebookLM 的毕业设计实现。它将资料收集、来源问答与学习内容整理放在同一个笔记本中：上传论文和文档、添加网页或文本后，系统异步解析并建立向量索引；回答通过流式输出呈现，并可点击引用核对来源片段。

## 已实现功能

- **欢迎着陆与多角色权限**：独立宣传首页 `/welcome`、邮箱注册登录、JWT 认证、注册自动发放初始引导笔记《知页使用指南》、首页精选示例模板、管理员后台 `/admin`；普通用户界面完全隐藏模型信息与切换入口。
- **多格式资料接入与原件定位**：PDF（按页提取）、DOCX、PPTX、XLSX、GBK CSV、TXT、Markdown、公开网页（防 SSRF）与粘贴文本；支持后台入库、摘要/主题提取、原件在线预览及引用页码跳定位。
- **可追溯 RAG 问答**：在选中来源的 `pgvector` HNSW 向量空间内检索 Top-K 分块，SSE 流式回答并在正文中标注 `[n]` 角标，支持点赞/点踩与引用原文核对。
- **工作室多格式生成与宽屏预览**：
  - 紧凑小块网格 + 自定义生成面板（默认全选可用来源并支持逐项删减，可配置用途场景、篇幅/页数、受众语气、内容侧重与具体要求）；
  - 生成并导出真实可编辑的 `PPT 课件 (.pptx)`、`Word 文档 (.docx)`、`PDF 讲义 (.pdf)` 以及思维导图、学习指南、简报、FAQ 与时间线，点击预览自动宽屏展开。
- **双模运行与运维保障**：支持 DeepSeek 对话 + 阿里云百炼向量 + OpenAI 兼容 TTS + **阿里云 OSS 对象存储**，未配置 API Key 时自动启用内置 Fake 能力；管理员后台支持模型密钥与 OSS 凭据 AES-GCM 加密热更新、连通性测试、用户启用/停用/角色管理、每日额度并发锁与异步向量重建。

## 功能截图位置说明

以下是论文与展示材料的截图位置规划，仓库未放置占位图片或伪造截图。运行系统后可按表取景。

| 截图位 | 页面与建议内容 |
|---|---|
| 图 1：产品首页与认证入口 | `/welcome` 宣传着陆页或 `/register` / `/login` 注册登录页 |
| 图 2：三栏工作台与来源管理 | 笔记本首页及三栏工作台，展示《知页使用指南》或多格式来源摘要 |
| 图 3：可追溯问答 | 回答引用角标 `[1]`、左侧引用片段和原件页码高亮同时显示 |
| 图 4：工作室定制生成 | 点击 PPT/Word/PDF 弹出的自定义生成面板（来源勾选、篇幅/受众/具体要求） |
| 图 5：宽屏预览与导出 | 点击产物后右侧面板自动展开，展示 16:9 幻灯片画布或内嵌中文 PDF 预览 |
| 图 6：管理后台 `/admin` | 管理员模型/阿里云 OSS 配置热更新、用户管理、全站用量统计与向量重建面板 |

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

仅管理员可通过设置弹窗中的模型切换卡或 `/admin` 管理后台配置和切换对话模型；普通用户界面不展示模型与 `/api/models` 入口（调用 `/api/models` 或修改模型均返回 `403`），所有普通用户请求统一由系统全局激活模型执行。管理员也可直接在 `/admin` 的「模型与存储」面板配置阿里云 OSS（`STORAGE_DRIVER`、`OSS_REGION`、`OSS_ENDPOINT`、`OSS_BUCKET`、`OSS_ACCESS_KEY_ID`、`OSS_ACCESS_KEY_SECRET`、`OSS_PREFIX`），支持先点击「测试连接」验证 Bucket 权限，再切换 `oss` 保存即时生效（AK/SK 同模型密钥使用 AES-GCM 加密入库）。保留 OpenAI 兼容语音合成的通用配置 `TTS_BASE_URL/API_KEY/MODEL/VOICES`，目前没有业务调用方；图像生成底层能力（`ImageClient`）已预留，尚未开放业务界面。

共享 Key 保护：默认每用户每日 200 次对话生成、200000 额度（管理员后台以 Token 统计），每分钟最多 20 次对话生成；摘要、概要、工作室也计入对话次数，ASR 转写也占用请求次数，向量与通用语音合成计入 Token 额度。`QUOTA_DAILY_CHAT_REQUESTS/TOKENS=0` 表示不限，`LLM_MAX_CONCURRENCY` 默认 8（单进程）。调用前按输入字符数与最大输出预算预留，完成后按上游 usage 或字符估算结算，因此剩余额度不足一次请求预算时会提前拒绝。普通用户设置中仅展示今日服务额度与存储空间；北京时间每日重置，失败/取消请求也记录。

开发环境未设置 `JWT_SECRET` 时会生成临时密钥；持久使用和生产部署应注入稳定随机密钥，生产至少 32 字节。密钥不能写入 `VITE_` 变量。

### 验证与复现

```bash
# 数据库启动后，集中完成 Go 构建/vet/测试、前端传输状态测试与生产构建
./scripts/check.sh

# 保持 dev.sh 运行，在另一终端执行完整端到端验收
cd frontend
E2E_BASE_URL=http://localhost:5173 npm run test:e2e
```

端到端脚本经 Vite 代理使用真实前端 API 和工作台状态，覆盖注册登录、三类来源、引用 SSE、笔记、五种产物、删除和退出；PDF multipart 使用 Node fetch 上传，浏览器 XHR 上传另由传输测试与实际 Chrome 验证。脚本需公网访问 `https://example.com`，按无 Key/Fake 模式验收。脚本成功或失败均尝试清理自己的笔记本；测试用户保留，因为产品没有用户删除接口。数据库不可用时 Go 集成测试会标明 Skip，因此正式验收须先确认数据库健康且没有 Skip。

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

进一步阅读：[开发总结报告](docs/开发总结报告.md)、[接口契约](docs/api.md)、[架构设计](docs/架构设计.md)、[数据库设计](docs/数据库设计.md)、[测试报告](docs/测试报告.md)、[论文写作指南](docs/论文/论文写作指南.md)。

## 当前限制

扫描 PDF 需先 OCR，旧 DOC 需先转换；音视频与 YouTube/B站暂不自动转写。网页抓取不执行 JavaScript，不支持登录/反爬页面或内网地址。上传不超过 200MiB，提取正文另有 20MiB 限制，网页响应限制 10MiB。来源队列面向单进程；错误来源需删除重加。Fake 用于流程演示，不能代替真实模型的语义能力；真实付费模型与真实 TTS 本轮未调用。完整验收证据与其他边界见测试报告。

## 语音输入与用户头像

对话输入框支持录完再转文字，转写结果填入输入框，由用户确认后发送。真实模式通过鉴权的 `GET /api/speech/config` 判断是否显示麦克风；未配置 ASR 时隐藏。浏览器需允许麦克风并运行在安全上下文（HTTPS 或 localhost），录音最长 60 秒，编码为 16kHz 单声道 16-bit WAV，通过 `POST /api/speech/transcribe` 上传（字段 `audio`，最大 4MB）。音频与转写文本不落盘、不写日志。音频来源上传仍保留，来源自动转写尚未实现。

后端使用阿里云百炼 Qwen-ASR，环境变量为 `ASR_BASE_URL`、`ASR_API_KEY`、`ASR_MODEL`（默认 `qwen3-asr-flash`）；前两项缺省时，仅当 `EMBED_BASE_URL` 域名含 `aliyuncs.com` 才分别回退到向量配置。`scripts/dev.sh` 已允许注入 `ASR_*`。每用户每分钟最多 10 次，每次占用一次共享请求额度，Token 按上游用量或秒数估算。

账户菜单和偏好设置展示用户头像，偏好设置可上传或移除。`POST /api/me/avatar` 接收字段 `avatar`（PNG/JPEG，最大 2MB），后端居中裁切并规范化为 256×256 JPEG，经统一存储接口写入本地或私有 OSS；`DELETE /api/me/avatar` 移除头像。`GET /api/files/avatar/:userId` 需鉴权（支持 `?token=`），仅可读取本人头像，跨用户返回 404，OSS 使用 10 分钟签名跳转。mock 模式头像使用会话内对象 URL，刷新后不保留；语音输入返回固定演示文字，不调用云端识别。

## 部署

本地推送到 GitHub → 服务器 git clone / git pull → 填写环境配置 → Docker Compose 一键构建启动（服务器上 `make init && make deploy`，`make help` 查看全部命令），详见[部署说明](docs/部署说明.md)。
