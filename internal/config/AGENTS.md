# AGENTS.md — internal/config

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

`~/.work/config.yaml` 的统一读写入口，基于 yaml.Node 点分路径导航，保留注释与键顺序

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 读写配置 | `config.go` — Get/Set/Unset/List |
| 配置路径 | `config.go` — Path |
| YAML 节点操作 | `config.go` — findValue/setNode/unsetNode/flatten |
| 值类型推断 | `config.go` — buildValueNode/scalarNode |
| 用法错误 | 返回 `usage.Error`（CLI 层映射为退出码 2） |

## 本目录文件

- `internal/config/config.go` (go, 20 symbols)
- `internal/config/config_test.go` (go, 11 symbols)

## 关键符号

- `Path` (exported) — `internal/config/config.go:30`
- `Load` (exported) — `internal/config/config.go:40`
- `Save` (exported) — `internal/config/config.go:67`
- `Get` (exported) — `internal/config/config.go:87`
- `Set` (exported) — `internal/config/config.go:116`
- `Unset` (exported) — `internal/config/config.go:131`
- `List` (exported) — `internal/config/config.go:147`
- `UsageError` (exported, → `usage.Error`) — `internal/config/config.go:21`

## 相关目录

- 父目录: `internal/`
- 依赖: `internal/platform`, `internal/usage`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`
