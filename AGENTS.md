# AGENTS.md — KnowNote 项目协作规则（Codex 每次开始前必读）

## 项目
知页 KnowNote：仿 NotebookLM 的 AI 个人资料管理系统（毕业设计）。
- `frontend/` React 18 + TS + Vite + Tailwind + zustand（UI 已完成）
- `backend/` Go + Gin + GORM + PostgreSQL/pgvector
- `docs/api.md` **前后端接口契约，唯一事实来源**；`docs/开发计划.md` 任务拆分与进度
- 界面文案、错误信息、代码注释、文档一律用**中文**；标识符用英文。

## 工作方式
1. 只做派给你的任务（Tn），只改任务指定的目录；需要改契约或越界时，先在 `docs/progress/Tn.md` 的“待协调”里写明，不要擅自改。
2. 开始前读：本文件、`docs/api.md`、`docs/开发计划.md`、`docs/progress/` 下已有的交付说明。
3. 完成后：
   - 运行验证：后端 `cd backend && go build ./... && go vet ./... && go test ./...`；前端 `cd frontend && npm run build`。
   - 写 `docs/progress/Tn.md`：做了什么、关键文件、如何验证、已知问题、待协调事项。
   - 在 `docs/开发计划.md` 表格里把该任务状态改为“已完成”。
   - 本地提交：`git add` 只加你改动的路径，提交信息格式 `Tn: 简述`。若遇到 `.git/index.lock`，等待几秒重试。**禁止 push、禁止改写历史（rebase/reset --hard/force）。**
4. 不确定的产品细节按 `docs/api.md` 与前端现有行为决定，不要停下来提问。

## 安全
- 绝不读取、打印或提交任何密钥：`~/.codex/`、`~/.ssh/`、任何 `.env`（只能读写 `.env.example`）。
- API Key 只从环境变量读取；未配置时使用 Fake LLM。
- 不要安装全局软件、不要修改系统设置；Go/npm 依赖装在项目内即可。

## 后端约定（Go）
- 目录：
  ```
  backend/
    cmd/server/main.go
    internal/config      环境变量配置（见 backend/.env.example）
    internal/db          连接、AutoMigrate、pgvector 扩展与索引
    internal/model       GORM 模型（UUID 主键、CreatedAt/UpdatedAt）
    internal/middleware  CORS、日志、Recover、JWT
    internal/handler     HTTP 层（参数校验、调用 service、统一响应）
    internal/service     业务逻辑
    internal/llm         Client 接口：Chat / ChatStream / Embed / TTS；OpenAI 兼容实现 + Fake 实现
    internal/parser      各格式正文提取
    internal/rag         切分、检索、提示词
    internal/storage     本地文件存储（默认 backend/data/）
    pkg/response         {code,data,message} 封装
  ```
- 统一响应与错误码：成功 `code=0`；业务错误用 HTTP 状态码 + 非 0 code（如 40001 参数错误、40101 未登录、40401 不存在、40901 冲突、50001 服务异常）。
- 所有查询必须带 user_id 约束，跨用户访问返回 404。
- JSON 字段 camelCase，时间 RFC3339，ID 为 UUID 字符串，与 `frontend/src/types/index.ts` 一致。
- 依赖首选：gin、gorm + gorm.io/driver/postgres、github.com/pgvector/pgvector-go、golang-jwt/jwt/v5、golang.org/x/crypto/bcrypt、github.com/sashabaranov/go-openai、github.com/ledongthuc/pdf、github.com/go-shiori/go-readability。
- 测试：纯逻辑（切分、提示词、JSON 解析、Fake LLM）写单元测试；依赖数据库的测试在 `DATABASE_URL` 不可用时 `t.Skip`。

## 前端约定
- 组件只调用 `src/api/`，不直接 fetch；类型改动同步 `src/types/index.ts`。
- 保持现有设计语言（Tailwind 变量色、圆角、中文文案），新增页面风格与现有页面一致。
- mock 模式保留（`VITE_USE_MOCK=true` 时仍可离线演示）。

## 本地环境
- Go 1.27、Node 24、Python 3.12、Docker（数据库：`docker compose up -d`，连接串 `postgres://knownote:knownote@localhost:5432/knownote?sslmode=disable`）。
- 端口：后端 8080，前端 5173。
