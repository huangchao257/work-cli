// Package installer 解析 installer.yaml，并执行外部 CLI 工具的安装与校验命令。

package installer

// EnvVar 描述一个环境变量及其是否必需。
// 与 bundle.EnvVar / hooks.EnvVar 结构一致，按包自包含原则独立定义。
type EnvVar struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type Manifest struct {
	Type        string       `yaml:"type"`
	Name        string       `yaml:"name"`
	Version     string       `yaml:"version"`
	Description string       `yaml:"description"`
	Env         []EnvVar     `yaml:"env"`
	Install     CommandSpec  `yaml:"install"`
	Verify      *VerifySpec  `yaml:"verify"`
	Uninstall   *CommandSpec `yaml:"uninstall"`
	Update      *CommandSpec `yaml:"update"`
}

type CommandSpec struct {
	Run       string                     `yaml:"run"`
	Platforms map[string]PlatformCommand `yaml:"platforms"`
}

type PlatformCommand struct {
	Run string `yaml:"run"`
}

type VerifySpec struct {
	Command []string `yaml:"command"`
}

// RequiredEnvNames 返回标记为 required 的环境变量名称列表。
// engine/cli_install.go 安装 CLI 时用于检查缺失的环境变量。
func RequiredEnvNames(env []EnvVar) []string {
	var names []string
	for _, e := range env {
		if e.Required {
			names = append(names, e.Name)
		}
	}
	return names
}
