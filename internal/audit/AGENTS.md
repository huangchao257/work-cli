# AGENTS.md — internal/audit

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

本地 hooks 事件审计引擎：加载策略、读取事件队列、按规则评估违规，纯离线旁路分析

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 策略加载与校验 | `audit.go` — LoadPolicy |
| 策略预编译 | `audit.go` — Policy.Compile（缓存正则） |
| 事件读取（含时间过滤） | `audit.go` — ReadEvents |
| 违规评估 | `audit.go` — Evaluate（纯函数，时间过滤由 ReadEvents 负责） |
| 规则匹配逻辑 | `audit.go` — compiledRule.match |
| Payload 文本化 | `audit.go` — payloadText |
| 路径字段提取 | `audit.go` — extractPaths |
| 违规/策略/规则类型 | `audit.go` — Violation/Policy/Rule/Severity |

## 本目录文件

- `internal/audit/audit.go` (go, 14 symbols)
- `internal/audit/audit_test.go` (go, 12 symbols)

## 关键符号

- `LoadPolicy` (exported) — `internal/audit/audit.go:80`
- `Compile` (exported) — `internal/audit/audit.go:51`
- `CompiledPolicy` (exported) — `internal/audit/audit.go:47`
- `ReadEvents` (exported) — `internal/audit/audit.go:118`
- `Evaluate` (exported) — `internal/audit/audit.go:150`
- `Policy` (exported) — `internal/audit/audit.go:38`
- `Rule` (exported) — `internal/audit/audit.go:29`
- `Violation` (exported) — `internal/audit/audit.go:58`
- `Severity`/`Low`/`Medium`/`High` (exported) — `internal/audit/audit.go:21`
- `EventRecord` (exported, = hook.EventRecord) — `internal/audit/audit.go:69`

## 相关目录

- 父目录: `internal/`
- 依赖: `internal/hooks`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`
