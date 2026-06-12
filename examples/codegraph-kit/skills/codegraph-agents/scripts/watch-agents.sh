#!/usr/bin/env bash
# 文件系统监听：不依赖 IDE hook，在终端保持运行时自动同步 AGENTS.md
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="."

while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--path)
      PROJECT_ROOT="$2"
      shift 2
      ;;
    -h|--help)
      echo "用法: watch-agents.sh [-p 项目路径]"
      exit 0
      ;;
    *)
      PROJECT_ROOT="$1"
      shift
      ;;
  esac
done

PROJECT_ROOT="$(cd "$PROJECT_ROOT" && pwd)"
cd "$PROJECT_ROOT"

echo "监听 $PROJECT_ROOT 源码变更（防抖 2s，Ctrl+C 停止）..."

watch_with_inotify() {
  inotifywait -r -m -e modify,create,delete,move --format '%w%f' \
    --exclude '(/\.codegraph/|/node_modules/|/vendor/|/dist/|/build/|/target/|/\.git/)' \
    "$PROJECT_ROOT" 2>/dev/null | while read -r _; do
    bash "$SCRIPT_DIR/sync-agents-schedule.sh" "$PROJECT_ROOT"
  done
}

watch_with_fswatch() {
  fswatch -r \
    --exclude '\.codegraph' \
    --exclude 'node_modules' \
    --exclude 'vendor' \
    --exclude '/dist/' \
    --exclude '/build/' \
    --exclude '/target/' \
    --exclude '\.git' \
    "$PROJECT_ROOT" | while read -r _; do
    bash "$SCRIPT_DIR/sync-agents-schedule.sh" "$PROJECT_ROOT"
  done
}

if command -v inotifywait >/dev/null 2>&1; then
  watch_with_inotify
elif command -v fswatch >/dev/null 2>&1; then
  watch_with_fswatch
else
  echo "错误: 需要 inotifywait（linux: sudo apt install inotify-tools）或 fswatch（macOS: brew install fswatch）" >&2
  exit 1
fi
