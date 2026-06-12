#!/usr/bin/env bash
# 基于 CodeGraph 知识图谱，为项目各「有意义」目录生成 AGENTS.md
set -euo pipefail

PROJECT_ROOT="."
DRY_RUN=false
QUIET=false
SKIP_SYNC=false

usage() {
  cat <<'EOF'
用法: generate-agents.sh [选项] [项目路径]

选项:
  -p, --path <dir>   项目根目录（默认当前目录）
  --dry-run          仅打印将写入的文件，不实际写入
  --quiet            静默模式（供自动同步使用）
  --skip-sync        跳过 codegraph sync（调用方已同步时使用）
  -h, --help         显示帮助

依赖: codegraph, jq
EOF
}

log() {
  $QUIET || echo "$@"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--path)
      PROJECT_ROOT="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --quiet)
      QUIET=true
      shift
      ;;
    --skip-sync)
      SKIP_SYNC=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      echo "未知选项: $1" >&2
      usage >&2
      exit 1
      ;;
    *)
      PROJECT_ROOT="$1"
      shift
      ;;
  esac
done

if ! command -v codegraph >/dev/null 2>&1; then
  echo "错误: 未找到 codegraph。请先执行: work install codegraph-stack" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "错误: 未找到 jq。请安装 jq 后重试。" >&2
  exit 1
fi

PROJECT_ROOT="$(cd "$PROJECT_ROOT" && pwd)"
cd "$PROJECT_ROOT"

log "项目: $PROJECT_ROOT"

INIT_STATUS="$(codegraph status --json 2>/dev/null || echo '{}')"
if [[ "$(echo "$INIT_STATUS" | jq -r '.initialized // false')" != "true" ]]; then
  log "正在初始化 CodeGraph 索引..."
  if $QUIET; then
    codegraph init -i >/dev/null 2>&1
  else
    codegraph init -i
  fi
elif ! $SKIP_SYNC; then
  log "同步 CodeGraph 索引..."
  if $QUIET; then
    codegraph sync >/dev/null 2>&1 || true
  else
    codegraph sync 2>/dev/null || true
  fi
fi

FILES_JSON="$(codegraph files --json)"
FUNCS_JSON="$(codegraph query "" --kind function --limit 3000 --json 2>/dev/null || echo '[]')"
STRUCTS_JSON="$(codegraph query "" --kind struct --limit 1000 --json 2>/dev/null || echo '[]')"
CLASSES_JSON="$(codegraph query "" --kind class --limit 1000 --json 2>/dev/null || echo '[]')"

# 跳过这些目录前缀
should_skip_dir() {
  local d="$1"
  case "$d" in
    .git/*|.git|node_modules/*|node_modules|vendor/*|vendor|dist/*|dist|\
build/*|build|target/*|target|.codegraph/*|.codegraph|\
.goreleaser/*|.github/*)
      return 0
      ;;
  esac
  return 1
}

# 目录用途（路径启发式，可按项目扩展）
dir_purpose() {
  local dir="$1"
  case "$dir" in
    .) echo "项目根目录：全局配置、文档与示例资源" ;;
    cmd|cmd/*) echo "程序入口：main 函数与 CLI 根命令启动" ;;
    internal/cli|internal/cli/*) echo "CLI 命令层：用户可见子命令、全局参数与帮助" ;;
    internal/engine|internal/engine/*) echo "业务编排层：install / list / uninstall / update 核心流程" ;;
    internal/adapter|internal/adapter/*) echo "IDE 适配层：向 Cursor / Qoder / Claude 写入 Skills / Rules / MCP" ;;
    internal/bundle|internal/bundle/*) echo "资源套装解析：bundle.yaml 读取与校验" ;;
    internal/hooks|internal/hooks/*) echo "Hooks 模块：事件模型、脱敏与上报" ;;
    internal/state|internal/state/*) echo "安装状态：installed.json 持久化" ;;
    internal/source|internal/source/*) echo "包来源：Registry / Git / 本地路径解析" ;;
    internal/platform|internal/platform/*) echo "跨平台路径、IDE 探测与环境提示" ;;
    internal/selfupdate|internal/selfupdate/*) echo "work 自身版本检查与自动更新" ;;
    internal/output|internal/output/*) echo "终端输出：人类可读与 --json 格式" ;;
    internal/installer|internal/installer/*) echo "外部 CLI：installer.yaml 解析与命令执行" ;;
    internal/pkg|internal/pkg/*) echo "内部共享工具包" ;;
    examples|examples/*) echo "示例套装：供 work install 引用的 bundle / cli 样例" ;;
    docs|docs/*) echo "设计与实现文档" ;;
    pkg|pkg/*) echo "可被外部引用的公共库" ;;
    api|api/*) echo "API 层：HTTP/RPC 接口定义与路由" ;;
    web|web/*|frontend|frontend/*) echo "前端代码" ;;
    test|tests|*_test|*/*_test) echo "测试代码" ;;
    *)
      if [[ "$dir" == *test* ]]; then
        echo "测试相关代码"
      else
        echo "源码目录"
      fi
      ;;
  esac
}

# AI 操作指引行（任务 → 去哪里改）
task_hints() {
  local dir="$1"
  case "$dir" in
    .)
      echo "| 了解项目目标 | \`README.md\`、\`docs/\` |"
      echo "| 修改发布配置 | \`.goreleaser.yaml\`、\`.github/workflows/\` |"
      ;;
    cmd|cmd/*)
      echo "| 修改程序入口 | \`main.go\` |"
      ;;
    internal/cli|internal/cli/*)
      echo "| 新增/修改子命令 | 在本目录添加或编辑 \`*_cmd.go\` / 命令文件 |"
      echo "| 修改全局参数 | \`root.go\` |"
      echo "| 修改中文帮助 | \`help.go\` |"
      ;;
    internal/engine|internal/engine/*)
      echo "| 修改安装/卸载/更新逻辑 | 本目录对应 \`*.go\` |"
      echo "| 新增安装类型 | \`install.go\` 分发 + 新 \`*_install.go\` |"
      ;;
    internal/adapter|internal/adapter/*)
      echo "| 支持新 IDE 或修改安装路径 | 新增 \`*_adapter.go\` 或编辑现有适配器 |"
      echo "| MCP 配置合并 | \`mcp_merge.go\` |"
      ;;
    internal/bundle|internal/bundle/*)
      echo "| 扩展 bundle.yaml 字段 | \`manifest.go\`、\`parse.go\` |"
      ;;
    internal/hooks|internal/hooks/*)
      echo "| 修改事件模型或上报 | 本目录 \`*.go\` |"
      ;;
    internal/state|internal/state/*)
      echo "| 修改已安装记录结构 | \`types.go\`、\`store.go\` |"
      ;;
    internal/source|internal/source/*)
      echo "| 新增包来源类型 | \`resolver.go\` + 新解析器文件 |"
      ;;
    internal/platform|internal/platform/*)
      echo "| 修改 IDE 路径探测 | \`ide_paths.go\`、\`paths.go\` |"
      ;;
    internal/selfupdate|internal/selfupdate/*)
      echo "| 修改自动更新策略 | \`auto.go\`、\`updater.go\` |"
      ;;
    examples|examples/*)
      echo "| 新增示例套装 | 创建子目录 + \`bundle.yaml\` 或 \`installer.yaml\` |"
      echo "| 试用安装 | \`work install ./examples/<name>\` |"
      ;;
    docs|docs/*)
      echo "| 更新设计文档 | 本目录 \`*.md\` |"
      ;;
    *)
      echo "| 修改本目录功能 | 查看下方「关键符号」定位具体文件 |"
      echo "| 理解调用关系 | 使用 CodeGraph MCP 的 \`codegraph_explore\` |"
      ;;
  esac
}

# 收集有意义目录（目录内符号总数 > 0）
mapfile -t MEANINGFUL_DIRS < <(
  echo "$FILES_JSON" | jq -r '
    [.[] | select(.nodeCount > 0) | .path | rtrimstr("/") | split("/") | .[0:-1] | join("/")]
    | map(select(length > 0))
    | unique
    | .[]
  ' 2>/dev/null || true
)

# 根目录：若根下有带符号文件也纳入
ROOT_SYMBOLS="$(echo "$FILES_JSON" | jq '[.[] | select(.nodeCount > 0 and (.path | contains("/") | not))] | length')"
DIRS_TO_WRITE=()
if [[ "$ROOT_SYMBOLS" -gt 0 ]]; then
  DIRS_TO_WRITE+=(".")
fi

for d in "${MEANINGFUL_DIRS[@]:-}"; do
  if should_skip_dir "$d"; then
    continue
  fi
  DIRS_TO_WRITE+=("$d")
done

# 去重排序
mapfile -t DIRS_TO_WRITE < <(printf '%s\n' "${DIRS_TO_WRITE[@]}" | sort -u)

if [[ ${#DIRS_TO_WRITE[@]} -eq 0 ]]; then
  echo "未找到含源码符号的目录。请先确认 codegraph 索引成功。"
  exit 0
fi

symbols_for_dir() {
  local dir="$1"
  local prefix="$dir"
  [[ "$prefix" == "." ]] && prefix=""
  if [[ -n "$prefix" ]]; then
    prefix="${prefix}/"
  fi

  {
    echo "$FUNCS_JSON" | jq -r --arg p "$prefix" '
      [.[] | .node | select(.filePath | startswith($p)) |
        select(.kind == "function" or .kind == "method") |
        {name, file: .filePath, line: .startLine, exported: .isExported, sig: .signature}
      ] | sort_by((if .exported then 0 else 1 end), .name) | .[:12][] |
      "- `\(.name)`\(if .exported then " (exported)" else "" end) — `\(.file):\(.line)`"
    ' 2>/dev/null
    echo "$STRUCTS_JSON" | jq -r --arg p "$prefix" '
      [.[] | .node | select(.filePath | startswith($p)) |
        {name, file: .filePath, line: .startLine, exported: .isExported}
      ] | sort_by((if .exported then 0 else 1 end), .name) | .[:8][] |
      "- `type \(.name)`\(if .exported then " (exported)" else "" end) — `\(.file):\(.line)`"
    ' 2>/dev/null
    echo "$CLASSES_JSON" | jq -r --arg p "$prefix" '
      [.[] | .node | select(.filePath | startswith($p)) |
        {name, file: .filePath, line: .startLine, exported: .isExported}
      ] | sort_by((if .exported then 0 else 1 end), .name) | .[:8][] |
      "- `class \(.name)` — `\(.file):\(.line)`"
    ' 2>/dev/null
  } | sed '/^$/d' | head -20
}

files_in_dir() {
  local dir="$1"
  echo "$FILES_JSON" | jq -r --arg d "$dir" '
    if $d == "." then
      [.[] | select(.nodeCount > 0 and (.path | contains("/") | not)) | "- `\(.path)` (\(.language), \(.nodeCount) symbols)"]
    else
      [.[] | select(.nodeCount > 0 and (.path | startswith($d + "/"))) | "- `\(.path)` (\(.language), \(.nodeCount) symbols)"]
    end | .[:15][] // empty
  '
}

related_dirs() {
  local dir="$1"
  if [[ "$dir" != "." ]]; then
    parent="$(dirname "$dir")"
    [[ "$parent" == "." || -z "$parent" ]] && parent="."
    echo "- 父目录: \`$parent/\`"
  fi
  echo "$FILES_JSON" | jq -r --arg d "$dir" '
    if $d == "." then
      [.[] | select(.nodeCount > 0 and (.path | contains("/"))) | .path | split("/")[0]] | unique | .[:8][]
    else
      [.[] | select(.nodeCount > 0 and (.path | startswith($d + "/"))) | .path |
        sub("^" + $d + "/"; "") | if contains("/") then split("/")[0] else empty end
      ] | unique | .[:8][]
    end // empty | "- 子目录: `\(.)/`"
  ' 2>/dev/null
}

write_agents_md() {
  local dir="$1"
  local out
  if [[ "$dir" == "." ]]; then
    out="AGENTS.md"
  else
    out="${dir}/AGENTS.md"
  fi

  local title_dir="$dir"
  [[ "$title_dir" == "." ]] && title_dir="(root)"

  {
    echo "# AGENTS.md — ${title_dir}"
    echo ""
    echo "> 由 CodeGraph 知识图谱自动生成。手动更新: \`generate-agents.sh\`；开启自动同步: \`setup-auto-sync.sh\`"
    echo ""
    echo "## 目录用途"
    echo ""
    dir_purpose "$dir"
    echo ""
    echo "## AI 操作指引"
    echo ""
    echo "| 任务 | 去哪里 |"
    echo "|------|--------|"
    task_hints "$dir"
    echo ""
    echo "## 本目录文件"
    echo ""
    local flist
    flist="$(files_in_dir "$dir")"
    if [[ -n "$flist" ]]; then
      echo "$flist"
    else
      echo "_（无独立文件列表）_"
    fi
    echo ""
    echo "## 关键符号"
    echo ""
    local syms
    syms="$(symbols_for_dir "$dir")"
    if [[ -n "$syms" ]]; then
      echo "$syms"
    else
      echo "_（未索引到函数/类型，可用 \`codegraph query\` 进一步搜索）_"
    fi
    echo ""
    echo "## 相关目录"
    echo ""
    related_dirs "$dir"
    echo ""
    echo "## CodeGraph 提示"
    echo ""
    echo "- 结构/流程问题 → MCP \`codegraph_explore\`"
    echo "- 修改前评估影响 → \`codegraph_impact <符号名>\`"
    echo "- 手动同步索引 → \`codegraph sync\`"
    echo ""
  } > /tmp/agents_md_content_$$

  if $DRY_RUN; then
    log "[dry-run] 将写入: $out"
  else
    mkdir -p "$(dirname "$out")"
    mv /tmp/agents_md_content_$$ "$out"
    log "已写入: $out"
  fi
}

log "将为 ${#DIRS_TO_WRITE[@]} 个目录生成 AGENTS.md ..."
for dir in "${DIRS_TO_WRITE[@]}"; do
  write_agents_md "$dir"
done

log "完成。"
