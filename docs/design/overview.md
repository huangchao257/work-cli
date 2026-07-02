# 总览

> 跨模块的共性设计。模块细节见 `modules/` 下各文档。

## 1. 整体架构

```
用户 (work install / list / uninstall / update / hooks / graph / upgrade)
        │
        ▼
┌───────────────────────────────────────┐
│  CLI 层 (Cobra) — internal/cli/        │
│  子命令 + 全局参数 + 中文帮助          │
│  PersistentPreRunE: 自动更新检查+重执行 │
└───────────────────────────────────────┘        │ 自动更新
        │                                         ▼
        ▼                              internal/selfupdate/ (GitHub Releases)
┌───────────────────────────────────────┐
│  Engine 编排层 — internal/engine/      │
│  Install: source.Resolve →            │
│           pkg/manifest.DetectKind →    │
│           按 kind 分发                  │
└───────────────────────────────────────┘
        │
   ┌────┼────────────┬───────────────┐
   ▼    ▼            ▼               ▼
 KindBundle     KindCLI        KindHooks
 bundle.go      cli_install.go hooks.go
   │              │               │
   ▼              ▼               ▼
 Adapter        installer        hooks 包
 (写入 IDE)     (os/exec)       (merge+sidecar+queue)
        │
   source / state / platform / output
```

分层：`cmd/work → cli.Execute → engine → source/adapter/installer/hooks/state`，辅以 `platform`（跨平台路径）、`output`（human/json 渲染）、`selfupdate`（自更新）、`graph`（CodeGraph 封装）、`catalog`（内置包目录）。

## 2. 核心原则

1. **一份 manifest 描述一个安装包**，与来源无关；根目录文件名决定类型。
2. **统一状态文件** `installed.json` 记录三类安装（`kind` 字段区分），支撑 list/uninstall/update。
3. **跨平台路径** 统一经 `internal/platform` 解析，禁止硬编码 OS 路径。
4. **bundle/hooks**：三个 IDE Adapter 负责路径映射、文件写入、配置 merge。
5. **cli**：按 manifest 执行受信安装命令（按 OS 分平台），**不接受任意 shell**（防注入）。
6. **观察型 hook**：上报脚本透传 stdin/stdout、`exit 0`，不修改 IDE 行为。

## 3. 包类型与 Manifest 探测

每个可安装包根目录含一个 manifest，由 `internal/pkg/manifest.DetectKind` 按文件名探测：

| 文件 | kind | 解析包 | 模块文档 |
|------|------|--------|----------|
| `bundle.yaml` | `bundle` | `internal/bundle/` | [资源管理](./modules/resource.md) |
| `installer.yaml` | `cli` | `internal/installer/` | [资源管理](./modules/resource.md) |
| `hooks.yaml` | `hooks` | `internal/hooks/` | [Hooks](./modules/hooks.md) |

探测优先级：`installer.yaml` → `hooks.yaml` → `bundle.yaml`。

## 4. 命令与全局参数

| 命令 | 作用 |
|------|------|
| `work install <name>` | 安装内置或 Registry 资源（bundle/cli/hooks） |
| `work list [--kind] [--ide]` | 列出已安装项 |
| `work uninstall <name>` | 卸载 |
| `work update [name]` | 更新本机已安装资源 |
| `work upgrade [--check] [--version]` | 更新 work 自身（见 [自更新](./modules/selfupdate.md)） |
| `work version` | 显示版本（默认检查更新） |
| `work hooks status\|sync` | hooks 队列状态 / 手动上报（见 [Hooks](./modules/hooks.md)） |
| `work graph init\|sync\|status` | CodeGraph 图谱（见 [CodeGraph](./modules/codegraph.md)） |
| `work doctor` | 体检本机运行环境（见 [扩展能力](./extensions.md)） |
| `work init <type> <name>` | 生成套装骨架（见 [扩展能力](./extensions.md)） |
| `work config get\|set\|list\|path` | 读写 `~/.work/config.yaml`（见 [扩展能力](./extensions.md)） |
| `work pack <dir>` | 打包套装为可分发归档（见 [扩展能力](./extensions.md)） |
| `work publish <archive>` | 上传归档至 Registry（见 [扩展能力](./extensions.md)） |
| `work search [query]` | 列出可安装资源（见 [扩展能力](./extensions.md)） |
| `work hooks audit` | 本地 hooks 事件合规审计（见 [扩展能力](./extensions.md)） |
| `work help [command]` | 中文帮助 |

全局 persistent flag（`cli/root.go`）：

| 参数 | 默认 | 说明 |
|------|------|------|
| `--scope` | `user` | `user` 或 `project`（仅 bundle/hooks） |
| `--ide` | 全部已检测 | 逗号分隔：`qoder,cursor,claude` |
| `--kind` | 全部 | list 过滤：`bundle`/`cli`/`hooks` |
| `--dry-run` | `false` | 仅预览将执行的操作 |
| `--json` | `false` | JSON 输出（脚本/CI） |
| `--no-auto-update` | `false` | 跳过本次自动更新检查 |

## 5. 状态与配置

### 5.1 状态文件（`internal/state/`）

| scope | 路径 |
|-------|------|
| user | `~/.work/installed.json` |
| project | `<project-root>/.work/installed.json` |

记录结构（`state.BundleRecord`）：

```json
{
  "name": "dev-kit", "kind": "bundle", "version": "1.2.0",
  "scope": "user", "ref": "registry:dev-kit",
  "installed_at": "2026-06-11T10:00:00Z",
  "ides": ["cursor","claude"],
  "resources": { "skills": ["code-review"], "rules": ["go-style"], "mcp": ["internal-mysql"] },
  "install_command": "...",          // 仅 cli
  "telemetry": { "events": [...] }  // 仅 hooks
}
```

`kind` 取值 `bundle`/`cli`/`hooks`；`store.go::Open` 提供读写，是 list/uninstall/update 的共同依据。

### 5.2 用户配置 `~/.work/config.yaml`

```yaml
registry:
  url: https://registry.internal.example.com
cache:
  dir: ~/.work/cache
self_update:                       # 见自更新模块
  enabled: true
  check_interval: 2h
telemetry:                         # 见 hooks 模块
  enabled: true
  url: https://telemetry.internal.example.com/v1/events
  batch_size: 50
  sync_interval: 5m
  max_retries: 10
  events: [shell, mcp, file_read, file_edit, prompt]
  redact: [prompt, file_content, tool_input.content, env_secrets]
```

### 5.3 本地布局

```
~/.work/
├── config.yaml
├── installed.json
├── examples/                # 随包内置资源
├── cache/                   # Registry 下载缓存
├── telemetry/               # 见 hooks 模块
│   ├── queue.jsonl
│   ├── archive/
│   └── state.json
└── hooks-installed/         # 见 hooks 模块
    └── {name}.json
```

## 6. 跨平台与构建

目标平台：darwin(arm64/amd64)、linux(amd64/arm64)、windows(amd64)。

`internal/platform`：
- `paths.go` — `WorkConfigDir`/`WorkStatePath`/`ProjectRoot`/`UserHome`
- `ide_paths.go` — 各 IDE × scope 路径表（`SkillDir`/`RuleDir`/`MCPConfigPath` 等）
- `env_hint.go` — `EnvSetHint`，按 GOOS 生成环境变量设置提示

规则：用 `os.UserHomeDir()` + `filepath.Join`，禁硬编码 `/home`/`C:\Users`；JSON 文件 UTF-8/LF；目录 `os.MkdirAll(dir, 0755)`。

构建：`make build`（ldflags `-s -w -X ...internal/cli.Version=...` 注入版本号）；`make build-all` 交叉编译到 `dist/`。发布由 git tag 触发 GitHub Actions（`.github/workflows/release.yml`）→ GoReleaser（`.goreleaser.yaml`，前置 `go mod tidy` + `go test ./...`）。安装：macOS/Linux 用 `scripts/install.sh`，Windows 用 `scripts/install.ps1`。

## 7. 错误处理与输出

退出码（`cli/errors.go::ExitCode`）：

| 退出码 | 场景 |
|--------|------|
| 0 | 成功 |
| 1 | 一般错误（校验失败、IO 错误等） |
| 2 | 用法错误（参数不合法） |
| 3 | 环境不满足（缺 env、指定 IDE 未安装） |

输出（`internal/output/`）：默认 `human.go` 中文「问题 + 下一步」结构；`--json` 走 `json.go`（含 `success`/`warnings`/`files_written` 等字段，供脚本/CI）。

## 8. 测试策略

| 层级 | 内容 |
|------|------|
| 单元 | manifest 解析、路径解析（mock HOME）、MCP merge、hooks 事件映射/脱敏/queue 读写 |
| 集成 | `internal/engine/e2e_test.go` 跑 `examples/` 全流程 install→list→uninstall |
| CI 矩阵 | ubuntu/macos/windows（`.github/workflows/ci.yml`） |

## 9. 目录结构

```
work-cli/
├── cmd/work/main.go              # 入口 → cli.Execute()
├── internal/
│   ├── cli/                      # cobra 子命令 + 全局参数 + 中文帮助 + 自动更新/reexec
│   ├── engine/                   # install/list/uninstall/update 编排，按 kind 分发
│   ├── bundle/                   # bundle.yaml 解析与校验
│   ├── installer/                # installer.yaml 解析与 CLI 命令执行
│   ├── hooks/                    # hooks.yaml + 事件/脱敏/队列/上报/sidecar
│   ├── source/                   # 名称解析（拒绝本地/git）+ Registry
│   ├── catalog/                  # 内置名称 → examples/<dir>
│   ├── adapter/                  # Cursor/Qoder/Claude 适配器 + MCP merge
│   ├── state/                    # installed.json
│   ├── platform/                 # 跨平台路径 + IDE 探测 + env 提示
│   ├── selfupdate/               # GitHub Releases 检查/下载/替换/重执行
│   ├── graph/                    # codegraph CLI 封装
│   ├── doctor/                   # 体检诊断
│   ├── scaffold/                 # work init 脚手架
│   ├── config/                   # work config 读写 ~/.work/config.yaml
│   ├── pack/                     # work pack 打包归档
│   ├── publish/                  # work publish 上传 Registry
│   ├── search/                   # work search 可用资源发现
│   ├── audit/                    # work hooks audit 本地合规审计
│   ├── pkg/manifest/             # DetectKind
│   └── output/                   # human / json 渲染
├── examples/                     # 内置套装（dev-kit/openspec/company-hooks/codegraph-*），兼测试夹具
├── docs/
│   ├── design/                   # 总体设计（本目录）
│   ├── install-guide.md
│   └── superpowers/              # 历史规格与计划（变更记录）
├── scripts/                      # install.sh / install.ps1
├── .github/workflows/            # ci.yml / release.yml
├── Makefile / .goreleaser.yaml / go.mod
└── README.md
```

各 `internal/<pkg>/` 附 CodeGraph 自动生成的 `AGENTS.md`（文件、关键符号 `file:line`、任务→文件表），导航某包时先读它；勿手改，由 `codegraph sync` 重新生成。

## 10. 后续扩展

| 功能 | 说明 |
|------|------|
| Hooks 阶段二 | 执行审计：服务端/本地策略引擎对 shell/mcp/file_edit 合规审计与告警（旁路分析，不阻断 IDE） |
| Hooks 阶段三 | 触发执行自动化：阻断型 hooks、审批流、Webhook、`failClosed` 策略 |
| `work pack/publish/doctor/init` | 打包/发布/体检/模板 |
| 认证 | SSO/API Key，对接 Registry、Git、Telemetry（已预留 `Authenticator`/`TelemetryAuthenticator` 接口，默认空实现） |
| Vault 集成 | 自动拉取 MCP 密钥 / 上报带短期 token |
| 更多 IDE | VS Code、Windsurf 等 |
| Windows arm64 | 补充构建目标 |

非目标：用户认证与权限控制；Windows arm64；WSL 与 Windows 原生环境自动合并；自研多语言 AST 解析器（CodeGraph 封装上游 CLI）。
