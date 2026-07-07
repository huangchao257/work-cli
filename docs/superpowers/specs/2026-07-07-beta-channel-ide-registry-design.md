# Beta 通道 + 多 IDE Hooks 重构 — 设计文档

> 关联计划：`docs/superpowers/plans/2026-07-07-beta-channel-ide-registry.md`

## 一、Beta 通道

### 1.1 概述

CLI 自更新支持 `stable`/`beta` 两个通道。stable 使用 GitHub `releases/latest` API（仅返回最新非 pre-release），beta 使用 `releases` 列表 API（取最新 pre-release）。

### 1.2 配置

```yaml
# ~/.work/config.yaml
self_update:
  channel: stable  # stable | beta，默认 stable
```

环境变量覆盖：`WORK_SELF_UPDATE_CHANNEL=beta`

### 1.3 CLI

```
work upgrade                        # 使用配置通道（默认 stable）
work upgrade --channel beta         # 临时选 beta 通道
work upgrade --version v0.3.0-beta.1  # 指定具体 pre-release 版本
work upgrade --channel beta --check   # 仅检查 beta 通道
work upgrade --check                  # 检查配置通道
```

自动更新（`PersistentPreRunE` 中的 `TryAuto`）也跟随配置通道。

### 1.4 实现层

| 文件 | 改动 |
|------|------|
| `internal/selfupdate/config.go` | `Config` 加 `Channel string`，默认 `"stable"` |
| `internal/selfupdate/github.go` | `fetchLatestRelease(ctx, client, repo, channel)` — beta 通道调用 `releases?per_page=10` 取第一个 `prerelease=true` |
| `internal/selfupdate/updater.go` | `Check`/`Upgrade`/`UpgradeOptions` 加 `Channel`；`Upgrade` 中 `--version` 指定时走 `fetchReleaseByTag`（不受 channel 限制） |
| `internal/cli/upgrade.go` | 加 `--channel` flag |
| `internal/cli/autoupdate.go` | `TryAuto` 从 config 读 channel |
| `internal/cli/version.go` | `version --check-update` 从 config 读 channel |

### 1.5 版本解析

`resolveAsset` 中的资产匹配格式 `work_{ver}_{os}_{arch}.{ext}` 不变。beta 版本 tag 如 `v0.3.0-beta.1` 经 `Normalize` 后为 `0.3.0-beta.1`，匹配文件名 `work_0.3.0-beta.1_linux_amd64.tar.gz`。

---

## 二、多 IDE Hooks — IDE Registry

### 2.1 问题

当前 hooks 系统中 IDE 特定信息散布在 4+ 个文件中，每处都用 switch 硬编码：

```
hooks/paths.go:   HooksConfigPath (3 cases × 2 scopes)
hooks/paths.go:   ideHooksBase    (3 cases × 2 scopes)
hooks/paths.go:   commandPathForIDE (cursor 特殊处理)
hooks/events.go:  bindingsForAbstract (3 cases)
hooks/events.go:  cursorBindings + settingsBindings (2 functions)
engine/hooks.go:  installHooks 中的 merge 分发 (cursor vs default)
engine/hooks.go:  uninstallHooks 中的 unmerge 分发 (cursor vs default)
```

添加新 IDE 需修改 7 处 switch 块。

### 2.2 方案：IDE Registry

新增 `internal/platform/ide_registry.go`，将所有 IDE 元数据集中为一组注册表项：

```go
type EventBinding struct {
    Event   string // IDE 事件名，如 "beforeShellExecution"
    Matcher string // 匹配器正则，如 "Bash"（空 = 不需要）
}

type IDEInfo struct {
    ID          IDE
    DotDir      string                    // ".cursor", ".qoder", ".claude"
    HooksFile   string                    // "hooks.json" 或 "settings.json"
    RulesSubdir string                    // "rules" 或 ""（Claude 无 rules 子目录）
    RuleExt     string                    // ".mdc" 或 ".md"
    DetectFn    func() bool
    Events      map[string][]EventBinding // 抽象事件 → IDE 事件绑定列表
}
```

内置注册表（包级变量）：

```go
var ideRegistry = map[IDE]*IDEInfo{
    IDECursor: {ID: IDECursor, DotDir: ".cursor", HooksFile: "hooks.json", ...},
    IDEQoder:  {ID: IDEQoder, DotDir: ".qoder", HooksFile: "settings.json", ...},
    IDEClaude: {ID: IDEClaude, DotDir: ".claude", HooksFile: "settings.json", ...},
}
```

### 2.3 改动清单

| 文件 | 改动 |
|------|------|
| **新增** `internal/platform/ide_registry.go` | IDEInfo、EventBinding 类型 + 内置注册表 + `LookupIDE(IDE)` 函数 |
| `internal/platform/ide_paths.go` | `ideBase` 改用 registry 拿 DotDir；`RuleFile/RuleDir/MCPConfigPath/SkillDir` 无需改变（已用 `ideBase`） |
| `internal/hooks/paths.go` | `HooksConfigPath`/`ideHooksBase`/`commandPathForIDE` 删除 switch，改为 `platform.LookupIDE(ide).HooksFile` 等注册表查表 |
| `internal/hooks/events.go` | `bindingsForAbstract`/`cursorBindings`/`settingsBindings` 删除，改为 `platform.LookupIDE(ide).Events[abstract]` |
| `internal/engine/hooks.go` | `installHooks` 和 `uninstallHooks` 中的 merge/unmerge 分支改为 `info.HooksFile == "hooks.json"` → `MergeCursorHooks`，否则 → `MergeSettingsHooks`（或用 registry 方法） |

### 2.4 merge/unmerge 分发

原来 `switch ide { case "cursor": MergeCursorHooks(...); default: MergeSettingsHooks(...) }` 改为：

```go
info := platform.LookupIDE(platform.IDE(ideName))
if info.HooksFile == "hooks.json" {
    // Cursor 格式
    hooks.MergeCursorHooks(configPath, entries)
} else {
    // Qoder/Claude/未来 IDE 用 settings.json 格式
    hooks.MergeSettingsHooks(configPath, entries)
}
```

### 2.5 事件绑定

原来每种事件在 `cursorBindings` 和 `settingsBindings` 中各写一份，添加新 IDE 要决定它属于哪一类（Cursor 格式 vs Settings 格式）。现在事件绑定直接写在 registry 条目中：

```go
IDEInfo{
    ...
    Events: map[string][]EventBinding{
        EventShell:  {{Event: "beforeShellExecution"}, {Event: "afterShellExecution"}},
        EventFileEdit: {{Event: "afterFileEdit"}},
        ...
    },
}
```

`BindingsForIDE` 改为：

```go
func BindingsForIDE(ide string, events []string) ([]Binding, []string) {
    info := platform.LookupIDE(platform.IDE(ide))
    if info == nil {
        return nil, []string{fmt.Sprintf("未知 IDE: %s", ide)}
    }
    var bindings []Binding
    var warnings []string
    for _, ev := range events {
        eventBindings, ok := info.Events[ev]
        if !ok {
            warnings = append(warnings, fmt.Sprintf("%s 不支持事件 %s，已跳过", ide, ev))
            continue
        }
        for _, eb := range eventBindings {
            bindings = append(bindings, Binding{IDEEvent: eb.Event, Matcher: eb.Matcher})
        }
    }
    return bindings, warnings
}
```

### 2.6 新增 IDE 的成本

加了 registry 后，新增 IDE 只需：
1. 在 `ide_registry.go` 注册一个 `IDEInfo` 条目（含检测函数、事件绑定）
2. 在 `adapter/` 加一个 ~15 行适配器文件（复用 `baseAdapter`）

无需修改 `hooks/paths.go`、`hooks/events.go`、`engine/hooks.go`。

---

## 三、向后兼容

- 未配置 `self_update.channel` → 默认 `"stable"`，行为不变
- `work upgrade --version vX.Y.Z` 不受 channel 影响
- hooks 路径和事件绑定逻辑对外接口不变（`BindingsForIDE` 返回类型不变）
- `installed.json` 格式不变
- 所有现有测试继续通过
