# AGENTS.md — internal/search

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

`work search` 命令核心：列出可安装资源（内置 catalog + 可选 Registry 远程清单），模糊搜索

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 搜索主流程 | `search.go` — Run |
| 内置 catalog 加载 | `search.go` — loadBuiltin |
| Manifest 解析 | `search.go` — parseManifestMeta |
| Registry 远程查询 | `search.go` — fetchRegistry |
| Item 类型定义 | `search.go` — Item/Options/Result |
| 警告降级 | 本地/远程错误不返回 error，仅产出 warning |

## 本目录文件

- `internal/search/search.go` (go, 8 symbols)
- `internal/search/search_test.go` (go, 7 symbols)

## 关键符号

- `Run` (exported) — `internal/search/search.go:54`
- `Item` (exported) — `internal/search/search.go:23`
- `Options` (exported) — `internal/search/search.go:32`
- `Result` (exported) — `internal/search/search.go:39`

## 相关目录

- 父目录: `internal/`
- 依赖: `internal/catalog`, `internal/pkg/manifest`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`
