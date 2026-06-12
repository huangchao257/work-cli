#!/usr/bin/env bash
# 防抖调度：与 CodeGraph 默认 2s 窗口对齐，静默结束后执行 sync-agents.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${WORK_PROJECT_ROOT:-${1:-.}}"
PROJECT_ROOT="$(cd "$PROJECT_ROOT" 2>/dev/null && pwd || pwd)"

# 与 CodeGraph 一致，可用 AGENTS_SYNC_DEBOUNCE_MS 或 CODEGRAPH_WATCH_DEBOUNCE_MS 覆盖
DEBOUNCE_MS="${AGENTS_SYNC_DEBOUNCE_MS:-${CODEGRAPH_WATCH_DEBOUNCE_MS:-2000}}"
if [[ "$DEBOUNCE_MS" -lt 100 ]]; then DEBOUNCE_MS=100; fi
if [[ "$DEBOUNCE_MS" -gt 60000 ]]; then DEBOUNCE_MS=60000; fi

STATE_DIR="$PROJECT_ROOT/.codegraph/agents-sync"
mkdir -p "$STATE_DIR"

now_ms() {
  date +%s%3N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1000))'
}

touch "$STATE_DIR/pending"
echo "$(now_ms)" > "$STATE_DIR/pending"

if [[ -f "$STATE_DIR/worker.pid" ]]; then
  pid="$(cat "$STATE_DIR/worker.pid" 2>/dev/null || true)"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    exit 0
  fi
fi

(
  while true; do
    sleep 0.25
    pending_at="$(cat "$STATE_DIR/pending" 2>/dev/null || echo 0)"
    current="$(now_ms)"
    elapsed=$((current - pending_at))
    if [[ "$elapsed" -ge "$DEBOUNCE_MS" ]]; then
      break
    fi
  done
  bash "$SCRIPT_DIR/sync-agents.sh" "$PROJECT_ROOT" >>"$STATE_DIR/sync.log" 2>&1 || true
) &

echo $! > "$STATE_DIR/worker.pid"
