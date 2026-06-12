# AGENTS.md — internal/output

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

终端输出：人类可读与 --json 格式

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 修改本目录功能 | 查看下方「关键符号」定位具体文件 |
| 理解调用关系 | 使用 CodeGraph MCP 的 `codegraph_explore` |

## 本目录文件

- `internal/output/human.go` (go, 12 symbols)
- `internal/output/json.go` (go, 9 symbols)

## 关键符号

- `PrintHooksStatusHuman` (exported) — `internal/output/human.go:69`
- `PrintHooksStatusJSON` (exported) — `internal/output/json.go:25`
- `PrintHuman` (exported) — `internal/output/human.go:13`
- `PrintHumanList` (exported) — `internal/output/human.go:44`
- `PrintInstallJSON` (exported) — `internal/output/json.go:17`
- `PrintJSON` (exported) — `internal/output/json.go:11`
- `PrintListJSON` (exported) — `internal/output/json.go:21`
- `formatAge` — `internal/output/human.go:90`
- `scopeLabel` — `internal/output/human.go:62`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

