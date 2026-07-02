# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 这是什么

`work` 是公司内部 Go CLI（module `github.com/huangchao257/work-cli`），作为统一入口安装 AI IDE 资源套装（Skills / MCP / Rules）、外部 CLI 工具与 hooks 套装；同时支持从 GitHub Releases 自更新，并维护 CodeGraph 知识图谱与各目录 `AGENTS.md`。界面文案、帮助与注释均为中文。

## 常用命令

```bash
make build            # 构建到 bin/work（通过 -ldflags 注入版本号）
go build -o bin/work ./cmd/work
make test             # go test ./...
go test ./internal/engine/...              # 测试单个包
go test -run TestE2EBundleInstallListUninstall ./internal/engine/  # 单个测试
make build-all        # 交叉编译所有平台到 dist/
make package          # build-all + 列出 dist/
make clean
```

发布由 git tag 驱动，推送到 GitHub Actions（`.github/workflows/release.yml` → GoReleaser `.goreleaser.yaml`）。形如 `v0.1.0` 的 tag 触发 release；GoReleaser 以 `go mod tidy` + `go test ./...` 作为构建前置钩子。版本号通过 ldflags 注入 `internal/cli.Version`。

没有独立 lint 配置——基线为 `go vet`/`gofmt`（`gofmt -l .` 检查格式）。

## 架构

入口：`cmd/work/main.go` → `cli.Execute()`。其余代码在 `internal/` 下，分层为 CLI → engine → source/adapter/installer/state。

**命令层 — `internal/cli/`**：cobra 子命令，一文件一命令（`install.go`、`list.go`、`uninstall.go`、`update.go`、`upgrade.go`、`hooks.go`、`graph.go`、`version.go`）。全局 persistent flag 在 `root.go`（`--scope`、`--ide`、`--kind`、`--dry-run`、`--json`、`--no-auto-update`）。中文帮助集中在 `help.go`。`errors.go` 定义 `ExitCode(err)` / `exitError`，用于受控退出码。

**自动更新 + 重执行 — `internal/cli/autoupdate.go`、`reexec*.go`**：`PersistentPreRunE`（`runAutoUpdate`）在每条命令前运行。它检查 GitHub Releases（由 `~/.work/config.yaml` 中 `self_update.check_interval` 节流，默认 2h）；若有新版本则下载、替换二进制并**重新执行同一 argv**（`reexec.go` + 平台文件）。对 `upgrade`/`version`/`help`/`--dry-run`/`--json`/`--no-auto-update` 跳过。修改命令流程时务必记住：一条命令可能透明地运行两次（更新前一次、更新后一次）。

**Engine — `internal/engine/`**：编排核心。`install.go::Install` 通过 `source.Resolve` 解析包目录，用 `pkg/manifest.DetectKind` 探测类型后分发：`KindCLI`→`cli_install.go`、`KindHooks`→`hooks.go`、`KindBundle`→`bundle.go`。另有 `list.go`、`uninstall.go`、`update.go`。`Options`/`Result`/`ListItem` 类型在 `options.go`/`result.go`。`e2e_test.go` 针对内置示例包跑完整的 install→list→uninstall 循环——改 engine 逻辑时务必跑这些测试。

**包类型与 manifest**（每种类型对应包根目录的一个 YAML 文件，按文件名探测）：
- `bundle.yaml` → `internal/bundle/`（Skills/Rules/MCP 资源，由 `bundle.ParseDir` 解析；`validate.go` 检查必需环境变量）。
- `installer.yaml` → `internal/installer/`（外部 CLI 安装/校验命令；`runner.go` 按平台执行 shell 命令）。
- `hooks.yaml` → `internal/hooks/`（IDE hooks 定义 + telemetry 配置）。
探测逻辑：`internal/pkg/manifest/DetectKind` 依据 `installer.yaml` / `hooks.yaml` / `bundle.yaml` 是否存在判断。

**来源解析 — `internal/source/`**：`resolver.go::Resolve` 按 `Ref` 类型选择解析器。`ref.go` 解析/校验安装 `<name>`（仅支持 registry 风格名称——**本地路径与 git 引用被拒绝**；`ValidateInstallName`）。`registry.go` 从 `~/.work/config.yaml` 读 `registry.url`；`git.go`/`local.go` 存在但公开安装路径仅支持 name→catalog/registry。

**内置 catalog — `internal/catalog/builtin.go`**：硬编码 map，registry 名称 → `examples/<dir>`。`examplesRoot()` 依次查找 `WORK_EXAMPLES_DIR` 环境变量、`~/.work/examples`、相对二进制的路径，再从 cwd 向上找（开发/测试用）。内置项：`dev-kit`、`codegraph-stack`、`codegraph-kit`、`codegraph`、`company-hooks`、`openspec`、`openspec-mock`。`examples/` 下的示例包既是测试夹具也是随包发布的资源。

**IDE 适配层 — `internal/adapter/`**：向 Cursor（`cursor.go`）、Qoder（`qoder.go`）、Claude Code（`claude.go`）写入 Skills/Rules/MCP。`adapter.go` 注册 `All`/`ByName`。`mcp_merge.go` 把 MCP server 配置合并进既有 IDE MCP 文件。`common.go` 含共享路径/front-matter 工具。新增 IDE：新增适配器文件并注册。

**状态 — `internal/state/`**：`installed.json`（位于 `platform.WorkStatePath`）记录已安装项。`store.go::Open`，类型在 `types.go`。`update`/`uninstall`/`list` 读此记录。

**平台层 — `internal/platform/`**：`paths.go`（`WorkConfigDir`、`WorkStatePath`、`ProjectRoot`）、`ide_paths.go`（各 IDE 的 Skill/Rule/MCP 路径）、`env_hint.go`（缺必需环境变量时给提示）。

**Hooks — `internal/hooks/`**：解析 `hooks.yaml`、合并进 IDE hooks 配置、安装 sidecar 脚本，并采集/上报 telemetry 事件到本地队列（`~/.work/telemetry/queue.jsonl`），再异步同步到 `telemetry.url`。阶段一为纯观察（不阻断）。`status.go`/`report.go`/`queue.go`/`redact.go`（PII 脱敏）。

**自更新 — `internal/selfupdate/`**：`github.go` 查询 Releases，`updater.go` 下载/替换二进制，`auto.go` 决定是否更新（`ShouldAutoUpdate`，节流），`state.go` 持久化上次检查时间，`version.go` 比较 semver。`config.go` 从 `~/.work/config.yaml` 读配置。

**Graph — `internal/graph/`**：对外部 `codegraph` CLI 的薄封装；`runner.go` 跑 `init`/`sync`/`status` 并接好自动同步 hook，重新生成各目录 `AGENTS.md`。

**输出 — `internal/output/`**：`human.go`（默认）与 `json.go`（`--json`）渲染器。

## 约定

- 用户可见字符串与代码注释用**中文**——新增帮助文案、错误信息或注释时保持一致。
- 每个 `internal/<pkg>/` 都带一份 CodeGraph 自动生成的 `AGENTS.md`，列出文件、关键符号（带 `file:line`）以及「AI 操作指引」表（任务 → 文件）。导航某包时**先读该包的 `AGENTS.md`**，是最快的定位方式。这些文件是自动重新生成的——不要手工编辑。
- CodeGraph 索引位于 `.codegraph/`（已 gitignore）。编辑前用 `codegraph_explore` / `codegraph_callers` / `codegraph_impact` 等 MCP 工具查看调用路径、评估重构影响。
- 配置根目录 `~/.work/`：`config.yaml`（registry/cache/self_update/telemetry）、`installed.json`（状态）、`examples/`（随包内置）、`telemetry/queue.jsonl`。
- 凡是改动文件系统的命令都必须遵守 `--dry-run`；engine 与 output 层都据此分支。

## 设计文档

- 总体设计：`docs/superpowers/specs/2026-06-11-work-cli-design.md`
- CodeGraph 套装：`docs/superpowers/specs/2026-06-11-codegraph-agents-design.md`
- Hooks 模块：`docs/superpowers/specs/2026-06-11-hooks-module-design.md`
- 实现计划位于 `docs/superpowers/plans/`。
