# KnowNote 后端

Go 1.27，模块名 `knownote`，HTTP 默认监听 8080。接口以 [接口契约](../docs/api.md) 为准。认证、设置、笔记本、来源、RAG 对话、概要、笔记与工作室路由均已接入，完整启动与配置见 [根 README](../README.md)。

## 启动

在项目根目录启动数据库，再进入后端：

```sh
docker compose up -d
cd backend
go run ./cmd/server
```

默认配置连接本机 `knownote-postgres`，没有模型密钥时使用 Fake LLM，没有 JWT 密钥时生成临时开发密钥并警告。重启服务后临时密钥会变化，需重新登录。

**后端程序本身只读取进程环境变量。** 本地开发推荐把配置写进 `backend/.env`（已被 Git 忽略），用根目录的 `scripts/dev.sh` 启动，脚本会把其中的配置注入环境；单独 `go run` 时请在终端自行 `export`。不要把密钥提交到 Git。全部配置见 `.env.example`；`APP_ENV` 支持 `development`（默认）、`test`、`production`。生产环境必须设置至少 32 字节的 `JWT_SECRET`。`CORS_ORIGINS` 为逗号分隔的来源地址，默认 `http://localhost:5173`。

启动自动创建 vector 扩展、十一张表及 HNSW 余弦索引，需要建扩展和建表权限。`EMBEDDING_DIM` 默认为 1536，支持 1～2000；已有 chunks 维度与配置不一致时拒绝启动，避免改变已有数据。更换向量模型时须保证维度一致，并由后续迁移重新向量化已有内容。

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
- `internal/service`：现有业务逻辑；`Service.Files` 已注入本地存储清理器。数据库删除提交后调用清理器，传入当前用户、笔记本和所有来源文件路径。清理器支持幂等与失败重试。
- `internal/model`：十一张表、中文引用/导图结构与 JSONB 序列化。`Source.Content` 保存完整正文，接口预览截取前 3000 字；`FilePath` 为内部字段。使用 `model.NewVector([]float32)` 写入 pgvector，读取通过 `.Slice()`。
- `internal/llm`：通过 `llm.New(config)` 选择 Fake 或 OpenAI 兼容实现。`ChatOptions` 可覆盖模型、温度、最大输出 token。`ChatStream` 返回完整或已生成的部分文字，调用方负责保存。

OpenAI 兼容实现的 HTTP 超时为 60 秒，单次方法总预算为 90 秒；最多请求 3 次，仅重试网络错误、408、429、5xx。建立后的流不重试，避免重复内容。Embed 通常每批最多 64 条，百炼 `text-embedding-v*` 每批最多 10 条，按返回的 index 恢复顺序并校验维度；仅对 `text-embedding-3-*` 发送 dimensions 参数，其他模型须自身输出配置维度。TTS 返回 MP3 字节，未配置模型时返回 `llm.ErrNotConfigured`。单元测试使用本地 HTTP 模拟服务，不会调用付费接口。

Fake 向量通过英文词、中文单字和相邻字进行确定性特征哈希并归一化，用于离线演示，不具备真实语义模型的理解能力。Fake 回答从输入中提取编号片段并保留 `[n]` 引用，流式逐字回调；Fake 支持来源摘要/主题、概要、五种产物；Fake TTS 始终返回未配置错误。

普通 JSON 请求最多 1MB，来源上传限制 200MiB。日志不记录请求体、Authorization、查询字符串和 SQL 参数；JWT 支持 Bearer 请求头及原件预览与下载所需的 `?token=`，存在请求头时优先校验请求头。HTTP 服务不设置全局写超时，以支持 SSE 和耗时产物生成；生成逻辑使用请求上下文和模型自带超时。

## 多格式解析与原件预览

支持 PDF、DOC/DOCX、PPT/PPTX、XLS/XLSX、CSV。PDF 使用内嵌 PDFium WebAssembly（纯 Go 运行时，无 CGO），逐页提取中英文文字、清理边缘重复页眉页脚和英文连字符断行；不做 OCR。PPTX 按展示顺序保留标题、列表、表格和备注；DOCX 按标题分节；表格每工作表最多读取 2000 行数据（不含列名），截断说明写入摘要。

旧 Office 格式与 Word/PPT 的 PDF 预览需要 LibreOffice。`SOFFICE_PATH` 可指定可执行文件；留空依次查找 PATH 中的 `soffice`、`libreoffice` 和 `/Applications/LibreOffice.app/Contents/MacOS/soffice`。每次转换使用独立临时配置目录，超时 120 秒，全进程最多两个并发转换。无需安装额外系统软件。未找到转换器时旧版来源返回中文错误，新版正文解析可用，预览失败只记日志并返回 `previewable=false`。

**中文字体**：无界面运行的 LibreOffice 常常读不到系统字体，转出的预览 PDF 中文会变成空白。转换时会自动把本机常见中文字体（macOS 的冬青黑体/华文黑体/宋体/Arial Unicode，Linux 的 Noto CJK/文泉驿，Windows 的微软雅黑/宋体）以及 `backend/fonts/` 下的字体链接进本次转换的配置目录。Linux 服务器请先安装中文字体（如 `apt install fonts-noto-cjk`），或把字体文件放进 `backend/fonts/`，也可用 `SOFFICE_FONT_PATHS` 指定。

`GET /api/sources/:id/preview` 返回 inline PDF，`GET /api/sources/:id/file` 下载原件，均支持 Bearer 或 `?token=`；local 使用 Range 响应，OSS 返回 10 分钟有效的签名跳转。PDF 预览直接复用原件。新版文件键为 `uploads/<userID>/<sourceID>/original.<ext>`、同目录 `preview.pdf`；local 继续读取既有数据库记录中的旧文件名。DOCX 章节序号不代表物理页码，因此其 citation 提供章节标签，不返回 page；PDF、幻灯片和工作表的 citation 同时包含 page 与 locationLabel。

## 阿里云 OSS 配置

1. 在阿里云 OSS 控制台创建 Bucket，地域与后端配置一致，读写权限选择**私有**。本应用不需要公开读权限，也不负责创建 Bucket。
2. 创建专用 RAM 子账号，仅启用程序访问；为它绑定下面的自定义权限策略。将 `YOUR_BUCKET` 替换为 Bucket 名称，若更改 `OSS_PREFIX`，同步替换 `knownote/`。后端只需要在该前缀读写、删除对象，以及列举来源目录，不需要管理 Bucket、ACL 或列举所有 Bucket。

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["oss:GetObject", "oss:PutObject", "oss:DeleteObject"],
      "Resource": ["acs:oss:*:*:YOUR_BUCKET/knownote/*"]
    },
    {
      "Effect": "Allow",
      "Action": ["oss:ListObjects"],
      "Resource": ["acs:oss:*:*:YOUR_BUCKET"],
      "Condition": {"StringLike": {"oss:Prefix": ["knownote/uploads/*"]}}
    }
  ]
}
```

策略的 Bucket 与目录授权方式见[阿里云官方 RAM 文档](https://www.alibabacloud.com/help/en/oss/user-guide/access-control-base-on-ram-policy)。

3. 通过进程环境或本地开发启动脚本配置下列变量（真实值不要提交）：

| 变量 | 说明 |
|---|---|
| `STORAGE_DRIVER` | `oss`；留空或 `local` 使用本地 `DATA_DIR` |
| `OSS_REGION` | 必填，例如 `cn-hangzhou` |
| `OSS_BUCKET` | 必填，私有 Bucket 名称 |
| `OSS_ACCESS_KEY_ID` / `OSS_ACCESS_KEY_SECRET` | 必填，专用 RAM 子账号凭据 |
| `OSS_ENDPOINT` | 可选，自定义或内网域名；签名跳转也使用此域名，浏览器需要能访问 |
| `OSS_PREFIX` | 默认 `knownote/`，不要设置为根目录 |
| `DATA_DIR` | OSS 模式仍用于保存 `.oss-cleanup/` 清理日志，需持久化 |

4. 启动后确认日志 `文件存储已启用` 的 driver 为 `oss`。原件与 PDF 预览全部写入存储接口，对象键分别为 `knownote/uploads/<userID>/<sourceID>/original.<ext>`、同目录 `preview.pdf`。解析时下载至临时目录，结束后删除；删除业务数据后清理关联对象，失败写本地日志，每 5 秒重试，重启后恢复。切换驱动不自动搬迁旧数据。

当前预览 iframe 与文件下载通过后端鉴权后跳转到签名 URL，**无需为这些用法配置 OSS CORS**。若部署在公网，请使用浏览器可达的 HTTPS Endpoint；内网 Endpoint 仅适合客户端也能访问该网络的部署。

测试通过 `httptest` 模拟官方 SDK 的 Put/Get/Head/Delete/List、分页删除和签名；真实 OSS 验证默认跳过。显式设置 `T7_REAL_OSS=1`、OSS 环境变量后，可运行 `go test ./internal/storage -run TestRealOSS -v`。此测试只创建并删除自己的随机临时对象。测试样例由 `internal/testfixture` 现场生成，可用 `T7_FIXTURE_DIR=/tmp/knownote-fixtures go test ./internal/parser -run 'TestGeneratedDocuments|TestLibreOffice' -v` 导出样例用于 curl 验收。

## T9 分能力模型与共享用量

真实模型的完整配置见[根 README](../README.md#配置真实模型)与 `.env.example`。`CHAT_*`、`EMBED_*`、`TTS_*` 独立覆盖旧 `LLM_*` 回退配置，没有该能力的 Key 就使用 Fake。设置模型取自鉴权接口 `GET /api/models`，今日用量取自 `GET /api/me/usage`。

更换向量模型或维度：停止服务，导出目标配置，运行 `../scripts/reindex.sh`。重建保留原文和所有引用位置，在单个数据库事务中替换所有向量与 HNSW 索引；失败回滚，命令不会读取 `.env`。维护需要覆盖所有用户，是显式运维入口；仍记录用量，必要时仅给该进程设置 `QUOTA_DAILY_TOKENS=0`。批次与探测仅调用配置的向量服务。

`llm_usage` 保存用户、能力、模型、输入/输出 Token、耗时、成功状态；待完成行保存预留预算。数据库用户级事务锁保护额度与一分钟窗口，进程内信号量限制总并发。异常中断超过五分钟的预留在该用户下次调用时按预算保守结算。上游 429/5xx 最多三次尝试，已建立的流不重试，错误不含原始响应和凭据。

图像暂只提供 `llm.ImageClient` 与 `llm.NewImage(config, store)`，无业务路由；OpenAI 兼容 `/images/generations` 支持 b64/url，生成文件通过 Store 写入 local/OSS。`IMAGE_PROVIDER=dashscope` 预留兼容模式配置，原生异步任务协议未来单独适配。Fake PNG 优先系统中文字体，无字体时显示提示词 Unicode 转义。

## 语音输入与用户头像

对话输入框支持录完再转文字，转写结果填入输入框，由用户确认后发送。真实模式通过鉴权的 `GET /api/speech/config` 判断是否显示麦克风；未配置 ASR 时隐藏。浏览器需允许麦克风并运行在安全上下文（HTTPS 或 localhost），录音最长 60 秒，编码为 16kHz 单声道 16-bit WAV，通过 `POST /api/speech/transcribe` 上传（字段 `audio`，最大 4MB）。音频与转写文本不落盘、不写日志。音频来源上传仍保留，来源自动转写尚未实现。

后端使用阿里云百炼 Qwen-ASR，环境变量为 `ASR_BASE_URL`、`ASR_API_KEY`、`ASR_MODEL`（默认 `qwen3-asr-flash`）；前两项缺省时，仅当 `EMBED_BASE_URL` 域名含 `aliyuncs.com` 才分别回退到向量配置。`scripts/dev.sh` 已允许注入 `ASR_*`。每用户每分钟最多 10 次，每次占用一次共享请求额度，Token 按上游用量或秒数估算。

账户菜单和偏好设置展示用户头像，偏好设置可上传或移除。`POST /api/me/avatar` 接收字段 `avatar`（PNG/JPEG，最大 2MB），后端居中裁切并规范化为 256×256 JPEG，经统一存储接口写入本地或私有 OSS；`DELETE /api/me/avatar` 移除头像。`GET /api/files/avatar/:userId` 需鉴权（支持 `?token=`），仅可读取本人头像，跨用户返回 404，OSS 使用 10 分钟签名跳转。mock 模式头像使用会话内对象 URL，刷新后不保留；语音输入返回固定演示文字，不调用云端识别。
