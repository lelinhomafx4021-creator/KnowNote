#!/usr/bin/env bash
# 用本机 Codex 执行一个开发任务（每个任务一个独立的 Codex 对话）
# 用法：scripts/run-codex-task.sh T1
set -euo pipefail
TASK="${1:?用法: $0 <任务编号，如 T1>}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CODEX="${CODEX_BIN:-/Applications/ChatGPT.app/Contents/Resources/codex-cli/CodexCLI.app/Contents/MacOS/codex}"
PROMPT="$ROOT/docs/tasks/$TASK.md"
LOG_DIR="$ROOT/.codex-logs"
mkdir -p "$LOG_DIR"
[ -f "$PROMPT" ] || { echo "找不到任务说明 $PROMPT"; exit 1; }
echo "[$(date '+%F %T')] 开始 $TASK" | tee -a "$LOG_DIR/runs.log"
"$CODEX" exec -C "$ROOT" -o "$LOG_DIR/$TASK.final.md" - < "$PROMPT" > "$LOG_DIR/$TASK.log" 2>&1
STATUS=$?
echo "[$(date '+%F %T')] 结束 $TASK，退出码 $STATUS" | tee -a "$LOG_DIR/runs.log"
exit $STATUS
