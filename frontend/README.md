# 知页 KnowNote · AI 个人资料管理系统（前端）

参考 Google NotebookLM 的交互设计，基于 **React 18 + TypeScript + Vite + Tailwind CSS** 实现的 AI 个人资料管理系统前端。

## 快速开始

```bash
npm install
npm run dev      # http://localhost:5173
npm run build    # 类型检查 + 生产构建，输出到 dist/
```

默认使用前端内置模拟数据（`VITE_USE_MOCK=true`），无需后端。首次进入会跳转登录页，填写任意账号与非空密码即可体验；演示数据保存在内存中，刷新后恢复初始资料。

## 功能一览

| 模块 | 功能 |
|---|---|
| 首页 | 笔记本网格 / 列表视图、搜索（⌘K）、排序、新建 / 重命名 / 删除 |
| 来源面板 | 上传文件（拖拽，PDF/Word/TXT/MD/音频）、网址 / 视频链接、粘贴文本；上传进度与解析状态；勾选参与问答的来源；来源详情（AI 摘要、关键主题、原文预览、引用高亮） |
| 对话面板 | 笔记本概要与推荐问题、流式回答、Markdown 渲染（含表格）、**引用角标**（悬停预览原文片段，点击跳转来源）、停止生成、复制 / 点赞 / 保存到笔记 |
| 工作室 | 一键生成学习指南 / 简报 / FAQ / 时间线 / **思维导图**（SVG，可折叠、缩放、拖拽、全屏）、笔记编辑（Markdown，自动保存，可转为来源） |
| 通用 | 浅色 / 深色主题、设置（模型、回答风格、存储用量）、Toast、确认对话框、骨架屏、空状态 |
| 响应式 | ≥1024px 三栏（左右栏可收起为侧轨），移动端底部 Tab 切换「来源 / 对话 / 工作室」 |

## 目录结构

```
src/
  api/            接口层（组件只调用这里）
    client.ts       fetch 封装、SSE 解析、USE_MOCK 开关
    auth.ts notebooks.ts sources.ts chat.ts notes.ts studio.ts
    mock/           模拟数据与模拟后端
  store/          zustand 全局状态（useWorkspaceStore 为工作台核心状态）
  components/     通用 UI 组件（Button、Modal、Dropdown…）与布局
  features/       业务模块：home / sources / chat / studio
  pages/          路由页面：AuthPage（登录 / 注册）、HomePage、NotebookPage、NotFound
  types/          领域模型类型定义
```

## 对接后端

仓库根目录可直接运行 `./scripts/dev.sh` 一起启动数据库、后端和真实前端；配置示例见 [根 README](../README.md)。

1. 在项目根目录运行 `docker compose up -d`，随后在 `backend/` 运行 `go run ./cmd/server`。没有配置模型密钥时后端使用 Fake LLM。
2. 在 `frontend/` 运行：

   ```bash
   VITE_USE_MOCK=false npm run dev
   ```

   也可按 `.env.example` 配置两个公开变量。`VITE_USE_MOCK` 默认 `true`；`VITE_API_BASE_URL` 默认 `/api`。变量在启动 / 构建时生效，修改后需重启；真实模式生产构建使用 `VITE_USE_MOCK=false npm run build`。
3. 浏览器打开 `http://localhost:5173`，注册邮箱、至少 6 位密码与昵称；注册后自动登录。退出后可用同一邮箱密码登录。
4. `/api` 已在 `vite.config.ts` 代理至 `http://localhost:8080`。如另用前端端口，后端启动时将 `CORS_ORIGINS` 设置为对应地址（例如 `CORS_ORIGINS=http://localhost:5175 go run ./cmd/server`）。生产部署需提供 SPA 路由回退，并将 `/api` 反向代理到后端，或配置完整 API 地址与对应 CORS。

请求统一解包 `{ code: 0, data, message }`，使用 `Authorization: Bearer <token>`；token 存在 `localStorage['kn-token']`，读写不可用时仍支持当前会话。刷新后通过 `/me` 恢复用户，网络故障可重试，401 会清除会话并跳登录页。业务路由保留 `redirect`，登录后回到原页；已登录进入认证页返回首页。请求失败统一显示后端中文 toast，登录表单另有行内提示。

文件通过带鉴权的 XHR 上传并显示进度；文件 / 网页 / 文本创建后每 1.5 秒查询来源状态，直到 `ready` 或 `error`。失败条目显示红色原因提示并支持删除；刷新时继续轮询尚未完成的来源。切换笔记本或退出会取消当前轮询与对话连接。

SSE 支持 UTF-8 字符分包、LF / CRLF、多行 data、错误事件与主动停止。停止会保留已有内容并立即恢复输入；尚未取得服务端消息 ID 的部分回答需要刷新后才能反馈。设置弹窗读取并 PATCH `/me/settings`，保留四个现有模型选项，保存失败时保留原设置。


对话 / 笔记 / 工作室接口已完成真实联调；接口临时不可用时显示局部加载失败提示，真实模式不会退回 mock 数据。完整接口以 [`docs/api.md`](../docs/api.md) 为准。

### 必要验证

```bash
npm run build     # TypeScript 检查和生产构建
npm run test:api  # 鉴权、SSE、上传与轮询、并发及取消状态的确定性验证
E2E_BASE_URL=http://localhost:5173 npm run test:e2e  # 先启动真实系统，使用 Fake 模型验收完整流程
```

### 已对接接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/auth/register`、`/auth/login` | 注册 / 登录 |
| GET | `/me` | 恢复当前用户 |
| GET / POST | `/notebooks` | 笔记本列表 / 新建 |
| GET / PATCH / DELETE | `/notebooks/:id` | 详情 / 修改 / 删除 |
| GET | `/notebooks/:id/guide` | 笔记本概要与推荐问题 |
| GET | `/notebooks/:id/sources` | 来源列表 |
| POST | `/notebooks/:id/sources/upload` | 上传文件（multipart，字段 `file`） |
| POST | `/notebooks/:id/sources` | 添加网址 / 文本来源 |
| GET / PATCH / DELETE | `/sources/:id` | 查询解析状态 / 重命名 / 删除 |
| GET / DELETE | `/notebooks/:id/messages` | 对话历史 / 清空 |
| POST | `/notebooks/:id/chat` | **流式问答（SSE）**，body：`{ question, sourceIds }` |
| POST | `/messages/:id/feedback` | 回答反馈 |
| GET / POST | `/notebooks/:id/notes` | 笔记列表 / 新建 |
| PATCH / DELETE | `/notes/:id` | 修改 / 删除笔记 |
| GET / POST | `/notebooks/:id/artifacts` | 生成内容列表 / 生成（body：`{ kind }`） |
| DELETE | `/artifacts/:id` | 删除生成内容 |
| GET / PATCH | `/me/settings` | 用户设置 |

SSE 事件格式（每条 `data: <json>\n\n`）：

```json
{ "type": "citations", "citations": [{ "index": 1, "sourceId": "00000000-0000-4000-8000-000000000001", "sourceTitle": "...", "quote": "..." }] }
{ "type": "delta", "text": "部分回答文本，引用写作 [1]" }
{ "type": "done", "messageId": "00000000-0000-4000-8000-000000000002" }
{ "type": "error", "message": "生成失败，请重试" }
```

## 语音输入与用户头像

对话输入框支持录完再转文字，转写结果填入输入框，由用户确认后发送。真实模式通过鉴权的 `GET /api/speech/config` 判断是否显示麦克风；未配置 ASR 时隐藏。浏览器需允许麦克风并运行在安全上下文（HTTPS 或 localhost），录音最长 60 秒，编码为 16kHz 单声道 16-bit WAV，通过 `POST /api/speech/transcribe` 上传（字段 `audio`，最大 4MB）。音频与转写文本不落盘、不写日志。音频来源上传仍保留，来源自动转写尚未实现。

后端使用阿里云百炼 Qwen-ASR，环境变量为 `ASR_BASE_URL`、`ASR_API_KEY`、`ASR_MODEL`（默认 `qwen3-asr-flash`）；前两项缺省时，仅当 `EMBED_BASE_URL` 域名含 `aliyuncs.com` 才分别回退到向量配置。`scripts/dev.sh` 已允许注入 `ASR_*`。每用户每分钟最多 10 次，每次占用一次共享请求额度，Token 按上游用量或秒数估算。

账户菜单和偏好设置展示用户头像，偏好设置可上传或移除。`POST /api/me/avatar` 接收字段 `avatar`（PNG/JPEG，最大 2MB），后端居中裁切并规范化为 256×256 JPEG，经统一存储接口写入本地或私有 OSS；`DELETE /api/me/avatar` 移除头像。`GET /api/files/avatar/:userId` 需鉴权（支持 `?token=`），仅可读取本人头像，跨用户返回 404，OSS 使用 10 分钟签名跳转。mock 模式头像使用会话内对象 URL，刷新后不保留；语音输入返回固定演示文字，不调用云端识别。
