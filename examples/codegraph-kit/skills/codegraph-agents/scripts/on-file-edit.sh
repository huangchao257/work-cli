#!/usr/bin/env bash
# Cursor afterFileEdit hook：透传 stdin，后台防抖更新 CodeGraph + AGENTS.md
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
input="$(cat)"

# 从 hook stdin 推断项目根（Cursor 会传入 file_path）
project_root="."
if command -v jq >/dev/null 2>&1; then
  file_path="$(printf '%s' "$input" | jq -r '.file_path // .path // empty' 2>/dev/null || true)"
  if [[ -n "$file_path" && "$file_path" != "null" ]]; then
    if [[ "$file_path" = /* ]]; then
      dir="$(dirname "$file_path")"
    else
      dir="$(dirname "$file_path")"
      dir="$(cd "$dir" 2>/dev/null && pwd || echo ".")"
    fi
    # 向上查找含 .codegraph 或 go.mod 的目录作为项目根
    while [[ "$dir" != "/" ]]; do
      if [[ -d "$dir/.codegraph" || -f "$dir/go.mod" || -f "$dir/package.json" ]]; then
        project_root="$dir"
        break
      fi
      dir="$(dirname "$dir")"
    done
  fi
fi

export WORK_PROJECT_ROOT="$project_root"
nohup bash "$SCRIPT_DIR/sync-agents-schedule.sh" "$project_root" >/dev/null 2>&1 &

printf '%s' "$input"
exit 0
