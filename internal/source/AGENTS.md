# AGENTS.md — internal/source

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

包来源：Registry / Git / 本地路径解析

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 新增包来源类型 | `resolver.go` + 新解析器文件 |

## 本目录文件

- `internal/source/git.go` (go, 7 symbols)
- `internal/source/local.go` (go, 6 symbols)
- `internal/source/ref.go` (go, 15 symbols)
- `internal/source/ref_test.go` (go, 5 symbols)
- `internal/source/registry.go` (go, 23 symbols)
- `internal/source/resolver.go` (go, 4 symbols)

## 关键符号

- `CacheDir` (exported) — `internal/source/registry.go:58`
- `LoadUserConfig` (exported) — `internal/source/registry.go:38`
- `ParseInstallName` (exported) — `internal/source/ref.go:33`
- `ParseRef` (exported) — `internal/source/ref.go:70`
- `Resolve` (exported) — `internal/source/resolver.go:9`
- `ResolveGit` (exported) — `internal/source/git.go:11`
- `ResolveLocal` (exported) — `internal/source/local.go:11`
- `ResolveRegistry` (exported) — `internal/source/registry.go:69`
- `TestParseInstallNameAcceptsRegistryName` (exported) — `internal/source/ref_test.go:19`
- `TestParseInstallNameRejectsInvalidName` (exported) — `internal/source/ref_test.go:29`
- `TestParseInstallNameRejectsLocalPath` (exported) — `internal/source/ref_test.go:5`
- `ValidateInstallName` (exported) — `internal/source/ref.go:54`
- `type Ref` (exported) — `internal/source/ref.go:20`
- `type RegistryConfig` (exported) — `internal/source/registry.go:19`
- `type UserConfig` (exported) — `internal/source/registry.go:23`
- `type registryResponse` — `internal/source/registry.go:30`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

