# KnowNote 后端

Go 1.27，模块名 `knownote`，HTTP 默认监听 8080。接口以 [接口契约](../docs/api.md) 为准。认证、设置、笔记本、来源、RAG 对话、概要、笔记、工作室和音频路由均已接入，完整启动与配置见 [根 README](../README.md)。

## 启动

在项目根目录启动数据库，再进入后端：

```sh
docker compose up -d
cd backend
go run ./cmd/server
```

默认配置连接本机 `knownote-postgres`，没有模型密钥时使用 Fake LLM，没有 JWT 密钥时生成临时开发密钥并警告。重启服务后临时密钥会变化，需重新登录。

**程序只读取进程环境变量，不自动读取 `.env`。** 在启动终端或部署环境注入变量，不要把密钥提交到 Git。全部配置见 `.env.example`；`APP_ENV` 支持 `development`（默认）、`test`、`production`。生产环境必须设置至少 32 字节的 `JWT_SECRET`。`CORS_ORIGINS` 为逗号分隔的来源地址，默认 `http://localhost:5173`。

启动自动创建 vector 扩展、十张表及 HNSW 余弦索引，需要建扩展和建表权限。`EMBEDDING_DIM` 默认为 1536，支持 1～2000；已有 chunks 维度与配置不一致时拒绝启动，避免改变已有数据。更换向量模型时须保证维度一致，并由后续迁移重新向量化已有内容。

## 验证

```sh
go build ./...
go vet ./...
go test ./...
curl http://localhost:8080/api/health
```

集成测试直接使用 `DATABASE_URL`，未设置时尝试示例中的本机数据库；无法连接时 Skip，可连接但迁移或业务行为错误时 Fail。测试使用随机邮箱，结束后级联清理自身数据。请使用开发或测试数据库。

## 主要模块

- `cmd/server/main.go`：配置、数据库、模型客户端、业务服务和 HTTP 生命周期组装；收到 SIGINT/SIGTERM 后最多等待 10 秒，随后关闭连接池。
- `internal/handler/router.go`：`NewRouter(..., registrars...)` 组装来源、对话与工作室路由注册器；注册器获得已带 JWT 的 `/api` 路由组。
- `internal/service`：现有业务逻辑；`Service.Files` 已注入本地存储清理器。数据库删除提交后调用清理器，传入当前用户、笔记本和所有来源/音频文件路径。清理器支持幂等与失败重试。
- `internal/model`：十张表、中文引用/导图结构与 JSONB 序列化。`Source.Content` 保存完整正文，接口预览截取前 3000 字；`FilePath` 为内部字段。使用 `model.NewVector([]float32)` 写入 pgvector，读取通过 `.Slice()`。
- `internal/llm`：通过 `llm.New(config)` 选择 Fake 或 OpenAI 兼容实现。`ChatOptions` 可覆盖模型、温度、最大输出 token。`ChatStream` 返回完整或已生成的部分文字，调用方负责保存。

OpenAI 兼容实现的 HTTP 超时为 60 秒，单次方法总预算为 90 秒；最多请求 3 次，仅重试网络错误、408、429、5xx。建立后的流不重试，避免重复内容。Embed 通常每批最多 64 条，百炼 `text-embedding-v*` 每批最多 10 条，按返回的 index 恢复顺序并校验维度；仅对 `text-embedding-3-*` 发送 dimensions 参数，其他模型须自身输出配置维度。TTS 返回 MP3 字节，未配置模型时返回 `llm.ErrNotConfigured`。单元测试使用本地 HTTP 模拟服务，不会调用付费接口。

Fake 向量通过英文词、中文单字和相邻字进行确定性特征哈希并归一化，用于离线演示，不具备真实语义模型的理解能力。Fake 回答从输入中提取编号片段并保留 `[n]` 引用，流式逐字回调；Fake 支持来源摘要/主题、概要、五种产物和双主持文稿；Fake TTS 始终返回未配置错误，音频概览展示脚本。

普通 JSON 请求最多 1MB，来源上传限制 200MiB。日志不记录请求体、Authorization、查询字符串和 SQL 参数；JWT 支持 Bearer 请求头及音频所需的 `?token=`，存在请求头时优先校验请求头。HTTP 服务不设置全局写超时，以支持 SSE 和耗时产物生成；生成逻辑使用请求上下文和模型自带超时。
