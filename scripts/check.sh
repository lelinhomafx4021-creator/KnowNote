#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root/backend"
echo "检查后端构建、静态分析与测试（数据库不可用时集成测试会跳过）"
go build ./...
go vet ./...
go test -count=1 -v ./...
cd "$project_root/frontend"
echo "检查前端传输状态与生产构建"
npm run test:api
npm run build
