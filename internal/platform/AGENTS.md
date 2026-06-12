# AGENTS.md — internal/platform

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

跨平台路径、IDE 探测与环境提示

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 修改 IDE 路径探测 | `ide_paths.go`、`paths.go` |

## 本目录文件

- `internal/platform/env_hint.go` (go, 3 symbols)
- `internal/platform/ide_paths.go` (go, 15 symbols)
- `internal/platform/ide_paths_test.go` (go, 4 symbols)
- `internal/platform/paths.go` (go, 7 symbols)
- `internal/platform/paths_test.go` (go, 4 symbols)

## 关键符号

- `EnvSetHint` (exported) — `internal/platform/env_hint.go:5`
- `MCPConfigPath` (exported) — `internal/platform/ide_paths.go:49`
- `ProjectRoot` (exported) — `internal/platform/paths.go:35`
- `RuleDir` (exported) — `internal/platform/ide_paths.go:36`
- `RuleFile` (exported) — `internal/platform/ide_paths.go:24`
- `SkillDir` (exported) — `internal/platform/ide_paths.go:16`
- `TestCursorUserSkillDir` (exported) — `internal/platform/ide_paths_test.go:8`
- `TestWorkConfigDir` (exported) — `internal/platform/paths_test.go:8`
- `UserHome` (exported) — `internal/platform/paths.go:8`
- `WorkConfigDir` (exported) — `internal/platform/paths.go:12`
- `WorkStatePath` (exported) — `internal/platform/paths.go:20`
- `errUnknownIDE` — `internal/platform/ide_paths.go:131`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

