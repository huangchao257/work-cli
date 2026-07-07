# Beta 通道 + 多 IDE Hooks 重构 — 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 1) CLI 自更新支持 beta 通道；2) 消除 hooks 系统中 IDE 特定 switch 块，通过注册表模式统一管理

**Architecture:** Beta 通道通过在 `selfupdate` 层增加 channel 参数实现，从 `self_update.channel` 配置读取，默认 stable。IDE 注册表新增 `internal/platform/ide_registry.go`，集中管理 IDE 元数据（dot-dir、hooks 文件名、事件绑定），hooks 包的所有 switch 改为查表调用。

**Tech Stack:** Go 1.21+, `github.com/spf13/cobra`, GitHub Releases API

## Global Constraints

- 所有现有测试继续通过：`go test ./... -count=1`
- 错误消息保持中文
- 使用 `fmt.Errorf("...: %w", err)` 错误包装
- 未配置 `self_update.channel` → 默认 `"stable"`，行为不变
- `work upgrade --version vX.Y.Z` 不受 channel 影响（走 `fetchReleaseByTag`）
- hooks 对外接口（`BindingsForIDE`、`HooksConfigPath` 等）签名不变

---

### Task 1: 新增 IDE Registry 核心文件

**Files:**
- Create: `internal/platform/ide_registry.go`
- Check: `internal/platform/ide_paths.go` (读取现有 `IDE` 类型和常量)

**Interfaces:**
- Produces: `type IDEInfo struct`, `type EventBinding struct`, `func LookupIDE(IDE) *IDEInfo`, `func AllIDEs() []IDE`
- Consumes: `platform.IDE`, `platform.IDEQoder`, `platform.IDECursor`, `platform.IDEClaude`（均已有）

- [ ] **Step 1: Create ide_registry.go**

```go
package platform

// EventBinding 描述 IDE 事件名与可选的匹配器正则。
type EventBinding struct {
	Event   string // IDE 事件名，如 "beforeShellExecution"
	Matcher string // 匹配器正则，如 "Bash"；空表示无需匹配
}

// IDEInfo 描述一个 IDE 的所有元数据，取代 hooks/paths.go 和 hooks/events.go 中的 switch 块。
type IDEInfo struct {
	ID          IDE
	DotDir      string                    // ".cursor", ".qoder", ".claude"
	HooksFile   string                    // "hooks.json" 或 "settings.json"
	RulesSubdir string                    // "rules" 或 ""（Claude 无 rules 子目录）
	RuleExt     string                    // ".mdc" 或 ".md"
	DetectFn    func() bool
	Events      map[string][]EventBinding // 抽象事件名 → IDE 事件绑定列表
}

// 抽象事件常量（从 hooks 包移过来以消除依赖；hooks 包引用这里的常量名）
const (
	EventShell    = "shell"
	EventMCP      = "mcp"
	EventFileRead = "file_read"
	EventFileEdit = "file_edit"
	EventPrompt   = "prompt"
	EventSession  = "session"
	EventTool     = "tool"
)

var ideRegistry = map[IDE]*IDEInfo{}

func init() {
	registerIDE(&IDEInfo{
		ID: IDECursor, DotDir: ".cursor", HooksFile: "hooks.json",
		RulesSubdir: "rules", RuleExt: ".mdc",
		DetectFn: detectCursor,
		Events: map[string][]EventBinding{
			EventShell:    {{Event: "beforeShellExecution"}, {Event: "afterShellExecution"}},
			EventMCP:      {{Event: "beforeMCPExecution"}, {Event: "afterMCPExecution"}},
			EventFileRead: {{Event: "beforeReadFile"}},
			EventFileEdit: {{Event: "afterFileEdit"}},
			EventPrompt:   {{Event: "beforeSubmitPrompt"}},
			EventSession:  {{Event: "sessionStart"}, {Event: "sessionEnd"}},
			EventTool:     {{Event: "preToolUse"}, {Event: "postToolUse"}},
		},
	})
	registerIDE(&IDEInfo{
		ID: IDEQoder, DotDir: ".qoder", HooksFile: "settings.json",
		RulesSubdir: "rules", RuleExt: ".md",
		DetectFn: detectQoder,
		Events: map[string][]EventBinding{
			EventShell:    {{Event: "PreToolUse", Matcher: "Bash"}, {Event: "PostToolUse", Matcher: "Bash"}},
			EventMCP:      {{Event: "PreToolUse", Matcher: "MCP.*|mcp__.*"}, {Event: "PostToolUse", Matcher: "MCP.*|mcp__.*"}},
			EventFileRead: {{Event: "PreToolUse", Matcher: "Read"}},
			EventFileEdit: {{Event: "PostToolUse", Matcher: "Write|Edit"}},
			EventPrompt:   {{Event: "UserPromptSubmit"}},
			EventSession:  nil, // Qoder 不支持 session
			EventTool:     {{Event: "PreToolUse"}, {Event: "PostToolUse"}},
		},
	})
	registerIDE(&IDEInfo{
		ID: IDEClaude, DotDir: ".claude", HooksFile: "settings.json",
		RulesSubdir: "", RuleExt: ".md",
		DetectFn: detectClaude,
		Events: map[string][]EventBinding{
			EventShell:    {{Event: "PreToolUse", Matcher: "Bash"}, {Event: "PostToolUse", Matcher: "Bash"}},
			EventMCP:      {{Event: "PreToolUse", Matcher: "MCP.*|mcp__.*"}, {Event: "PostToolUse", Matcher: "MCP.*|mcp__.*"}},
			EventFileRead: {{Event: "PreToolUse", Matcher: "Read"}},
			EventFileEdit: {{Event: "PostToolUse", Matcher: "Write|Edit"}},
			EventPrompt:   {{Event: "UserPromptSubmit"}},
			EventSession:  {{Event: "SessionStart"}, {Event: "SessionEnd"}},
			EventTool:     {{Event: "PreToolUse"}, {Event: "PostToolUse"}},
		},
	})
}

func registerIDE(info *IDEInfo) { ideRegistry[info.ID] = info }

// LookupIDE 返回指定 IDE 的注册信息。未找到返回 nil。
func LookupIDE(ide IDE) *IDEInfo { return ideRegistry[ide] }

// AllIDEs 返回所有已注册的 IDE 信息（用于遍历）。
func AllIDEs() []*IDEInfo {
	out := make([]*IDEInfo, 0, len(ideRegistry))
	for _, info := range ideRegistry {
		out = append(out, info)
	}
	return out
}
```

- [ ] **Step 2: 将 detect 函数从 adapter 包移到 platform 包**

三个 `detect*` 函数（`detectCursor`/`detectQoder`/`detectClaude`）目前在 `internal/adapter/claude.go`、`cursor.go`、`qoder.go` 中。需要在 `internal/platform/` 中新建独立文件或合并到一个 `ide_detect.go`：

```go
// internal/platform/ide_detect.go
package platform

import (
	"os"
	"path/filepath"
)

func detectCursor() bool {
	home, err := UserHome()
	if err != nil { return false }
	_, err = os.Stat(filepath.Join(home, ".cursor"))
	return err == nil
}

func detectQoder() bool {
	home, err := UserHome()
	if err != nil { return false }
	return dirExists(filepath.Join(home, ".qoder"))
}

func detectClaude() bool { /* 现有逻辑 */ }

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
```

同时更新 `internal/adapter/claude.go`、`cursor.go`、`qoder.go` 中的 `detectFn` 引用为 `platform.DetectCursor` 等。

- [ ] **Step 3: 更新 hooks/events.go 中的事件常量引用**

将 `hooks/events.go` 中的 `EventShell` 等常量改为引用 `platform.EventShell`（或加 type alias）：

```go
// internal/hooks/events.go
import "github.com/huangchao257/work-cli/internal/platform"

// 保持向后兼容的类型别名
const (
	EventShell    = platform.EventShell
	EventMCP      = platform.EventMCP
	EventFileRead = platform.EventFileRead
	EventFileEdit = platform.EventFileEdit
	EventPrompt   = platform.EventPrompt
	EventSession  = platform.EventSession
	EventTool     = platform.EventTool
)
```

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: PASS (无编译错误)

- [ ] **Step 5: 提交**

```bash
git add internal/platform/ide_registry.go internal/platform/ide_detect.go \
        internal/adapter/claude.go internal/adapter/cursor.go internal/adapter/qoder.go \
        internal/hooks/events.go
git commit -m "feat(platform): add IDE registry with centralized metadata and event bindings"
```

---

### Task 2: selfupdate 配置加 Channel 字段

**Files:**
- Modify: `internal/selfupdate/config.go`

**Interfaces:**
- Consumes: 现有 `fileConfig` 结构体
- Produces: `Config.Channel string`（默认 `"stable"`）

- [ ] **Step 1: 修改 Config 结构体和 config.go**

```go
type Config struct {
	Enabled       bool
	CheckInterval time.Duration
	Channel       string // "stable" 或 "beta"，默认 "stable"
}

type fileConfig struct {
	SelfUpdate struct {
		Enabled       *bool  `yaml:"enabled"`
		CheckInterval string `yaml:"check_interval"`
		Channel       string `yaml:"channel"`
	} `yaml:"self_update"`
}

func LoadConfig() (Config, error) {
	cfg := defaultConfig()
	// ...existing code...
	if fc.SelfUpdate.Channel != "" {
		cfg.Channel = fc.SelfUpdate.Channel
	}
	return applyEnv(cfg), nil
}

func defaultConfig() Config {
	return Config{
		Enabled:       true,
		CheckInterval: defaultCheckInterval,
		Channel:       "stable",
	}
}

func applyEnv(cfg Config) Config {
	// ...existing WORK_AUTO_UPDATE...
	if ch := os.Getenv("WORK_SELF_UPDATE_CHANNEL"); ch != "" {
		ch = strings.ToLower(strings.TrimSpace(ch))
		if ch == "beta" || ch == "stable" {
			cfg.Channel = ch
		}
	}
	return cfg
}

func ValidateChannel(ch string) error {
	switch ch {
	case "stable", "beta":
		return nil
	default:
		return fmt.Errorf("未知更新通道: %s（支持 stable 或 beta）", ch)
	}
}
```

- [ ] **Step 2: 运行测试**

Run: `go test ./internal/selfupdate/ -v -count=1`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/selfupdate/config.go
git commit -m "feat(selfupdate): add Channel field to Config (stable/beta)"
```

---

### Task 3: github.go 支持 channel 感知的 release 获取

**Files:**
- Modify: `internal/selfupdate/github.go`

**Interfaces:**
- Consumes: `Config.Channel` (from Task 2)
- Produces: `fetchLatestRelease(ctx, client, repo, channel string) (*releaseInfo, error)`

- [ ] **Step 1: 修改 fetchLatestRelease 支持 channel**

```go
func fetchLatestRelease(ctx context.Context, client *http.Client, repo, channel string) (*releaseInfo, error) {
	switch channel {
	case "beta":
		return fetchLatestPrerelease(ctx, client, repo)
	default:
		return fetchStableRelease(ctx, client, repo)
	}
}

func fetchStableRelease(ctx context.Context, client *http.Client, repo string) (*releaseInfo, error) {
	info, err := fetchReleaseResponse(ctx, client, "releases/latest", repo, "获取最新版本失败")
	if err != nil {
		return nil, err
	}
	if info.TagName == "" {
		return nil, fmt.Errorf("Release 缺少 tag_name 字段")
	}
	return info, nil
}

// fetchLatestPrerelease 从 releases 列表 API 获取最新 pre-release。
func fetchLatestPrerelease(ctx context.Context, client *http.Client, repo string) (*releaseInfo, error) {
	info, err := fetchReleaseResponse(ctx, client, "releases?per_page=20", repo, "获取 beta 版本失败")
	if err != nil {
		return nil, err
	}
	// releases 列表 API 返回数组；fetchReleaseResponse 只解析单个对象。
	// 需要单独处理数组响应。
	resp, err := gitHubAPI(ctx, client, "releases?per_page=20", repo)
	if err != nil {
		return nil, fmt.Errorf("获取 beta 版本失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var releases []releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("解析 Release 列表失败: %w", err)
	}
	for _, r := range releases {
		if r.Prerelease && r.TagName != "" {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("未找到 beta 版本")
}
```

注意：`releaseInfo` 结构体需加 `Prerelease bool` 字段：

```go
type releaseInfo struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}
```

- [ ] **Step 2: 确保 fetchReleaseByTag 不受 channel 影响**

`fetchReleaseByTag` 已独立存在，通过 `releases/tags/{tag}` API，不涉及 channel。无需修改。

- [ ] **Step 3: 运行测试**

Run: `go test ./internal/selfupdate/ -v -count=1`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/selfupdate/github.go
git commit -m "feat(selfupdate): support beta channel via releases list API"
```

---

### Task 4: updater.go — Check/Upgrade 传递 Channel

**Files:**
- Modify: `internal/selfupdate/updater.go`

**Interfaces:**
- Consumes: `fetchLatestRelease(ctx, client, repo, channel)`, `Config.Channel`
- Produces: `Check(ctx, channel)`, `Upgrade(ctx, opts)`, `UpgradeOptions.Channel`

- [ ] **Step 1: 修改 Check 方法**

```go
func (u *Updater) Check(ctx context.Context, channel string) (*CheckResult, error) {
	if channel == "" {
		channel = "stable"
	}
	info, err := fetchLatestRelease(ctx, u.HTTPClient, u.repo(), channel)
	if err != nil {
		return nil, fmt.Errorf("查询最新版本失败: %w", err)
	}
	// ...rest unchanged...
}
```

- [ ] **Step 2: UpgradeOptions 加 Channel**

```go
type UpgradeOptions struct {
	Version   string
	DryRun    bool
	CheckOnly bool
	Channel   string // "stable" 或 "beta"
}
```

- [ ] **Step 3: 修改 Upgrade 方法**

```go
func (u *Updater) Upgrade(ctx context.Context, opts UpgradeOptions) (*CheckResult, error) {
	if opts.Version != "" {
		// 指定版本不受 channel 限制
		// ...existing tag-based logic...
		return u.upgradeToVersion(ctx, opts)
	}
	channel := opts.Channel
	if channel == "" {
		channel = "stable"
	}
	if err := ValidateChannel(channel); err != nil {
		return nil, err
	}
	// ...existing check + upgrade logic, passing channel to fetchLatestRelease...
}
```

- [ ] **Step 4: 更新所有调用者**

Run: `go build ./...` 找到所有调用 `u.Check(ctx)` 和 `u.Upgrade(ctx, opts)` 的地方，传 channel 参数。

- [ ] **Step 5: 运行测试**

Run: `go test ./internal/selfupdate/ -v -count=1`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/selfupdate/updater.go internal/selfupdate/updater_test.go
git commit -m "feat(selfupdate): pass channel through Check/Upgrade to fetchLatestRelease"
```

---

### Task 5: CLI wiring — upgrade + autoupdate + version

**Files:**
- Modify: `internal/cli/upgrade.go`
- Modify: `internal/cli/autoupdate.go`
- Modify: `internal/cli/version.go`

**Interfaces:**
- Consumes: `selfupdate.UpgradeOptions.Channel`, `selfupdate.ValidateChannel`
- Produces: `--channel` flag on upgrade command

- [ ] **Step 1: upgrade.go 加 --channel flag**

```go
var upgradeChannel string

var upgradeCmd = &cobra.Command{
	// ...
	RunE: func(cmd *cobra.Command, args []string) error {
		// ...
		res, err := updater.Upgrade(ctx, selfupdate.UpgradeOptions{
			Version:   upgradeVersion,
			DryRun:    upgradeDryRun || dryRun,
			CheckOnly: upgradeCheckOnly,
			Channel:   upgradeChannel,
		})
		// ...
	},
}

func init() {
	// ...existing flags...
	upgradeCmd.Flags().StringVar(&upgradeChannel, "channel", "", "更新通道：stable 或 beta（默认从配置读取）")
}
```

- [ ] **Step 2: autoupdate.go 传递配置的 channel**

```go
func runAutoUpdate(cmd *cobra.Command, args []string) error {
	// ...
	cfg, _ := selfupdate.LoadConfig()
	channel := cfg.Channel
	// ...
	res, err := updater.Check(ctx, channel)
	// ...
}
```

- [ ] **Step 3: version.go 传递配置的 channel**

```go
// versionCmd.RunE:
cfg, _ := selfupdate.LoadConfig()
res, err := selfupdate.NewUpdater(Version).Check(signalContext(), cfg.Channel)
```

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/cli/ -v -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/cli/upgrade.go internal/cli/autoupdate.go internal/cli/version.go
git commit -m "feat(cli): wire --channel flag and config to upgrade/autoupdate/version"
```

---

### Task 6: hooks/paths.go — 消除 IDE switch

**Files:**
- Modify: `internal/hooks/paths.go`

**Interfaces:**
- Consumes: `platform.LookupIDE(platform.IDE(ide))`
- Produces: `HooksConfigPath`, `HooksScriptDir`, `commandPathForIDE`（签名不变）

- [ ] **Step 1: 重写 HooksConfigPath**

```go
func HooksConfigPath(ide, scope string) (string, error) {
	info := platform.LookupIDE(platform.IDE(ide))
	if info == nil {
		return "", fmt.Errorf("未知 IDE: %s", ide)
	}
	base, err := ideHooksBase(ide, scope)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(base), info.HooksFile), nil
}
```

- [ ] **Step 2: 重写 ideHooksBase**

```go
func ideHooksBase(ide, scope string) (string, error) {
	info := platform.LookupIDE(platform.IDE(ide))
	if info == nil {
		return "", fmt.Errorf("未知 IDE: %s", ide)
	}
	if scope == "project" {
		root, err := platform.ProjectRoot()
		if err != nil { return "", err }
		return filepath.Join(root, info.DotDir, "hooks"), nil
	}
	home, err := platform.UserHome()
	if err != nil { return "", err }
	return filepath.Join(home, info.DotDir, "hooks"), nil
}
```

- [ ] **Step 3: 重写 commandPathForIDE**

```go
func commandPathForIDE(ide, scope, kitName, scriptName string) (string, error) {
	info := platform.LookupIDE(platform.IDE(ide))
	if info == nil {
		return "", fmt.Errorf("未知 IDE: %s", ide)
	}
	dir, err := HooksScriptDir(ide, scope, kitName)
	if err != nil { return "", err }
	abs, err := filepath.Abs(filepath.Join(dir, scriptName))
	if err != nil { return "", err }
	if info.HooksFile != "hooks.json" { // 非 Cursor 格式：用绝对路径
		return abs, nil
	}
	// Cursor：返回相对路径
	var base string
	if scope == "project" {
		base, err = platform.ProjectRoot()
	} else {
		base, err = platform.UserHome()
		if err == nil { base = filepath.Join(base, info.DotDir) }
	}
	if err != nil { return abs, nil }
	rel, err := filepath.Rel(base, abs)
	if err != nil { return abs, nil }
	return filepath.ToSlash(rel), nil
}
```

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/hooks/ -v -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/hooks/paths.go
git commit -m "refactor(hooks): replace IDE switch blocks with platform.LookupIDE"
```

---

### Task 7: hooks/events.go — 消除 bindingsForAbstract/cursorBindings/settingsBindings

**Files:**
- Modify: `internal/hooks/events.go`

**Interfaces:**
- Consumes: `platform.LookupIDE(ide).Events`
- Produces: `BindingsForIDE`, `AbstractForIDEReport`（签名不变）

- [ ] **Step 1: 替换 BindingsForIDE 实现**

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

- [ ] **Step 2: 更新 AbstractForIDEReport**

```go
func AbstractForIDEReport(ide, ideEvent string) string {
	info := platform.LookupIDE(platform.IDE(ide))
	if info == nil { return ideEvent }
	for abstract, bindings := range info.Events {
		for _, b := range bindings {
			if strings.EqualFold(b.Event, ideEvent) {
				return abstract
			}
		}
	}
	return ideEvent
}
```

- [ ] **Step 3: 删除 cursorBindings/settingsBindings/bindingsForAbstract**

这三个函数不再需要，可以删除（约 80 行代码）。

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/hooks/ -v -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/hooks/events.go
git commit -m "refactor(hooks): use IDE registry event bindings, delete switch-based binding functions"
```

---

### Task 8: engine/hooks.go — merge/unmerge 分发改为 registry

**Files:**
- Modify: `internal/engine/hooks.go`

**Interfaces:**
- Consumes: `platform.LookupIDE(platform.IDE(ideName)).HooksFile`
- Produces: unchange（行为不变）

- [ ] **Step 1: 替换 installHooks 中的 merge 分发**

```go
// 原代码:
// switch ideName {
// case "cursor":
//     hooks.MergeCursorHooks(configPath, entries)
// default:
//     hooks.MergeSettingsHooks(configPath, entries)
// }

// 新代码:
info := platform.LookupIDE(platform.IDE(ideName))
if info == nil {
    return Result{}, fmt.Errorf("未知 IDE: %s", ideName)
}
if info.HooksFile == "hooks.json" {
    if err := hooks.MergeCursorHooks(configPath, entries); err != nil {
        return Result{}, fmt.Errorf("合并 Cursor hooks 失败: %w", err)
    }
} else {
    if err := hooks.MergeSettingsHooks(configPath, entries); err != nil {
        return Result{}, fmt.Errorf("合并 settings hooks 失败: %w", err)
    }
}
```

- [ ] **Step 2: 替换 uninstallHooks 和 uninstallHooksFallback 中的 unmerge 分发**

同样的 `info.HooksFile == "hooks.json"` 判断。

- [ ] **Step 3: 运行测试**

Run: `go test ./internal/engine/ -v -count=1`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/engine/hooks.go
git commit -m "refactor(engine): use IDE registry for hooks merge/unmerge dispatch"
```

---

### Task 9: platform/ide_paths.go — ideBase 使用 registry

**Files:**
- Modify: `internal/platform/ide_paths.go`

**Interfaces:**
- Consumes: `ideRegistry`
- Produces: `ideBase` 用 `DotDir` 查表替代 switch

- [ ] **Step 1: 简化 ideBase**

```go
func ideBase(ide IDE, scope string) (string, error) {
	info := LookupIDE(ide)
	if info == nil {
		return "", errUnknownIDE(ide)
	}
	if scope == "project" {
		root, err := ProjectRoot()
		if err != nil { return "", err }
		return filepath.Join(root, info.DotDir), nil
	}
	home, err := UserHome()
	if err != nil { return "", err }
	return filepath.Join(home, info.DotDir), nil
}
```

- [ ] **Step 2: RuleDir/RuleFile 用 registry 替代 switch**

```go
func RuleDir(ide IDE, scope string) (string, error) {
	info := LookupIDE(ide)
	if info == nil { return "", errUnknownIDE(ide) }
	base, err := ideBase(ide, scope)
	if err != nil { return "", err }
	if info.RulesSubdir == "" {
		return base, nil // Claude: rules 直接放在 ideBase
	}
	return filepath.Join(base, info.RulesSubdir), nil
}

func RuleFile(ide IDE, scope, ruleID string) (string, error) {
	info := LookupIDE(ide)
	if info == nil { return "", errUnknownIDE(ide) }
	dir, err := RuleDir(ide, scope)
	if err != nil { return "", err }
	ext := info.RuleExt
	if ext == "" { ext = ".md" }
	return filepath.Join(dir, ruleID+ext), nil
}
```

- [ ] **Step 3: 运行完整测试**

Run: `go test ./internal/platform/ ./internal/adapter/ -v -count=1`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/platform/ide_paths.go
git commit -m "refactor(platform): use IDE registry DotDir/RuleExt in path functions"
```

---

### Task 10: 全面测试 + 清理

**Files:**
- None (仅运行测试)

- [ ] **Step 1: 全量测试**

Run: `go test ./... -count=1`
Expected: ALL PASS

- [ ] **Step 2: Race 检测**

Run: `go test -race ./... -count=1`
Expected: ALL PASS (no data races)

- [ ] **Step 3: 检查死代码**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 4: 提交（如有清理）**

```bash
git add -A
git diff --cached --stat  # 确认无意外变更
# 如果有死代码清理，提交；否则跳过
```

---

### Task 11: 最终全分支审查

Use `superpowers:requesting-code-review` to do final review covering beta channel + IDE registry changes.
