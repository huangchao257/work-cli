# work-cli

公司统一 CLI 入口。当前提供：

- **资源管理模块** — 安装 AI IDE 资源套装（Skills / MCP / Rules），支持 Qoder、Cursor、Claude Code
- **Hooks 模块** — 安装独立 `hooks.yaml` 套装，采集 IDE hooks 事件（本地队列 + 异步上报内网）
- 委托安装外部 CLI（如 OpenSpec）

## 安装 work（员工）

### 一键安装

**macOS / Linux：**

```bash
curl -fsSL https://github.com/huangchao257/work-cli/releases/latest/download/install.sh | bash
```

**Windows（PowerShell）：**

```powershell
irm https://github.com/huangchao257/work-cli/releases/latest/download/install.ps1 | iex
```

安装后执行 `work version` 验证。详细说明见 [员工安装指南](docs/install-guide.md)。

### 手动下载

从 [Releases](https://github.com/huangchao257/work-cli/releases) 下载对应平台压缩包，解压后将 `work` 加入 PATH。

### 本地构建（开发）

```bash
make build
# 或
go build -o bin/work ./cmd/work
```

### 发布新版本（IT）

```bash
git tag v0.1.0 && git push origin v0.1.0   # 自动触发 GitHub Release
# 本地打包: make package  → 产物在 dist/
```

## 快速开始

```bash
# 安装内置资源套装（写入 ~/.cursor）
work install dev-kit

# 安装公司 hooks 上报套装（预览）
work install company-hooks --dry-run

# 预览 OpenSpec 安装命令（不实际执行）
work install openspec --dry-run

# 实际安装 OpenSpec（需要 npm）
work install openspec

# 更新本机已安装的资源
work update dev-kit
work update
```

## 常用命令

| 命令 | 说明 |
|------|------|
| `work help [command]` | 查看命令帮助（中文说明与示例） |
| `work install <name>` | 安装已配置的资源（内置或 Registry） |
| `work list` | 列出已安装项 |
| `work list --kind cli` | 仅列出 CLI |
| `work uninstall <name>` | 卸载 |
| `work update [name]` | 更新本机已安装资源（读取 installed.json） |
| `work upgrade` | 更新 work 自身到最新版 |
| `work upgrade --check` | 仅检查 work 是否有新版本 |
| `work version` | 显示版本（默认检查更新） |
| `work hooks status` | 查看 hooks 事件上报队列状态 |
| `work hooks sync` | 将本地队列同步到内网 Telemetry |

### 全局参数

| 参数 | 默认 | 说明 |
|------|------|------|
| `--scope` | `user` | `user` 或 `project`（仅 bundle） |
| `--ide` | 全部已检测 | `qoder,cursor,claude` |
| `--dry-run` | false | 预览操作 |
| `--json` | false | JSON 输出 |

### 安装资源名称 `<name>`

仅支持公司内部已配置的资源名称，**不支持**本地路径或 git 引用。

| 来源 | 示例 |
|------|------|
| 内置资源 | `dev-kit`、`codegraph-stack`、`company-hooks`、`openspec` |
| 内部 Registry | 需在 `~/.work/config.yaml` 配置 `registry.url` |

内置资源列表随 `work` 发行包附带；安装时使用名称即可，例如 `work install dev-kit`。

## Registry 配置

`~/.work/config.yaml`：

```yaml
registry:
  url: https://registry.internal.example.com

cache:
  dir: ~/.work/cache

self_update:
  enabled: true
  check_interval: 2h

telemetry:
  enabled: true
  url: https://telemetry.internal.example.com/v1/events
  events: [shell, mcp, file_read, file_edit, prompt]
```

## OpenSpec

```bash
work install openspec
# 执行: npm install -g @fission-ai/openspec@latest
```

## CodeGraph 知识图谱 + AGENTS.md

对标 [CodeGraph](https://github.com/colbymchenry/codegraph) 体验：**一条命令安装，保存代码后无感自动更新**。

```bash
# 一键安装（CodeGraph CLI + IDE MCP + 索引 + AGENTS 自动同步）
work install codegraph-stack
```

安装完成后无需其他操作；在 Cursor 中保存源码，约 2 秒内自动同步图谱并更新各目录 `AGENTS.md`。

### 简易命令

| 命令 | 说明 |
|------|------|
| `work graph init` | 初始化图谱并开启自动同步（等同 `codegraph init -i` + 配置 hooks） |
| `work graph sync` | 手动同步索引与 AGENTS.md |
| `work graph status` | 查看状态 |

**说明：**

- 图谱数据在 `.codegraph/`（已 gitignore）
- `AGENTS.md` 写入各源码目录，告诉 AI「改什么去哪个文件」
- 安装 `codegraph-kit`（`--scope project`）时会自动执行 `work graph init`
- 需要 `jq`（生成 AGENTS.md 时）

设计文档：`docs/superpowers/specs/2026-06-11-codegraph-agents-design.md`

## 自动更新 work 自身

**默认开启**：执行 `work install`、`work list` 等命令时，会每 2 小时自动检查 GitHub Releases；若有新版本会自动下载并静默更新，然后重新执行你的命令。

```bash
# 手动检查是否有新版本
work upgrade --check

# 手动更新到最新版
work upgrade

# 本次命令跳过自动更新
work install dev-kit --no-auto-update

# 预览将下载的版本
work upgrade --dry-run

# 更新到指定版本
work upgrade --version v0.2.0
```

在 `~/.work/config.yaml` 中可配置：

```yaml
self_update:
  enabled: true          # 是否自动更新，默认 true
  check_interval: 2h     # 检查间隔，默认 2h
```

也可用环境变量 `WORK_AUTO_UPDATE=false` 关闭自动更新。

`work version` 默认会检查更新；若有新版本会提示运行 `work upgrade`。

## 故障排查

| 问题 | 处理 |
|------|------|
| 未检测到 IDE | 先安装 Cursor/Qoder/Claude Code，或用 `--ide cursor` 指定 |
| 缺少环境变量 | 按提示执行 `export VAR=值` |
| Registry 失败 | 检查 `~/.work/config.yaml` 中 `registry.url` |
| npm 安装失败 | 确认已安装 Node.js 和 npm |
| work 更新失败 | 确认网络可访问 GitHub；Windows 下关闭占用 work 的终端后重试 |

## Hooks 模块

```bash
# 安装 hooks 套装（写入各 IDE hooks 配置 + 上报脚本）
work install company-hooks

# 查看待上报事件数量
work hooks status

# 手动同步到内网（安装后也会异步自动同步）
work hooks sync
```

hooks 事件先写入 `~/.work/telemetry/queue.jsonl`，再 POST 到 `config.yaml` 中的 `telemetry.url`。内网不可达时仅保留本地，不影响 IDE 使用。

## 文档

- 设计规格：`docs/superpowers/specs/2026-06-11-work-cli-design.md`
- CodeGraph 套装：`docs/superpowers/specs/2026-06-11-codegraph-agents-design.md`
- Hooks 规格：`docs/superpowers/specs/2026-06-11-hooks-module-design.md`
- 实现计划：`docs/superpowers/plans/2026-06-11-resource-module.md`
- Hooks 计划：`docs/superpowers/plans/2026-06-11-hooks-module.md`
