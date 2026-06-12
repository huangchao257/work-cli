#!/usr/bin/env bash
# 安装时由 work 生成可执行包装脚本；此文件为套装占位，实际运行使用安装目录中的 telemetry.sh。
set -euo pipefail
input=$(cat)
work hooks report --ide "${WORK_HOOKS_IDE}" --event "${WORK_HOOKS_EVENT}" --hooks-kit "${WORK_HOOKS_KIT:-company-hooks}" <<< "$input" || true
printf '%s' "$input"
exit 0
