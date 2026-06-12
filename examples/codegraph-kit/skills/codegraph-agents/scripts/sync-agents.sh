#!/usr/bin/env bash
# 同步 CodeGraph 索引并重新生成 AGENTS.md（单次执行）
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${1:-.}"
PROJECT_ROOT="$(cd "$PROJECT_ROOT" && pwd)"

if ! command -v codegraph >/dev/null 2>&1; then
  exit 0
fi

cd "$PROJECT_ROOT"

if [[ "$(codegraph status --json 2>/dev/null | jq -r '.initialized // false')" != "true" ]]; then
  codegraph init -i >/dev/null 2>&1 || exit 0
else
  codegraph sync >/dev/null 2>&1 || true
fi

bash "$SCRIPT_DIR/generate-agents.sh" --quiet --skip-sync -p "$PROJECT_ROOT"
