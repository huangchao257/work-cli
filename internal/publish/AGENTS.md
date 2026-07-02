# AGENTS.md — internal/publish

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

将 `work pack` 产出的归档上传至内部 Registry，含校验和验证与流式 multipart 上传

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 发布主流程 | `publish.go` — Run |
| 归档内 manifest 推断 | `publish.go` — InspectArchive |
| zip 归解析 | `publish.go` — inspectZip |
| tar.gz 归解析 | `publish.go` — inspectTarGz |
| 校验和验证 | `publish.go` — verifyChecksumFile |
| 流式上传 | `publish.go` — upload（使用 io.Pipe） |
| 校验和错误类型 | `publish.go` — checksumError |
| 用法错误 | 返回 `usage.Error`（由 `internal/usage` 统一定义） |

## 本目录文件

- `internal/publish/publish.go` (go, 19 symbols)
- `internal/publish/publish_test.go` (go, 10 symbols)

## 关键符号

- `Run` (exported) — `internal/publish/publish.go:66`
- `Options` (exported) — `internal/publish/publish.go:26`
- `Result` (exported) — `internal/publish/publish.go:34`
- `InspectArchive` (exported) — `internal/publish/publish.go:130`
- `IsUsageError` (exported) — `internal/publish/publish.go:43`

## 相关目录

- 父目录: `internal/`
- 依赖: `internal/pack`, `internal/pkg/manifest`, `internal/usage`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`
