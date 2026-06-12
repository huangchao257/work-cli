# AGENTS.md — internal/installer

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

外部 CLI：installer.yaml 解析与命令执行

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 修改本目录功能 | 查看下方「关键符号」定位具体文件 |
| 理解调用关系 | 使用 CodeGraph MCP 的 `codegraph_explore` |

## 本目录文件

- `internal/installer/manifest.go` (go, 6 symbols)
- `internal/installer/parse.go` (go, 10 symbols)
- `internal/installer/runner.go` (go, 11 symbols)

## 关键符号

- `ParseDir` (exported) — `internal/installer/parse.go:14`
- `ParseFile` (exported) — `internal/installer/parse.go:18`
- `ResolveCommand` (exported) — `internal/installer/runner.go:12`
- `Run` (exported) — `internal/installer/runner.go:22`
- `RunCommand` (exported) — `internal/installer/runner.go:35`
- `Validate` (exported) — `internal/installer/parse.go:33`
- `defaultShell` — `internal/installer/runner.go:49`
- `type CommandSpec` (exported) — `internal/installer/manifest.go:17`
- `type Manifest` (exported) — `internal/installer/manifest.go:5`
- `type PlatformCommand` (exported) — `internal/installer/manifest.go:22`
- `type VerifySpec` (exported) — `internal/installer/manifest.go:26`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

