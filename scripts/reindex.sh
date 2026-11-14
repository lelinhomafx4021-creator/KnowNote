#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# 仅使用进程环境，不读取密钥文件；先停止业务服务，再注入目标 EMBED_* 与 EMBEDDING_DIM。
cd "$project_root/backend"
exec go run ./cmd/reindex
