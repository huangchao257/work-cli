# AGENTS.md — internal/doctor

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

`work doctor` 诊断命令核心逻辑：逐个检查 IDE/PATH/config/MCP/codegraph/jq/自更新，返回结构化结果

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 新增检查项 | `doctor.go` — 添加 check* 函数并在 Run 中注册 |
| 修改检查输出格式 | `doctor.go` — CheckResult/HasError/Summary |
| YAML/JSON 解析校验 | `doctor.go` — ParseConfigYAML/ParseMCPJSON（纯函数，可单独测试） |
| IDE 适配器探测 | 调用 `internal/adapter` 包 |
| 平台路径 | 调用 `internal/platform` 包 |

## 本目录文件

- `internal/doctor/doctor.go` (go, 18 symbols)
- `internal/doctor/doctor_test.go` (go, 9 symbols)

## 关键符号

- `Run` (exported) — `internal/doctor/doctor.go:47`
- `Options` (exported) — `internal/doctor/doctor.go:34`
- `CheckResult` (exported) — `internal/doctor/doctor.go:26`
- `HasError` (exported) — `internal/doctor/doctor.go:67`
- `Summary` (exported) — `internal/doctor/doctor.go:77`
- `ParseConfigYAML` (exported) — `internal/doctor/doctor.go:303`
- `ParseMCPJSON` (exported) — `internal/doctor/doctor.go:310`
- `SeverityError`/`SeverityInfo` (exported) — `internal/doctor/doctor.go:41`

## 相关目录

- 父目录: `internal/`
- 依赖: `internal/adapter`, `internal/platform`, `internal/selfupdate`, `internal/state`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`
