#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"

for command in docker go node npm; do
  command -v "$command" >/dev/null 2>&1 || { echo "缺少命令：$command，请先安装项目要求的本地环境" >&2; exit 1; }
done
docker info >/dev/null 2>&1 || { echo "Docker 未就绪，请先启动 Docker Desktop" >&2; exit 1; }
docker compose version >/dev/null

# 读取 backend/.env 中的模型与密钥配置（只按 KEY=VALUE 解析，不执行文件内容）。
# 终端里已设置的同名变量优先；端口与 CORS 由本脚本统一管理，不从文件读取。
if [[ -f backend/.env ]]; then
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%$'\r'}"
    [[ "$line" =~ ^[[:space:]]*(JWT_SECRET|APP_ENV|DATA_DIR|DATABASE_URL|EMBEDDING_DIM|SOFFICE_PATH|STORAGE_DRIVER|OSS_[A-Z_]+|LLM_[A-Z_]+)=(.*)$ ]] || continue
    key="${BASH_REMATCH[1]}"; value="${BASH_REMATCH[2]}"
    value="${value%\"}"; value="${value#\"}"; value="${value%\'}"; value="${value#\'}"
    if [[ -z "${!key:-}" && -n "$value" ]]; then export "$key=$value"; fi
  done < backend/.env
  echo "已读取 backend/.env 中的配置"
fi

export PORT="${PORT:-8080}"
export FRONTEND_PORT="${FRONTEND_PORT:-5173}"
if [[ "$PORT" != 8080 ]]; then
  echo "当前 Vite 代理固定连接 8080，请使用 PORT=8080" >&2
  exit 1
fi
if ! [[ "$FRONTEND_PORT" =~ ^[0-9]+$ ]] || (( FRONTEND_PORT < 1 || FRONTEND_PORT > 65535 )); then
  echo "FRONTEND_PORT 必须是 1 到 65535 的整数" >&2
  exit 1
fi
export CORS_ORIGINS="${CORS_ORIGINS:-http://localhost:$FRONTEND_PORT}"
export DATABASE_URL="${DATABASE_URL:-postgres://knownote:knownote@localhost:5432/knownote?sslmode=disable}"
export VITE_USE_MOCK=false
export VITE_API_BASE_URL=/api

# 不让 Vite 自动换端口，也不向原有服务发送退出信号。
node - "$PORT" "$FRONTEND_PORT" <<'JS'
const net = require('node:net');
(async () => {
  for (const port of process.argv.slice(2)) {
    await new Promise((resolve, reject) => {
      const server = net.createServer();
      server.once('error', () => reject(new Error(`端口 ${port} 已被占用，请停止原服务或设置 FRONTEND_PORT`)));
      server.listen(Number(port), () => server.close(resolve));
    });
  }
})().catch((error) => { console.error(error.message); process.exitCode = 1; });
JS

docker compose up -d
database_ready=false
for (( attempt=0; attempt<60; attempt++ )); do
  if docker compose exec -T postgres pg_isready -U knownote -d knownote >/dev/null 2>&1; then
    database_ready=true
    break
  fi
  sleep 1
done
[[ "$database_ready" == true ]] || { echo "等待数据库就绪超时" >&2; exit 1; }

if [[ ! -d frontend/node_modules ]]; then
  (cd frontend && npm ci)
fi

# POSIX 进程组覆盖 go run 的服务子进程与 npm 的 Vite 子进程。
# Node 作为监督进程接收 Ctrl-C，并等待后端优雅退出，避免残留监听进程。
exec node --input-type=commonjs - "$project_root" <<'JS'
const { spawn } = require('node:child_process');
const root = process.argv[2];
const children = [];
let stopping = false;
let hardStop;
function stop(code) {
  if (stopping) return;
  stopping = true;
  process.exitCode = code;
  console.log('\n正在停止前后端，数据库容器保留……');
  for (const child of children) {
    try { process.kill(-child.pid, 'SIGINT'); } catch (error) { if (error.code !== 'ESRCH') console.error('停止子进程失败'); }
  }
  hardStop = setTimeout(() => {
    for (const child of children) { try { process.kill(-child.pid, 'SIGKILL'); } catch {} }
  }, 15000);
  hardStop.unref();
}
function start(command, args, directory) {
  const child = spawn(command, args, { cwd: `${root}/${directory}`, stdio: 'inherit', detached: true });
  children.push(child);
  child.on('error', () => { console.error(`无法启动 ${directory}`); stop(1); });
  child.on('exit', (code) => {
    if (!stopping) { console.error(`${directory} 已退出，正在停止完整系统`); stop(code || 1); }
    if (children.every((item) => item.exitCode !== null || item.signalCode !== null)) clearTimeout(hardStop);
  });
}
process.on('SIGINT', () => stop(0));
process.on('SIGTERM', () => stop(0));
start('go', ['run', './cmd/server'], 'backend');
start('npm', ['run', 'dev', '--', '--host', 'localhost', '--port', process.env.FRONTEND_PORT, '--strictPort'], 'frontend');
console.log(`前端：http://localhost:${process.env.FRONTEND_PORT}；后端：http://localhost:8080；按 Ctrl-C 一起退出`);
JS
