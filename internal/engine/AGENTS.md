# AGENTS.md — internal/engine

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

业务编排层：install / list / uninstall / update 核心流程

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 修改安装/卸载/更新逻辑 | 本目录对应 `*.go` |
| 新增安装类型 | `install.go` 分发 + 新 `*_install.go` |

## 本目录文件

- `internal/engine/bundle.go` (go, 14 symbols)
- `internal/engine/cli_install.go` (go, 11 symbols)
- `internal/engine/e2e_test.go` (go, 9 symbols)
- `internal/engine/hooks.go` (go, 16 symbols)
- `internal/engine/install.go` (go, 5 symbols)
- `internal/engine/list.go` (go, 4 symbols)
- `internal/engine/options.go` (go, 3 symbols)
- `internal/engine/result.go` (go, 4 symbols)
- `internal/engine/uninstall.go` (go, 10 symbols)
- `internal/engine/update.go` (go, 9 symbols)

## 关键符号

- `Install` (exported) — `internal/engine/install.go:10`
- `List` (exported) — `internal/engine/list.go:8`
- `TestE2EBundleInstallListUninstall` (exported) — `internal/engine/e2e_test.go:12`
- `TestE2ECLIMockInstall` (exported) — `internal/engine/e2e_test.go:55`
- `TestE2EOpenSpecDryRun` (exported) — `internal/engine/e2e_test.go:89`
- `Uninstall` (exported) — `internal/engine/uninstall.go:14`
- `Update` (exported) — `internal/engine/update.go:12`
- `findRecord` — `internal/engine/uninstall.go:88`
- `findRecordError` — `internal/engine/update.go:99`
- `hookIDs` — `internal/engine/hooks.go:258`
- `ids` — `internal/engine/bundle.go:163`
- `installBundle` — `internal/engine/bundle.go:15`
- `type ListItem` (exported) — `internal/engine/result.go:22`
- `type ListResult` (exported) — `internal/engine/result.go:18`
- `type Options` (exported) — `internal/engine/options.go:5`
- `type Result` (exported) — `internal/engine/result.go:3`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

