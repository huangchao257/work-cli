# AGENTS.md — internal/bundle

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

资源套装解析：bundle.yaml 读取与校验

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 扩展 bundle.yaml 字段 | `manifest.go`、`parse.go` |

## 本目录文件

- `internal/bundle/manifest.go` (go, 8 symbols)
- `internal/bundle/parse.go` (go, 8 symbols)
- `internal/bundle/parse_test.go` (go, 5 symbols)
- `internal/bundle/validate.go` (go, 7 symbols)

## 关键符号

- `CheckRequiredEnv` (exported) — `internal/bundle/validate.go:27`
- `CheckRequiredEnvVars` (exported) — `internal/bundle/validate.go:40`
- `ParseDir` (exported) — `internal/bundle/parse.go:13`
- `ParseFile` (exported) — `internal/bundle/parse.go:17`
- `TestParseDir` (exported) — `internal/bundle/parse_test.go:9`
- `Validate` (exported) — `internal/bundle/validate.go:9`
- `type EnvVar` (exported) — `internal/bundle/manifest.go:20`
- `type MCPResource` (exported) — `internal/bundle/manifest.go:44`
- `type Manifest` (exported) — `internal/bundle/manifest.go:3`
- `type PostInstall` (exported) — `internal/bundle/manifest.go:15`
- `type Resources` (exported) — `internal/bundle/manifest.go:26`
- `type RuleResource` (exported) — `internal/bundle/manifest.go:37`
- `type SkillResource` (exported) — `internal/bundle/manifest.go:32`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

