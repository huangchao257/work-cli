# work-cli

公司统一 CLI 入口，提供 AI IDE 资源管理、Hooks 事件上报、CodeGraph 知识图谱与自更新能力。

## 安装（员工）

**macOS / Linux：**

```bash
curl -fsSL https://github.com/huangchao257/work-cli/releases/latest/download/install.sh | bash
```

**Windows（PowerShell）：**

```powershell
irm https://github.com/huangchao257/work-cli/releases/latest/download/install.ps1 | iex
```

安装后执行 `work version` 验证。详细说明见 [员工安装指南](docs/install-guide.md)。

如需手动下载，从 [Releases](https://github.com/huangchao257/work-cli/releases) 获取对应平台压缩包，解压后将 `work` 加入 PATH。

## 快速开始

```bash
# 安装内置资源套装
work install dev-kit

# 安装 hooks 上报套装
work install company-hooks

# 预览 OpenSpec 安装（不实际执行）
work install openspec --dry-run

# 安装 OpenSpec
work install openspec

# 更新已安装资源
work update dev-kit
work update
```

## 命令参考

### 资源管理

| 命令 | 说明 |
|------|------|
| `work install <name>` | 安装内置或 Registry 资源 |
| `work list` | 列出已安装项（`--kind bundle/cli/hooks` 过滤） |
| `work uninstall <name>` | 卸载 |
| `work update [name]` | 更新本机已安装资源 |

### Hooks

| 命令 | 说明 |
|------|------|
| `work hooks status` | 查看事件上报队列状态 |
| `work hooks sync` | 将本地队列同步至内网 Telemetry |
| `work hooks audit` | 按策略对本地 hooks 事件做合规审计 |

### CodeGraph 知识图谱

| 命令 | 说明 |
|------|------|
| `work graph init` | 初始化图谱并开启自动同步 |
| `work graph sync` | 手动同步索引与 AGENTS.md |
| `work graph status` | 查看图谱状态 |

一键安装：`work install codegraph-stack`

### 自更新

| 命令 | 说明 |
|------|------|
| `work upgrade` | 更新 work 自身 |
| `work upgrade --check` | 仅检查是否有新版本 |
| `work upgrade --version v0.2.0` | 更新到指定版本 |
| `work version` | 显示版本（默认检查更新） |

自动更新默认开启——执行命令时隔 2h 检查 GitHub Releases；新版本自动下载替换后重执行。跳过本次检查：`--no-auto-update`。

### 套装开发与发布

| 命令 | 说明 |
|------|------|
| `work init <type> <name>` | 生成套装骨架（type: `bundle`/`cli`/`hooks`） |
| `work pack <dir>` | 打包套装为可分发归档 + 校验和 |
| `work publish <archive>` | 上传归档至内部 Registry |
| `work search [query]` | 搜索可安装资源（`--remote` 查询 Registry） |

### 配置与诊断

| 命令 | 说明 |
|------|------|
| `work config get/set/list/unset/path` | 读写 `~/.work/config.yaml` |
| `work doctor` | 体检本机运行环境（IDE/PATH/config/MCP/codegraph） |

### 全局参数

| 参数 | 默认 | 说明 |
|------|------|------|
| `--scope` | `user` | `user` 或 `project`（仅 bundle） |
| `--ide` | 全部已检测 | 逗号分隔：`qoder,cursor,claude` |
| `--dry-run` | false | 仅预览操作 |
| `--json` | false | JSON 输出 |
| `--no-auto-update` | false | 跳过本次自动更新检查 |

### 安装资源名称

仅支持内置名称或 Registry 名称，**不支持**本地路径或 git 引用。

| 来源 | 示例 |
|------|------|
| 内置资源 | `dev-kit`、`codegraph-stack`、`company-hooks`、`openspec` |
| 内部 Registry | 需在 `~/.work/config.yaml` 配置 `registry.url` |

## 配置

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

也可用环境变量 `WORK_AUTO_UPDATE=false` 关闭自动更新。

## Hooks 模块

采集 IDE hooks 事件，本地脱敏后写入 `~/.work/telemetry/queue.jsonl`，异步同步至内网 Telemetry。

```bash
work install company-hooks   # 安装 hooks 套装
work hooks status            # 查看待上报事件数
work hooks sync              # 手动同步
work hooks audit             # 本地合规审计
```

阶段一（当前）：观察型上报，不阻断 IDE。阶段二：审计告警。阶段三：触发执行自动化（阻断、审批、Webhook）。

## CodeGraph 知识图谱

一键安装 CodeGraph CLI + IDE MCP 配置 + 自动索引同步：

```bash
work install codegraph-stack
```

保存源码后约 2 秒内自动同步图谱并更新各目录 `AGENTS.md`。图谱数据在 `.codegraph/`（已 gitignore）。简单链路（单文件且符号少于 12 个）不生成 AGENTS.md。

也可单独操作：`work graph init|sync|status`。

## 故障排查

| 问题 | 处理 |
|------|------|
| 未检测到 IDE | 先安装 Cursor/Qoder/Claude Code，或用 `--ide cursor` 指定 |
| 缺少环境变量 | 按提示执行 `export VAR=值` |
| Registry 失败 | 检查 `~/.work/config.yaml` 中 `registry.url` |
| npm 安装失败 | 确认已安装 Node.js 和 npm |
| work 更新失败 | 确认网络可访问 GitHub；Windows 下关闭占用 work 的终端后重试 |

## 开发

```bash
make build          # 构建到 bin/work
make test           # go test ./...
make build-all      # 交叉编译所有平台到 dist/
```

发布由 git tag 驱动：`git tag v0.1.0 && git push origin v0.1.0` → GitHub Actions 自动 Release。

## 文档

- 设计文档（按模块）：`docs/design/`（[总览](docs/design/overview.md) + [资源管理](docs/design/modules/resource.md) + [Hooks](docs/design/modules/hooks.md) + [CodeGraph](docs/design/modules/codegraph.md) + [自更新](docs/design/modules/selfupdate.md) + [扩展能力](docs/design/extensions.md)）
- 设计规格：`docs/superpowers/specs/`
- 实现计划：`docs/superpowers/plans/`
