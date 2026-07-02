// Package scaffold 提供脚手架生成能力，为套装作者生成符合 manifest 规范的骨架目录。
//
// 支持三种类型：bundle（资源套装）、cli（外部 CLI 安装清单）、hooks（IDE 事件上报 hooks）。
// 生成内容字段带中文注释，name/version 自动填入，description 留占位。
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/huangchao257/work-cli/internal/usage"
)

// Type 表示脚手架类型。
type Type string

const (
	TypeBundle Type = "bundle"
	TypeCLI    Type = "cli"
	TypeHooks  Type = "hooks"
)

// 合法类型集合（用于 ParseType 校验）。
var validTypes = map[Type]bool{
	TypeBundle: true,
	TypeCLI:    true,
	TypeHooks:  true,
}

// ErrUnknownType 表示未知的脚手架类型。
var ErrUnknownType = usage.New("未知的脚手架类型，可选：bundle、cli、hooks")

// IsUsageError 判断 err 是否为用法错误。
var IsUsageError = usage.Is

// ParseType 将字符串解析为 Type，非法值返回错误。
func ParseType(s string) (Type, error) {
	t := Type(s)
	if !validTypes[t] {
		return "", ErrUnknownType
	}
	return t, nil
}

// Options 是脚手架生成的输入参数。
type Options struct {
	Type   Type   // 脚手架类型
	Name   string // 套装/CLI 名称，写入 manifest 的 name 字段
	Dir    string // 输出目录，默认 ./<name>
	DryRun bool   // 仅预览，不写盘
}

// fileSpec 描述一个待生成的文件：相对路径、内容、是否需要可执行权限。
type fileSpec struct {
	path       string
	content    string
	executable bool
}

// Run 按类型生成骨架文件，返回写入（或预览将写入）的文件路径列表（绝对路径）。
// 目标目录已存在且非空时返回用法错误。
func Run(opts Options) ([]string, error) {
	if opts.Name == "" {
		return nil, usage.New("name 不能为空")
	}
	dir := opts.Dir
	if dir == "" {
		dir = "./" + opts.Name
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("解析目录路径失败: %w", err)
	}

	// 目标目录已存在且非空 → 用法错误。
	if entries, err := os.ReadDir(absDir); err == nil && len(entries) > 0 {
		return nil, usage.Newf("目标目录已存在且非空: %s", absDir)
	}

	specs, err := buildSpecs(opts.Type, opts.Name)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(specs))
	for _, sp := range specs {
		full := filepath.Join(absDir, sp.path)
		files = append(files, full)
		if opts.DryRun {
			continue
		}
		if err := writeFile(full, sp.content, sp.executable); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// writeFile 写入单个文件，按需创建父目录并设置可执行权限。
func writeFile(path, content string, executable bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建目录失败 %s: %w", filepath.Dir(path), err)
	}
	mode := os.FileMode(0o644)
	if executable {
		mode = 0o755
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return fmt.Errorf("写入文件失败 %s: %w", path, err)
	}
	// os.WriteFile 受 umask 影响，显式 chmod 保证可执行位。
	if executable {
		if err := os.Chmod(path, 0o755); err != nil {
			return fmt.Errorf("设置可执行权限失败 %s: %w", path, err)
		}
	}
	return nil
}

// buildSpecs 按类型构造文件清单（相对路径 + 内容）。
func buildSpecs(t Type, name string) ([]fileSpec, error) {
	switch t {
	case TypeBundle:
		return bundleSpecs(name), nil
	case TypeCLI:
		return cliSpecs(name), nil
	case TypeHooks:
		return hooksSpecs(name), nil
	default:
		return nil, ErrUnknownType
	}
}

// bundleSpecs 生成 bundle 类型骨架。
func bundleSpecs(name string) []fileSpec {
	manifest := fmt.Sprintf(`# bundle.yaml — 资源套装清单（Skills / MCP / Rules）
# 字段带中文注释；可选段可按需删除。安装：work install %[1]s
name: %[1]s                       # 套装唯一标识，用于 uninstall/update
version: 0.1.0                    # 语义化版本
description: TODO 填写本套装用途  # 人类可读描述（可选）

# env:                              # 可选，安装前校验的环境变量
#   - name: MY_API_KEY
#     required: true

resources:
  skills:                          # 可选，Skill 资源列表（id + source 目录，目录内含 SKILL.md）
    - id: %[1]s
      source: ./skills/%[1]s
  # rules:                         # 可选，Rule 资源列表
  #   - id: go-style
  #     source: ./rules/go-style.md
  #     apply: always              # always | manual | files
  #     globs: ["**/*.go"]         # apply=files 时必填
  # mcp:                           # 可选，MCP 服务配置列表
  #   - id: internal-mysql
  #     source: ./mcp/mysql.json
  #     env:
  #       - API_KEY: ${MY_API_KEY} # 支持 ${VAR} 占位

# targets: [cursor]                # 可选；省略则三家 IDE 都装
`, name)

	skill := fmt.Sprintf(`---
name: %[1]s
description: TODO 填写本技能用途
---
# %[1]s

TODO 在此编写技能正文。
`, name)

	rules := `# 示例规则

- 这是一个示例规则文件，按需修改或删除。
- 规则以 Markdown 编写，` + "`apply`" + ` 决定生效方式（always/manual/files）。
`

	mcp := `{
  "mcpServers": {
    "sample-server": {
      "command": "npx",
      "args": ["-y", "example-mcp-server"],
      "env": {
        "API_KEY": "替换为实际密钥或使用 ${VAR} 占位"
      }
    }
  }
}
`

	return []fileSpec{
		{path: "bundle.yaml", content: manifest},
		{path: filepath.Join("skills", name, "SKILL.md"), content: skill},
		{path: filepath.Join("rules", "sample.md"), content: rules},
		{path: filepath.Join("mcp", "sample.json"), content: mcp},
	}
}

// cliSpecs 生成 cli 类型骨架。
func cliSpecs(name string) []fileSpec {
	manifest := fmt.Sprintf(`# installer.yaml — 外部 CLI 安装清单
# 字段带中文注释；可选段可按需删除。安装：work install %[1]s
type: cli                        # 固定为 cli
name: %[1]s                      # CLI 唯一标识，对应 work install <name>
version: 0.1.0                   # 安装包版本（非被安装 CLI 的运行时版本）
description: TODO 填写本 CLI 用途

install:
  run: TODO 替换为官方安装命令    # 单行 shell；或按平台覆盖见下
  # platforms:                   # 按 GOOS 覆盖，优先于全局 run
  #   darwin:
  #     run: brew install %[1]s
  #   linux:
  #     run: curl -fsSL https://example.com/install | bash
  #   windows:
  #     run: winget install %[1]s

verify:                          # 可选，安装后验证命令；失败仅警告不阻断
  command: [TODO, --version]

uninstall:                       # 可选；缺失则提示手动卸载，仅删状态记录
  run: TODO 替换为卸载命令

update:                          # 可选；缺失则回退为重新执行 install
  run: TODO 替换为更新命令

# env:                           # 可选，执行前检查的环境变量（规则同 bundle）
#   - name: MY_API_KEY
#     required: true
`, name)

	readme := fmt.Sprintf(`# %[1]s

TODO 描述本 CLI 的用途与安装方式。

## 安装

`+"```bash"+`
work install %[1]s
`+"```"+`

## 验证

`+"```bash"+`work list --kind cli`+"`"+` 查看安装状态。
`, name)

	return []fileSpec{
		{path: "installer.yaml", content: manifest},
		{path: "README.md", content: readme},
	}
}

// hooksSpecs 生成 hooks 类型骨架。
func hooksSpecs(name string) []fileSpec {
	manifest := fmt.Sprintf(`# hooks.yaml — IDE 事件上报 hooks 清单
# 字段带中文注释；可选段可按需删除。安装：work install %[1]s
type: hooks                      # 固定为 hooks
name: %[1]s                      # 套装唯一标识
version: 0.1.0                   # 语义化版本
description: TODO 填写本 hooks 用途

env:                             # 可选，安装前环境变量检查
  - name: WORK_TELEMETRY_URL
    description: 可选，覆盖 config 中的 telemetry.url
    required: false

telemetry:
  preset: audit                  # 默认核心审计集；或 all
  # events: [shell, mcp, file_read, file_edit, prompt]  # 覆盖 preset
  # redact: [prompt, file_content]                      # 额外脱敏字段

resources:
  hooks:
    - id: work-telemetry
      source: ./scripts/telemetry.sh

# targets: [cursor, qoder, claude]  # 可选；省略则三家都装
`, name)

	script := `#!/usr/bin/env bash
# 观察型 hook：采集 IDE 事件并上报，透传原始 stdin，始终 exit 0 不阻断 IDE。
set -euo pipefail
input=$(cat)
work hooks report \
  --ide "${WORK_HOOKS_IDE}" \
  --event "${WORK_HOOKS_EVENT}" \
  --hooks-kit "${WORK_HOOKS_KIT:-hooks}" <<< "$input" || true
printf '%s' "$input"
exit 0
`

	return []fileSpec{
		{path: "hooks.yaml", content: manifest},
		{path: filepath.Join("scripts", "telemetry.sh"), content: script, executable: true},
	}
}
