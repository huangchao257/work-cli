# AGENTS.md — internal/state

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

安装状态：installed.json 持久化

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 修改已安装记录结构 | `types.go`、`store.go` |

## 本目录文件

- `internal/state/store.go` (go, 15 symbols)
- `internal/state/store_test.go` (go, 4 symbols)
- `internal/state/types.go` (go, 6 symbols)

## 关键符号

- `Open` (exported) — `internal/state/store.go:16`
- `TestStoreUpsertRemove` (exported) — `internal/state/store_test.go:8`
- `type BundleRecord` (exported) — `internal/state/types.go:9`
- `type BundleResources` (exported) — `internal/state/types.go:22`
- `type File` (exported) — `internal/state/types.go:5`
- `type Store` (exported) — `internal/state/store.go:12`
- `type TelemetryInfo` (exported) — `internal/state/types.go:29`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

