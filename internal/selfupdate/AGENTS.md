# AGENTS.md — internal/selfupdate

> 由 CodeGraph 知识图谱自动生成。手动更新: `generate-agents.sh`；开启自动同步: `setup-auto-sync.sh`

## 目录用途

work 自身版本检查与自动更新

## AI 操作指引

| 任务 | 去哪里 |
|------|--------|
| 修改自动更新策略 | `auto.go`、`updater.go` |

## 本目录文件

- `internal/selfupdate/auto.go` (go, 10 symbols)
- `internal/selfupdate/auto_test.go` (go, 12 symbols)
- `internal/selfupdate/config.go` (go, 13 symbols)
- `internal/selfupdate/github.go` (go, 14 symbols)
- `internal/selfupdate/state.go` (go, 11 symbols)
- `internal/selfupdate/updater.go` (go, 27 symbols)
- `internal/selfupdate/updater_test.go` (go, 22 symbols)
- `internal/selfupdate/version.go` (go, 9 symbols)
- `internal/selfupdate/version_test.go` (go, 3 symbols)

## 关键符号

- `CompareVersions` (exported) — `internal/selfupdate/version.go:11`
- `LoadConfig` (exported) — `internal/selfupdate/config.go:27`
- `NewUpdater` (exported) — `internal/selfupdate/updater.go:39`
- `NotifyAutoUpdate` (exported) — `internal/selfupdate/auto.go:73`
- `ShouldAutoUpdate` (exported) — `internal/selfupdate/auto.go:24`
- `TestCheckLatest` (exported) — `internal/selfupdate/updater_test.go:132`
- `TestCompareVersions` (exported) — `internal/selfupdate/version_test.go:5`
- `TestDownloadAsset` (exported) — `internal/selfupdate/updater_test.go:191`
- `TestExtractFromTarGz` (exported) — `internal/selfupdate/updater_test.go:25`
- `TestLoadConfigEnvOverride` (exported) — `internal/selfupdate/auto_test.go:53`
- `TestLoadConfigFromYAML` (exported) — `internal/selfupdate/auto_test.go:78`
- `TestReplaceExecutable` (exported) — `internal/selfupdate/updater_test.go:56`
- `type AutoOptions` (exported) — `internal/selfupdate/auto.go:10`
- `type AutoResult` (exported) — `internal/selfupdate/auto.go:15`
- `type CheckResult` (exported) — `internal/selfupdate/updater.go:18`
- `type Config` (exported) — `internal/selfupdate/config.go:15`
- `type Updater` (exported) — `internal/selfupdate/updater.go:32`
- `type UpgradeOptions` (exported) — `internal/selfupdate/updater.go:26`
- `type assetRef` — `internal/selfupdate/github.go:23`
- `type checkState` — `internal/selfupdate/state.go:10`

## 相关目录

- 父目录: `internal/`

## CodeGraph 提示

- 结构/流程问题 → MCP `codegraph_explore`
- 修改前评估影响 → `codegraph_impact <符号名>`
- 手动同步索引 → `codegraph sync`

