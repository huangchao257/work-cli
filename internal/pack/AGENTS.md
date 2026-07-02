# AGENTS.md — internal/pack

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

将本地套装目录打包为可分发归档（tar.gz/zip），生成 sha256 校验和

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 打包主流程 | `pack.go` — Run |
| 归档格式解析 | `pack.go` — ParseFormat |
| Manifest 探测 | 调用 `internal/pkg/manifest` |
| 输出路径解析 | `pack.go` — resolveOutputPath |
| 文件收集 | `pack.go` — collectFiles |
| tar.gz 写入 | `pack.go` — writeTarGz |
| zip 写入 | `pack.go` — writeZip |
| 用法错误 | 返回 `usage.Error`（由 `internal/usage` 统一定义） |

## 本目录文件

- `internal/pack/pack.go` (go, 16 symbols)
- `internal/pack/pack_test.go` (go, 7 symbols)

## 关键符号

- `Run` (exported) — `internal/pack/pack.go:75`
- `Options` (exported) — `internal/pack/pack.go:42`
- `Result` (exported) — `internal/pack/pack.go:50`
- `Format` (exported) — `internal/pack/pack.go:22`
- `ParseFormat` (exported) — `internal/pack/pack.go:30`
- `FormatTarGz`/`FormatZip` (exported) — `internal/pack/pack.go:25`
- `IsUsageError` (exported) — `internal/pack/pack.go:60`

## 相关目录

- 父目录: `internal/`
- 依赖: `internal/pkg/manifest`, `internal/usage`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`
