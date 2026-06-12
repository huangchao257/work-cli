# AGENTS.md — internal/catalog

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

源码目录

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 修改本目录功能 | 查看下方「关键符号」定位具体文件 |
| 理解调用关系 | 使用 CodeGraph MCP 的 `codegraph_explore` |

## 本目录文件

- `internal/catalog/builtin.go` (go, 12 symbols)
- `internal/catalog/builtin_test.go` (go, 5 symbols)

## 关键符号

- `Names` (exported) — `internal/catalog/builtin.go:50`
- `Resolve` (exported) — `internal/catalog/builtin.go:23`
- `TestResolveCodegraphStack` (exported) — `internal/catalog/builtin_test.go:9`
- `examplesRoot` — `internal/catalog/builtin.go:59`
- `fileExists` — `internal/catalog/builtin.go:112`
- `findExamplesUp` — `internal/catalog/builtin.go:93`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

