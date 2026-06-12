# AGENTS.md — internal/cli

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

CLI 命令层：用户可见子命令、全局参数与帮助

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 新增/修改子命令 | 在本目录添加或编辑 `*_cmd.go` / 命令文件 |
| 修改全局参数 | `root.go` |
| 修改中文帮助 | `help.go` |

## 本目录文件

- `internal/cli/autoupdate.go` (go, 12 symbols)
- `internal/cli/errors.go` (go, 7 symbols)
- `internal/cli/graph.go` (go, 11 symbols)
- `internal/cli/help.go` (go, 7 symbols)
- `internal/cli/hooks.go` (go, 15 symbols)
- `internal/cli/install.go` (go, 8 symbols)
- `internal/cli/list.go` (go, 6 symbols)
- `internal/cli/reexec.go` (go, 6 symbols)
- `internal/cli/reexec_unix.go` (go, 4 symbols)
- `internal/cli/reexec_windows.go` (go, 4 symbols)
- `internal/cli/root.go` (go, 12 symbols)
- `internal/cli/uninstall.go` (go, 7 symbols)
- `internal/cli/update.go` (go, 7 symbols)
- `internal/cli/upgrade.go` (go, 13 symbols)
- `internal/cli/version.go` (go, 10 symbols)

## 关键符号

- `Execute` (exported) — `internal/cli/root.go:37`
- `ExitCode` (exported) — `internal/cli/errors.go:18`
- `SplitIDEs` (exported) — `internal/cli/root.go:41`
- `exitErr` — `internal/cli/errors.go:26`
- `init` — `internal/cli/autoupdate.go:16`
- `init` — `internal/cli/list.go:24`
- `init` — `internal/cli/uninstall.go:27`
- `init` — `internal/cli/update.go:35`
- `init` — `internal/cli/upgrade.go:79`
- `init` — `internal/cli/version.go:42`
- `init` — `internal/cli/hooks.go:97`
- `init` — `internal/cli/install.go:37`
- `type exitError` — `internal/cli/errors.go:5`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

