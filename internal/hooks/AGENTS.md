# AGENTS.md — internal/hooks

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

Hooks 模块：事件模型、脱敏与上报

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 修改事件模型或上报 | 本目录 `*.go` |

## 本目录文件

- `internal/hooks/config.go` (go, 15 symbols)
- `internal/hooks/events.go` (go, 21 symbols)
- `internal/hooks/events_test.go` (go, 5 symbols)
- `internal/hooks/manifest.go` (go, 6 symbols)
- `internal/hooks/merge.go` (go, 14 symbols)
- `internal/hooks/merge_test.go` (go, 6 symbols)
- `internal/hooks/parse.go` (go, 11 symbols)
- `internal/hooks/paths.go` (go, 10 symbols)
- `internal/hooks/queue.go` (go, 20 symbols)
- `internal/hooks/redact.go` (go, 8 symbols)
- `internal/hooks/report.go` (go, 16 symbols)
- `internal/hooks/scripts.go` (go, 9 symbols)
- `internal/hooks/sidecar.go` (go, 16 symbols)
- `internal/hooks/status.go` (go, 4 symbols)
- `internal/hooks/sync.go` (go, 14 symbols)

## 关键符号

- `AbstractForIDEReport` (exported) — `internal/hooks/events.go:179`
- `AppendQueue` (exported) — `internal/hooks/queue.go:58`
- `BindingsForIDE` (exported) — `internal/hooks/events.go:84`
- `CheckRequiredEnv` (exported) — `internal/hooks/parse.go:60`
- `CommandPathForIDE` (exported) — `internal/hooks/scripts.go:9`
- `CountPending` (exported) — `internal/hooks/queue.go:225`
- `EncodeReportDebug` (exported) — `internal/hooks/report.go:97`
- `GetStatus` (exported) — `internal/hooks/status.go:14`
- `HooksConfigPath` (exported) — `internal/hooks/paths.go:11`
- `HooksInstalledDir` (exported) — `internal/hooks/config.go:137`
- `HooksScriptDir` (exported) — `internal/hooks/paths.go:44`
- `IsWorkManagedCommand` (exported) — `internal/hooks/sidecar.go:84`
- `type Binding` (exported) — `internal/hooks/events.go:25`
- `type EnvVar` (exported) — `internal/hooks/manifest.go:14`
- `type EventRecord` (exported) — `internal/hooks/queue.go:12`
- `type HookResource` (exported) — `internal/hooks/manifest.go:30`
- `type HookResources` (exported) — `internal/hooks/manifest.go:26`
- `type Manifest` (exported) — `internal/hooks/manifest.go:3`
- `type QueueEntry` (exported) — `internal/hooks/queue.go:28`
- `type ReportInput` (exported) — `internal/hooks/report.go:15`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

