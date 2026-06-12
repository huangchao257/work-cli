# AGENTS.md — internal/adapter

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

IDE 适配层：向 Cursor / Qoder / Claude 写入 Skills / Rules / MCP

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 支持新 IDE 或修改安装路径 | 新增 `*_adapter.go` 或编辑现有适配器 |
| MCP 配置合并 | `mcp_merge.go` |

## 本目录文件

- `internal/adapter/adapter.go` (go, 10 symbols)
- `internal/adapter/claude.go` (go, 15 symbols)
- `internal/adapter/common.go` (go, 14 symbols)
- `internal/adapter/cursor.go` (go, 16 symbols)
- `internal/adapter/mcp_merge.go` (go, 7 symbols)
- `internal/adapter/mcp_merge_test.go` (go, 4 symbols)
- `internal/adapter/qoder.go` (go, 15 symbols)

## 关键符号

- `All` (exported) — `internal/adapter/adapter.go:26`
- `ByName` (exported) — `internal/adapter/adapter.go:34`
- `ExtractMCPServer` (exported) — `internal/adapter/mcp_merge.go:41`
- `MergeMCPServers` (exported) — `internal/adapter/mcp_merge.go:12`
- `NewClaude` (exported) — `internal/adapter/claude.go:15`
- `NewCursor` (exported) — `internal/adapter/cursor.go:15`
- `NewQoder` (exported) — `internal/adapter/qoder.go:15`
- `RemoveMCPServer` (exported) — `internal/adapter/mcp_merge.go:26`
- `TestMergeMCPServers` (exported) — `internal/adapter/mcp_merge_test.go:8`
- `cursorRuleFrontMatter` — `internal/adapter/common.go:68`
- `dirExists` — `internal/adapter/cursor.go:88`
- `installMCPAt` — `internal/adapter/common.go:44`
- `type claudeAdapter` — `internal/adapter/claude.go:13`
- `type cursorAdapter` — `internal/adapter/cursor.go:13`
- `type mcpFile` — `internal/adapter/mcp_merge.go:8`
- `type qoderAdapter` — `internal/adapter/qoder.go:13`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

