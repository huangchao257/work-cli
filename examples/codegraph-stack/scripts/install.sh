#!/usr/bin/env bash
# 一键安装 CodeGraph + 技能套装 + 项目初始化（无感配置）
set -euo pipefail

WORK="${WORK_BIN:-work}"

echo "▸ 安装 CodeGraph CLI ..."
npm install -g @colbymchenry/codegraph@latest

echo "▸ 安装 IDE 技能与 MCP ..."
"$WORK" install codegraph-kit --scope project --no-auto-update

echo "▸ 初始化项目知识图谱与 AGENTS.md ..."
"$WORK" graph init --no-auto-update

echo ""
echo "✓ 完成。保存代码后 AGENTS.md 将自动更新；重启 IDE 以启用 CodeGraph MCP。"
