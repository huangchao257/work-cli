# AGENTS.md — internal/scaffold

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

为套装作者生成符合 manifest 规范的骨架目录（bundle/cli/hooks 三种类型）

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 脚手架主流程 | `scaffold.go` — Run |
| 类型解析 | `scaffold.go` — ParseType |
| bundle 骨架模板 | `scaffold.go` — bundleSpecs |
| cli 骨架模板 | `scaffold.go` — cliSpecs |
| hooks 骨架模板 | `scaffold.go` — hooksSpecs |
| 文件写入（含可执行位） | `scaffold.go` — writeFile |
| 用法错误 | 返回 `usage.Error`（由 `internal/usage` 统一定义） |

## 本目录文件

- `internal/scaffold/scaffold.go` (go, 12 symbols)
- `internal/scaffold/scaffold_test.go` (go, 5 symbols)

## 关键符号

- `Run` (exported) — `internal/scaffold/scaffold.go:62`
- `Options` (exported) — `internal/scaffold/scaffold.go:47`
- `Type` (exported) — `internal/scaffold/scaffold.go:15`
- `ParseType` (exported) — `internal/scaffold/scaffold.go:37`
- `ErrUnknownType` (exported) — `internal/scaffold/scaffold.go:31`
- `TypeBundle`/`TypeCLI`/`TypeHooks` (exported) — `internal/scaffold/scaffold.go:18`
- `IsUsageError` (exported) — `internal/scaffold/scaffold.go:34`

## 相关目录

- 父目录: `internal/`
- 依赖: `internal/usage`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`
