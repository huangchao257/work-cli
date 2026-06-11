# 资源管理模块 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 `work` CLI 的资源管理模块 MVP——员工可通过一条命令安装/列出/卸载/更新：(1) Qoder/Cursor/Claude Code 资源套装；(2) 外部 CLI 委托安装（如 `work install openspec` 执行官方安装命令）。

**Architecture:** 统一 Install Engine 按 manifest `type` 分发：`bundle` → IDE 适配器；`cli` → CLI Runner 执行 `installer.yaml` 中的命令。`internal/source` 从 Registry/Git/本地拉取包；`internal/platform` 处理跨平台路径与 shell 执行。

**Tech Stack:** Go 1.26+、Cobra、yaml.v3、标准库（net/http、os/exec git）

**Spec:** `docs/superpowers/specs/2026-06-11-work-cli-design.md`

---

## 文件结构总览

| 路径 | 职责 |
|------|------|
| `cmd/work/main.go` | 入口 |
| `internal/cli/root.go` | Cobra 根命令与全局 flags |
| `internal/cli/install.go` | install 子命令 |
| `internal/cli/list.go` | list 子命令 |
| `internal/cli/uninstall.go` | uninstall 子命令 |
| `internal/cli/update.go` | update 子命令 |
| `internal/bundle/manifest.go` | bundle.yaml 类型定义 |
| `internal/bundle/parse.go` | bundle YAML 解析与校验 |
| `internal/bundle/validate.go` | env/globs 校验 |
| `internal/installer/manifest.go` | installer.yaml 类型定义 |
| `internal/installer/parse.go` | installer 解析 |
| `internal/installer/runner.go` | 跨平台执行 install/uninstall/update |
| `internal/pkg/manifest/detect.go` | 识别目录是 bundle 还是 cli |
| `internal/platform/paths.go` | Home/WorkDir/ProjectRoot |
| `internal/platform/ide_paths.go` | IDE × scope × resource 路径 |
| `internal/platform/env_hint.go` | 跨平台 env 设置提示 |
| `internal/state/store.go` | installed.json 读写 |
| `internal/state/types.go` | BundleInstallState 类型 |
| `internal/adapter/adapter.go` | Adapter 接口与注册表 |
| `internal/adapter/mcp_merge.go` | MCP JSON merge/unmerge |
| `internal/adapter/cursor/adapter.go` | Cursor 适配器 |
| `internal/adapter/qoder/adapter.go` | Qoder 适配器 |
| `internal/adapter/claude/adapter.go` | Claude Code 适配器 |
| `internal/source/ref.go` | ref 解析（registry/git/local） |
| `internal/source/local.go` | 本地目录 source |
| `internal/source/git.go` | git shallow clone source |
| `internal/source/registry.go` | HTTP registry source |
| `internal/engine/install.go` | 统一安装入口（分发 bundle/cli） |
| `internal/engine/bundle.go` | bundle 安装分支 |
| `internal/engine/cli.go` | cli 委托安装分支 |
| `internal/engine/uninstall.go` | 卸载编排 |
| `internal/engine/update.go` | 更新编排 |
| `internal/output/human.go` | 中文友好输出 |
| `internal/output/json.go` | JSON 输出 |
| `examples/dev-kit/` | 示例 bundle |
| `examples/openspec/` | 示例 cli installer |
| `.github/workflows/ci.yml` | 测试矩阵 |
| `README.md` | 用户文档 |

---

### Task 1: 项目脚手架

**Files:**
- Create: `go.mod`, `cmd/work/main.go`, `internal/cli/root.go`

- [ ] **Step 1: 初始化 go.mod**

```bash
cd /home/zmn/projects/work-cli
go mod init github.com/huangchao257/work-cli
```

`go.mod` 内容：

```
module github.com/huangchao257/work-cli

go 1.26

require (
	github.com/spf13/cobra v1.9.1
	gopkg.in/yaml.v3 v3.0.1
)
```

- [ ] **Step 2: 写根命令**

`cmd/work/main.go`:

```go
package main

import (
	"os"

	"github.com/huangchao257/work-cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
```

`internal/cli/root.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	scope  string
	ide    string
	dryRun bool
	asJSON bool
)

var rootCmd = &cobra.Command{
	Use:   "work",
	Short: "公司统一 CLI 入口",
	Long:  "work 是企业级命令行工具。资源管理模块用于安装和管理 AI IDE 的 Skills、MCP、Rules。",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&scope, "scope", "user", "安装范围：user 或 project")
	rootCmd.PersistentFlags().StringVar(&ide, "ide", "", "目标 IDE，逗号分隔：qoder,cursor,claude")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "仅预览将写入的文件")
	rootCmd.PersistentFlags().BoolVar(&asJSON, "json", false, "JSON 格式输出")
}

func Execute() error {
	return rootCmd.Execute()
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	return 1
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(exitCode(err))
}
```

- [ ] **Step 3: 验证编译**

```bash
go mod tidy
go build -o bin/work ./cmd/work
./bin/work --help
```

Expected: 显示 `work` 帮助信息

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum cmd/work/main.go internal/cli/root.go
git commit -m "chore: scaffold work CLI project"
```

---

### Task 2: platform 包（跨平台路径）

**Files:**
- Create: `internal/platform/paths.go`, `internal/platform/ide_paths.go`, `internal/platform/env_hint.go`
- Test: `internal/platform/paths_test.go`, `internal/platform/ide_paths_test.go`, `internal/platform/env_hint_test.go`

- [ ] **Step 1: 写失败测试 paths**

`internal/platform/paths_test.go`:

```go
package platform

import (
	"path/filepath"
	"testing"
)

func TestWorkConfigDir(t *testing.T) {
	t.Setenv("HOME", "/tmp/testhome")
	dir, err := WorkConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/testhome", ".work")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/platform/... -run TestWorkConfigDir -v
```

Expected: FAIL（WorkConfigDir 未定义）

- [ ] **Step 3: 实现 paths.go**

`internal/platform/paths.go`:

```go
package platform

import (
	"os"
	"path/filepath"
)

func UserHome() (string, error) {
	return os.UserHomeDir()
}

func WorkConfigDir() (string, error) {
	home, err := UserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".work"), nil
}

func WorkStatePath(scope string) (string, error) {
	base, err := WorkConfigDir()
	if err != nil {
		return "", err
	}
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".work", "installed.json"), nil
	}
	return filepath.Join(base, "installed.json"), nil
}

func ProjectRoot() (string, error) {
	return os.Getwd()
}
```

- [ ] **Step 4: 写 ide_paths 测试与实现**

`internal/platform/ide_paths.go`:

```go
package platform

import (
	"path/filepath"
)

type IDE string

const (
	IDEQoder   IDE = "qoder"
	IDECursor  IDE = "cursor"
	IDEClaude  IDE = "claude"
)

type ResourceKind string

const (
	ResourceSkill ResourceKind = "skill"
	ResourceRule  ResourceKind = "rule"
	ResourceMCP   ResourceKind = "mcp"
)

func SkillDir(ide IDE, scope, skillID string) (string, error) {
	base, err := ideBase(ide, scope)
	if err != nil {
		return "", err
	}
	switch ide {
	case IDEQoder:
		return filepath.Join(base, "skills", skillID), nil
	case IDECursor:
		return filepath.Join(base, "skills", skillID), nil
	case IDEClaude:
		return filepath.Join(base, "skills", skillID), nil
	default:
		return "", errUnknownIDE(ide)
	}
}

func RuleDir(ide IDE, scope string) (string, error) {
	base, err := ideBase(ide, scope)
	if err != nil {
		return "", err
	}
	switch ide {
	case IDEQoder:
		return filepath.Join(base, "rules"), nil
	case IDECursor:
		return filepath.Join(base, "rules"), nil
	case IDEClaude:
		if scope == "project" {
			root, err := ProjectRoot()
			if err != nil {
				return "", err
			}
			return filepath.Join(root, ".claude"), nil
		}
		home, err := UserHome()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".claude"), nil
	default:
		return "", errUnknownIDE(ide)
	}
}

func MCPConfigPath(ide IDE, scope string) (string, error) {
	switch ide {
	case IDECursor:
		if scope == "project" {
			root, err := ProjectRoot()
			if err != nil {
				return "", err
			}
			return filepath.Join(root, ".cursor", "mcp.json"), nil
		}
		home, err := UserHome()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cursor", "mcp.json"), nil
	case IDEQoder:
		// MVP: user-level MCP config path; project scope uses .qoder in project root
		if scope == "project" {
			root, err := ProjectRoot()
			if err != nil {
				return "", err
			}
			return filepath.Join(root, ".qoder", "mcp.json"), nil
		}
		home, err := UserHome()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".qoder", "mcp.json"), nil
	case IDEClaude:
		if scope == "project" {
			root, err := ProjectRoot()
			if err != nil {
				return "", err
			}
			return filepath.Join(root, ".claude", "mcp.json"), nil
		}
		home, err := UserHome()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".claude", "mcp.json"), nil
	default:
		return "", errUnknownIDE(ide)
	}
}

func ideBase(ide IDE, scope string) (string, error) {
	if scope == "project" {
		root, err := ProjectRoot()
		if err != nil {
			return "", err
		}
		switch ide {
		case IDEQoder:
			return filepath.Join(root, ".qoder"), nil
		case IDECursor:
			return filepath.Join(root, ".cursor"), nil
		case IDEClaude:
			return filepath.Join(root, ".claude"), nil
		}
	}
	home, err := UserHome()
	if err != nil {
		return "", err
	}
	switch ide {
	case IDEQoder:
		return filepath.Join(home, ".qoder"), nil
	case IDECursor:
		return filepath.Join(home, ".cursor"), nil
	case IDEClaude:
		return filepath.Join(home, ".claude"), nil
	default:
		return "", errUnknownIDE(ide)
	}
}

type unknownIDEError string

func (e unknownIDEError) Error() string { return "unknown IDE: " + string(e) }
func errUnknownIDE(ide IDE) error       { return unknownIDEError(ide) }
```

`internal/platform/ide_paths_test.go` 至少覆盖 Cursor user skill 路径与 project mcp 路径。

- [ ] **Step 5: 实现 env_hint.go**

`internal/platform/env_hint.go`:

```go
package platform

import "runtime"

func EnvSetHint(name string) string {
	switch runtime.GOOS {
	case "windows":
		return "PowerShell: $env:" + name + '="你的值"' + "\nCMD: set " + name + "=你的值"
	default:
		return "export " + name + "=你的值"
	}
}
```

- [ ] **Step 6: 运行全部 platform 测试**

```bash
go test ./internal/platform/... -v
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/platform/
git commit -m "feat: add cross-platform path resolution"
```

---

### Task 3: bundle manifest 解析与校验

**Files:**
- Create: `internal/bundle/manifest.go`, `internal/bundle/parse.go`, `internal/bundle/validate.go`
- Test: `internal/bundle/parse_test.go`, `internal/bundle/validate_test.go`

- [ ] **Step 1: 写类型定义**

`internal/bundle/manifest.go`:

```go
package bundle

type Manifest struct {
	Name        string     `yaml:"name"`
	Version     string     `yaml:"version"`
	Description string     `yaml:"description"`
	Env         []EnvVar   `yaml:"env"`
	Resources   Resources  `yaml:"resources"`
	Targets     []string   `yaml:"targets"`
}

type EnvVar struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type Resources struct {
	Skills []SkillResource `yaml:"skills"`
	Rules  []RuleResource  `yaml:"rules"`
	MCP    []MCPResource   `yaml:"mcp"`
}

type SkillResource struct {
	ID     string `yaml:"id"`
	Source string `yaml:"source"`
}

type RuleResource struct {
	ID     string `yaml:"id"`
	Source string `yaml:"source"`
	Apply  string `yaml:"apply"`
	Globs  []string `yaml:"globs"`
}

type MCPResource struct {
	ID     string            `yaml:"id"`
	Source string            `yaml:"source"`
	Env    []map[string]string `yaml:"env"`
}
```

- [ ] **Step 2: 写失败测试 parse**

`internal/bundle/parse_test.go` 使用内联 YAML 测试 `ParseFile`。

- [ ] **Step 3: 实现 parse.go**

```go
package bundle

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ManifestFileName = "bundle.yaml"

func ParseDir(dir string) (*Manifest, error) {
	return ParseFile(filepath.Join(dir, ManifestFileName))
}

func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 bundle 配置失败: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 bundle.yaml 失败: %w", err)
	}
	if err := Validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}
```

- [ ] **Step 4: 实现 validate.go**

```go
package bundle

import (
	"fmt"
	"os"
	"strings"
)

func Validate(m *Manifest) error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("bundle.yaml 缺少 name 字段")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("bundle.yaml 缺少 version 字段")
	}
	for _, r := range m.Resources.Rules {
		if r.Apply == "files" && len(r.Globs) == 0 {
			return fmt.Errorf("规则 %s 的 apply=files 时必须提供 globs", r.ID)
		}
		if r.Apply != "always" && r.Apply != "manual" && r.Apply != "files" {
			return fmt.Errorf("规则 %s 的 apply 无效: %s", r.ID, r.Apply)
		}
	}
	return nil
}

func CheckRequiredEnv(m *Manifest) []string {
	var missing []string
	for _, e := range m.Env {
		if !e.Required {
			continue
		}
		if os.Getenv(e.Name) == "" {
			missing = append(missing, e.Name)
		}
	}
	return missing
}
```

- [ ] **Step 5: 运行测试**

```bash
go test ./internal/bundle/... -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/bundle/
git commit -m "feat: parse and validate bundle manifests"
```

---

### Task 3b: CLI Installer（外部 CLI 委托安装）

**Files:**
- Create: `internal/installer/manifest.go`, `internal/installer/parse.go`, `internal/installer/runner.go`
- Create: `internal/pkg/manifest/detect.go`
- Test: `internal/installer/runner_test.go`, `internal/pkg/manifest/detect_test.go`

- [ ] **Step 1: manifest 类型检测**

`internal/pkg/manifest/detect.go`:

```go
package manifest

import "os"

type Kind string

const (
	KindBundle Kind = "bundle"
	KindCLI    Kind = "cli"
)

func DetectKind(dir string) (Kind, error) {
	if fileExists(filepath.Join(dir, "installer.yaml")) {
		return KindCLI, nil
	}
	if fileExists(filepath.Join(dir, "bundle.yaml")) {
		return KindBundle, nil
	}
	return "", fmt.Errorf("未找到 installer.yaml 或 bundle.yaml")
}
```

- [ ] **Step 2: installer.yaml 类型与解析**

`internal/installer/manifest.go`:

```go
type Manifest struct {
	Type        string         `yaml:"type"` // cli
	Name        string         `yaml:"name"`
	Version     string         `yaml:"version"`
	Description string         `yaml:"description"`
	Env         []installer.EnvVar `yaml:"env"`
	Install     CommandSpec    `yaml:"install"`
	Verify      *VerifySpec    `yaml:"verify"`
	Uninstall   *CommandSpec   `yaml:"uninstall"`
	Update      *CommandSpec   `yaml:"update"`
}

type CommandSpec struct {
	Run       string                       `yaml:"run"`
	Platforms map[string]PlatformCommand `yaml:"platforms"`
}

type PlatformCommand struct {
	Run string `yaml:"run"`
}
```

- [ ] **Step 3: runner 实现**

`internal/installer/runner.go`:

```go
func ResolveCommand(spec CommandSpec) (string, error) {
	if p, ok := spec.Platforms[runtime.GOOS]; ok && p.Run != "" {
		return p.Run, nil
	}
	if spec.Run != "" {
		return spec.Run, nil
	}
	return "", fmt.Errorf("当前系统 %s 无可用安装命令", runtime.GOOS)
}

func Run(ctx context.Context, command string) error {
	shell, flag := defaultShell() // bash -c / cmd /C / powershell
	cmd := exec.CommandContext(ctx, shell, flag, command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
```

Windows 使用 `cmd.exe /C` 或 PowerShell；Unix 使用 `sh -c`。

- [ ] **Step 4: 测试（mock 脚本）**

测试 `ResolveCommand` 平台选择；测试 `Run` 执行 `echo installed` 临时脚本。

- [ ] **Step 5: Commit**

```bash
git add internal/installer/ internal/pkg/manifest/
git commit -m "feat: add CLI installer manifest and runner"
```

---

### Task 4: state 安装状态

**Files:**
- Create: `internal/state/types.go`, `internal/state/store.go`
- Test: `internal/state/store_test.go`

- [ ] **Step 1: 定义类型**

`internal/state/types.go`:

```go
package state

import "time"

type File struct {
	Bundles []BundleRecord `json:"bundles"`
}

type BundleRecord struct {
	Name           string          `json:"name"`
	Kind           string          `json:"kind"` // bundle | cli
	Version        string          `json:"version"`
	Scope          string          `json:"scope"`
	Ref            string          `json:"ref"`
	InstalledAt    time.Time       `json:"installed_at"`
	IDEs           []string        `json:"ides,omitempty"`
	Resources      BundleResources `json:"resources,omitempty"`
	InstallCommand string          `json:"install_command,omitempty"` // cli 类型记录已执行命令
}

type BundleResources struct {
	Skills []string `json:"skills"`
	Rules  []string `json:"rules"`
	MCP    []string `json:"mcp"`
}
```

- [ ] **Step 2: 实现 store（Load/Save/Upsert/Remove/List）**

`internal/state/store.go` 使用 `encoding/json`，路径来自 `platform.WorkStatePath(scope)`。

- [ ] **Step 3: 测试 round-trip**

```bash
go test ./internal/state/... -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/state/
git commit -m "feat: persist installed bundle state"
```

---

### Task 5: MCP merge 工具

**Files:**
- Create: `internal/adapter/mcp_merge.go`
- Test: `internal/adapter/mcp_merge_test.go`

- [ ] **Step 1: 写失败测试**

测试：空文件 merge 一个 server；已有 server 同 id 覆盖；unmerge 删除指定 id。

- [ ] **Step 2: 实现 merge（Cursor mcpServers 格式）**

```go
// MCPConfig 兼容 Cursor: { "mcpServers": { "id": { ... } } }
func MergeMCPServers(existing []byte, serverID string, serverJSON json.RawMessage) ([]byte, error)
func RemoveMCPServer(existing []byte, serverID string) ([]byte, error)
```

Qoder/Claude adapter 在写入前将各自格式转为统一 `mcpServers` map 再转回（MVP 可先统一用 Cursor 兼容 JSON 结构，adapter 层做字段映射）。

- [ ] **Step 3: 运行测试**

```bash
go test ./internal/adapter/... -run MCP -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/mcp_merge.go internal/adapter/mcp_merge_test.go
git commit -m "feat: merge and unmerge MCP server configs"
```

---

### Task 6: Adapter 接口与 Cursor 适配器

**Files:**
- Create: `internal/adapter/adapter.go`, `internal/adapter/cursor/adapter.go`, `internal/adapter/cursor/detect.go`
- Test: `internal/adapter/cursor/adapter_test.go`

- [ ] **Step 1: 定义接口**

`internal/adapter/adapter.go`:

```go
package adapter

import (
	"context"

	"github.com/huangchao257/work-cli/internal/bundle"
	"github.com/huangchao257/work-cli/internal/state"
)

type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

type Adapter interface {
	Name() string
	Detect() bool
	InstallSkill(ctx context.Context, bundleRoot string, skill bundle.SkillResource, scope Scope) (string, error)
	InstallRule(ctx context.Context, bundleRoot string, rule bundle.RuleResource, scope Scope) (string, error)
	InstallMCP(ctx context.Context, bundleRoot string, mcp bundle.MCPResource, scope Scope) (string, error)
	Uninstall(ctx context.Context, rec state.BundleRecord, scope Scope) error
}

func All() []Adapter {
	return []Adapter{
		cursor.New(),
		qoder.New(),
		claude.New(),
	}
}
```

（`qoder`/`claude` 在 Task 7 实现，此步可用 stub 或同 Task 7 一并完成。）

- [ ] **Step 2: Cursor Detect**

`Detect()`：检查 `~/.cursor` 目录或常见 Cursor 应用路径是否存在（`runtime.GOOS` 分支）。MVP 可简化为：用户目录下 `.cursor` 存在 **或** `--ide cursor` 强制时视为可安装。

- [ ] **Step 3: Cursor InstallSkill**

使用 `io.Copy` + `filepath.Walk` 将 `bundleRoot/skill.Source` 复制到 `platform.SkillDir(IDECursor, scope, skill.ID)`。

- [ ] **Step 4: Cursor InstallRule**

读取 source 文件，按 Cursor rules 格式写 front matter：

```markdown
---
description: {id}
alwaysApply: true   # apply=always 时
globs: **/*.go      # apply=files 时
---

{原文件内容}
```

写入 `RuleDir/{id}.mdc`（Cursor 规则扩展名）。

- [ ] **Step 5: Cursor InstallMCP**

读取 source JSON，调用 `MergeMCPServers` 写入 `MCPConfigPath`。

- [ ] **Step 6: 测试（使用 t.TempDir mock HOME）**

```bash
go test ./internal/adapter/cursor/... -v
```

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/
git commit -m "feat: add adapter interface and Cursor adapter"
```

---

### Task 7: Qoder 与 Claude Code 适配器

**Files:**
- Create: `internal/adapter/qoder/adapter.go`, `internal/adapter/claude/adapter.go`
- Test: `internal/adapter/qoder/adapter_test.go`, `internal/adapter/claude/adapter_test.go`

- [ ] **Step 1: Qoder adapter**

- Skill：复制到 `~/.qoder/skills/{id}/` 或 `.qoder/skills/{id}/`
- Rule：写入 `.qoder/rules/{id}.md`，元数据注释或 sidecar 记录 apply 类型（按 Qoder 文档）
- MCP：merge 到 `~/.qoder/mcp.json`（格式与 Cursor 类似，字段按 Qoder 文档调整）
- Detect：检查 `~/.qoder` 或应用安装痕迹

- [ ] **Step 2: Claude adapter**

- Skill：`~/.claude/skills/{id}/`
- Rule：project scope 写入 `CLAUDE.md` 追加段落或 `.claude/rules/{id}.md`；user scope 写入 `~/.claude/`
- MCP：merge 到 Claude MCP 配置路径
- Detect：检查 `~/.claude` 或 `claude` 命令是否存在

- [ ] **Step 3: 各写至少 1 个 install skill 集成测试**

```bash
go test ./internal/adapter/qoder/... ./internal/adapter/claude/... -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/qoder/ internal/adapter/claude/
git commit -m "feat: add Qoder and Claude Code adapters"
```

---

### Task 8: source 解析（local / git / registry）

**Files:**
- Create: `internal/source/ref.go`, `internal/source/local.go`, `internal/source/git.go`, `internal/source/registry.go`
- Test: `internal/source/ref_test.go`, `internal/source/local_test.go`

- [ ] **Step 1: ref 解析**

`internal/source/ref.go`:

```go
type Kind int
const (
	KindRegistry Kind = iota
	KindGit
	KindLocal
)

type Ref struct {
	Kind     Kind
	Name     string   // registry name 或 local path
	GitURL   string
	GitRef   string
}

func ParseRef(raw string) (Ref, error)
```

规则：
- `./` 或 `/` 开头 → `KindLocal`
- `git:` 前缀 → `KindGit`，解析 `git:host/org/repo@ref`
- 其他 → `KindRegistry`

- [ ] **Step 2: local source**

`ResolveLocal(path) (bundleDir string, err error)` — 验证 `bundle.yaml` 存在。

- [ ] **Step 3: git source**

`ResolveGit(url, ref, cacheDir) (bundleDir string, err error)`:

```go
cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, url, dest)
```

ref 无 `@` 时 clone 默认分支；缓存目录 `~/.work/cache/git/{hash}/`。

- [ ] **Step 4: registry source**

`ResolveRegistry(name, cfg) (bundleDir string, err error)`:

1. `GET {registry.url}/bundles/{name}/latest`
2. 下载 `download_url` zip 到 cache
3. 校验 `sha256:` checksum
4. 解压到 `~/.work/cache/registry/{name}/{version}/`

MVP registry 测试用 `httptest.Server` mock。

- [ ] **Step 5: 测试**

```bash
go test ./internal/source/... -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/source/
git commit -m "feat: resolve bundles from local, git, and registry"
```

---

### Task 9: engine 编排

**Files:**
- Create: `internal/engine/install.go`, `internal/engine/uninstall.go`, `internal/engine/update.go`, `internal/engine/options.go`
- Test: `internal/engine/install_test.go`

- [ ] **Step 1: 定义 Options**

```go
type Options struct {
	Scope   string
	IDEs    []string   // 空 = 全部已检测
	DryRun  bool
	Ref     source.Ref
}
```

- [ ] **Step 2: 统一 Install 入口**

`internal/engine/install.go` `Install(ctx, opts) (Result, error)`:

1. `resolvePackage(opts.Ref)` → pkgDir
2. `manifest.DetectKind(pkgDir)` → bundle | cli
3. **cli 分支**（`engine/cli.go`）：解析 installer → 检查 env → dry-run 打印命令 → `installer.Run(install)` → 可选 verify → `state.Upsert(kind=cli)`
4. **bundle 分支**（`engine/bundle.go`）：原有 Adapter 流程 → `state.Upsert(kind=bundle)`
5. cli 类型忽略 `--scope project`（警告后继续）

- [ ] **Step 3: Uninstall**

按 name+scope 查 state → `kind=cli` 执行 uninstall.run（或提示手动）→ `kind=bundle` 走 adapter → state.Remove

- [ ] **Step 4: Update**

List 匹配记录 → 查新版本 → cli 执行 update.run 或重装 install → bundle 先 uninstall 再 install

本地 ref：比较 version 字符串；registry：HTTP latest；git：fetch 后比较 tag/commit。

- [ ] **Step 5: 集成测试（temp HOME + examples/dev-kit）**

在 Task 10 创建 example 后补全；此步可先写 stub 测试。

- [ ] **Step 6: Commit**

```bash
git add internal/engine/
git commit -m "feat: bundle install uninstall update engine"
```

---

### Task 10: CLI 子命令

**Files:**
- Create: `internal/cli/install.go`, `internal/cli/list.go`, `internal/cli/uninstall.go`, `internal/cli/update.go`
- Create: `internal/output/human.go`, `internal/output/json.go`

- [ ] **Step 1: install 命令**

```go
var installCmd = &cobra.Command{
	Use:   "install <ref>",
	Short: "安装资源套装",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ref, err := source.ParseRef(args[0])
		if err != nil { return err }
		res, err := engine.Install(cmd.Context(), engine.Options{
			Scope: scope, IDEs: splitIDE(ide), DryRun: dryRun, Ref: ref,
		})
		if asJSON { return output.PrintJSON(os.Stdout, res) }
		return output.PrintHuman(os.Stdout, res)
	},
}
```

- [ ] **Step 2: list / uninstall / update 命令**

`list`：读 state，支持 `--ide` 过滤。  
`uninstall`：参数 bundle name，尊重 `--scope`。  
`update`：可选 bundle name，空则更新全部。

- [ ] **Step 3: 中文友好输出**

`internal/output/human.go` 实现 spec §11.2 文案格式。

- [ ] **Step 4: 注册子命令到 root**

```go
rootCmd.AddCommand(installCmd, listCmd, uninstallCmd, updateCmd)
```

- [ ] **Step 5: 手动验证**

```bash
go build -o bin/work ./cmd/work
./bin/work install ./examples/dev-kit --dry-run
```

- [ ] **Step 6: Commit**

```bash
git add internal/cli/ internal/output/
git commit -m "feat: add resource module CLI commands"
```

---

### Task 11: 示例 bundle 与端到端测试

**Files:**
- Create: `examples/dev-kit/bundle.yaml`
- Create: `examples/dev-kit/skills/code-review/SKILL.md`
- Create: `examples/dev-kit/rules/go-style.md`
- Create: `examples/dev-kit/mcp/mysql.json`
- Create: `internal/engine/e2e_test.go`

- [ ] **Step 1: 创建 openspec cli 示例**

`examples/openspec/installer.yaml`:

```yaml
type: cli
name: openspec
version: 1.0.0
description: OpenSpec CLI（@fission-ai/openspec）
install:
  run: npm install -g @fission-ai/openspec@latest
verify:
  command: [openspec, --version]
uninstall:
  run: npm uninstall -g @fission-ai/openspec
update:
  run: npm install -g @fission-ai/openspec@latest
```

`examples/openspec-mock/`（仅测试用）：含 mock `install.sh`，CI e2e 不依赖 npm 网络。

- [ ] **Step 2: 创建 dev-kit**

`examples/dev-kit/bundle.yaml`（无 required env 或 env required:false 便于测试）:

```yaml
name: dev-kit
version: 1.0.0
description: 公司通用 AI 技能包（示例）
resources:
  skills:
    - id: code-review
      source: ./skills/code-review
  rules:
    - id: go-style
      source: ./rules/go-style.md
      apply: always
targets: [cursor]
```

`examples/dev-kit/skills/code-review/SKILL.md`:

```markdown
---
name: code-review
description: 代码审查技能示例
---
# Code Review
审查代码变更并给出建议。
```

- [ ] **Step 3: e2e 测试**

`internal/engine/e2e_test.go`：

**bundle 流程：**
1. `Install(./examples/dev-kit)` → skill 文件存在 → `Uninstall dev-kit`

**cli 流程（CI 用 mock，不跑真实 npm）：**
1. `Install(./examples/openspec-mock)` → 标记文件存在 → `List --kind cli`
2. `Uninstall` → 标记文件删除

**手工验证：** `work install openspec --dry-run` 应显示 `npm install -g @fission-ai/openspec@latest`

```bash
go test ./internal/engine/... -run E2E -v
```

- [ ] **Step 3: Commit**

```bash
git add examples/ internal/engine/e2e_test.go
git commit -m "test: add dev-kit example and e2e install flow"
```

---

### Task 12: CI 与 README

**Files:**
- Create: `.github/workflows/ci.yml`, `README.md`

- [ ] **Step 1: CI 矩阵**

`.github/workflows/ci.yml`:

```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26.x'
      - run: go test ./...
      - run: go build -o work ./cmd/work
```

- [ ] **Step 2: README（面向全体员工）**

`README.md` 包含：

1. work-cli 是什么
2. 安装方式（下载二进制 / 脚本）
3. 快速开始：`work install ./examples/dev-kit`、`work install openspec`
4. 常用命令表（含 bundle 与 cli 两类）
5. 三 IDE 支持说明
6. 环境变量配置说明
7. 故障排查（未检测到 IDE、缺 env）

- [ ] **Step 3: 全量测试**

```bash
go test ./... -v
go build -o bin/work ./cmd/work
```

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml README.md
git commit -m "docs: add README and CI workflow"
```

---

## Spec 覆盖自检

| Spec 章节 | 对应 Task |
|-----------|-----------|
| §3 P0 命令 | Task 10 |
| §4 manifest (bundle) | Task 3 |
| §4.3 CLI installer | Task 3b |
| §5 IDE 适配器 | Task 6, 7 |
| §6 核心流程 | Task 9 |
| §7 状态与配置 | Task 4 |
| §8 套装来源 | Task 8 |
| §9 跨平台 | Task 2 |
| §11 错误与输出 | Task 10 (output) |
| §12 测试 | Task 2–11, 12 |
| §14 认证预留 | 不在 MVP；`source` 包 HTTP client 留 `Authenticator` 接口扩展点 |

## 实现顺序建议

```
Task 1 → 2 → 3 → 3b → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12
```

Task 6/7 可并行；Task 11 依赖 9+10+examples。

---

## 风险与决策记录

1. **IDE 配置格式可能随版本变化** — adapter 测试锁定样例 JSON；文档注明 IDE 最低版本。
2. **Claude rules 路径** — MVP 使用 `.claude/rules/{id}.md`；若官方变更，仅改 claude adapter。
3. **Registry MVP 无认证** — `config.yaml` 中 `registry.url` 默认空，纯本地/git 可先用；registry 需 IT 部署后配置。
4. **git 依赖** — `git` 需在 PATH；缺失时错误提示「请先安装 Git」。
5. **CLI 委托安装安全** — 仅执行 Registry/Git/本地 manifest 中预置命令，不接受任意 shell 字符串；dry-run 必须展示将执行命令。
