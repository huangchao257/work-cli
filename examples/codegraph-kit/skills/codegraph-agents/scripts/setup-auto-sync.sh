#!/usr/bin/env bash
# 兼容旧用法：请优先使用 work graph init
set -euo pipefail
WORK="${WORK_BIN:-work}"
if command -v "$WORK" >/dev/null 2>&1; then
  exec "$WORK" graph init "$@"
fi
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec bash "$SCRIPT_DIR/generate-agents.sh" "$@"
