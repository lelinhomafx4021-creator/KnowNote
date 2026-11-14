# 知页 KnowNote · AI 个人资料管理系统（前端）

参考 Google NotebookLM 的交互设计，基于 **React 18 + TypeScript + Vite + Tailwind CSS** 实现的 AI 个人资料管理系统前端。

## 快速开始

```bash
npm install
npm run dev      # http://localhost:5173
npm run build    # 类型检查 + 生产构建，输出到 dist/
```

默认使用前端内置模拟数据（`VITE_USE_MOCK=true`），无需后端即可完整演示。

## 功能一览

| 模块 | 功能 |
|---|---|
| 首页 | 笔记本网格 / 列表视图、搜索（⌘K）、排序、新建 / 重命名 / 删除 |
| 来源面板 | 上传文件（拖拽，PDF/Word/TXT/MD/音频）、网址 / 视频链接、粘贴文本；上传进度与解析状态；勾选参与问答的来源；来源详情（AI 摘要、关键主题、原文预览、引用高亮） |
| 对话面板 | 笔记本概要与推荐问题、流式回答、Markdown 渲染（含表格）、**引用角标**（悬停预览原文片段，点击跳转来源）、停止生成、复制 / 点赞 / 保存到笔记 |
| 工作室 | 音频概览（生成 + 播放器）、一键生成学习指南 / 简报 / FAQ / 时间线 / **思维导图**（SVG，可折叠、缩放、拖拽、全屏）、笔记编辑（Markdown，自动保存，可转为来源） |
| 通用 | 浅色 / 深色主题、设置（模型、回答风格、存储用量）、Toast、确认对话框、骨架屏、空状态 |
| 响应式 | ≥1024px 三栏（左右栏可收起为侧轨），移动端底部 Tab 切换「来源 / 对话 / 工作室」 |

## 目录结构

```
src/
  api/            接口层（组件只调用这里）
    client.ts       fetch 封装、SSE 解析、USE_MOCK 开关
    notebooks.ts sources.ts chat.ts notes.ts studio.ts
    mock/           模拟数据与模拟后端
  store/          zustand 全局状态（useWorkspaceStore 为工作台核心状态）
  components/     通用 UI 组件（Button、Modal、Dropdown…）与布局
  features/       业务模块：home / sources / chat / studio
  pages/          路由页面：HomePage、NotebookPage、NotFound
  types/          领域模型类型定义
```

## 对接后端

1. 复制 `.env.example` 为 `.env`，设置 `VITE_USE_MOCK=false`；
2. 开发环境下 `/api` 会被 Vite 代理到 `http://localhost:8080`（见 `vite.config.ts`）；
3. 后端统一返回 `{ code: 0, data, message }`，鉴权使用 `Authorization: Bearer <token>`（token 存于 `localStorage['kn-token']`）。

### 建议接口清单

| 方法 | 路径 | 说明 |
|---|---|---|
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
| GET / POST | `/notebooks/:id/audio` | 音频概览状态 / 生成 |
| GET / PATCH | `/me/settings` | 用户设置 |

SSE 事件格式（每条 `data: <json>\n\n`）：

```json
{ "type": "citations", "citations": [{ "index": 1, "sourceId": "src_1", "sourceTitle": "...", "quote": "..." }] }
{ "type": "delta", "text": "部分回答文本，引用写作 [1]" }
{ "type": "done", "messageId": "msg_xxx" }
```
