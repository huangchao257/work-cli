# AGENTS.md — internal/graph

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

源码目录

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 修改本目录功能 | 查看下方「关键符号」定位具体文件 |
| 理解调用关系 | 使用 CodeGraph MCP 的 `codegraph_explore` |

## 本目录文件

- `internal/graph/runner.go` (go, 28 symbols)
- `internal/graph/runner_test.go` (go, 6 symbols)

## 关键符号

- `CollectStatus` (exported) — `internal/graph/runner.go:139`
- `Init` (exported) — `internal/graph/runner.go:32`
- `PrintStatus` (exported) — `internal/graph/runner.go:94`
- `RunPostInstall` (exported) — `internal/graph/runner.go:318`
- `Sync` (exported) — `internal/graph/runner.go:74`
- `TestSetupCursorHook` (exported) — `internal/graph/runner_test.go:10`
- `codegraphStatus` — `internal/graph/runner.go:215`
- `ensureCodegraph` — `internal/graph/runner.go:180`
- `findScript` — `internal/graph/runner.go:228`
- `hookConfigured` — `internal/graph/runner.go:249`
- `resolveRoot` — `internal/graph/runner.go:165`
- `runBash` — `internal/graph/runner.go:306`
- `type Options` (exported) — `internal/graph/runner.go:17`
- `type Status` (exported) — `internal/graph/runner.go:24`
- `type cursorHookEntry` — `internal/graph/runner.go:262`
- `type cursorHooksFile` — `internal/graph/runner.go:257`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

