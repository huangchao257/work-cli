// Package installer 解析 installer.yaml，并执行外部 CLI 工具的安装与校验命令。

package installer

import "github.com/huangchao257/work-cli/internal/bundle"

type Manifest struct {
	Type        string         `yaml:"type"`
	Name        string         `yaml:"name"`
	Version     string         `yaml:"version"`
	Description string         `yaml:"description"`
	Env         []bundle.EnvVar `yaml:"env"`
	Install     CommandSpec    `yaml:"install"`
	Verify      *VerifySpec    `yaml:"verify"`
	Uninstall   *CommandSpec   `yaml:"uninstall"`
	Update      *CommandSpec   `yaml:"update"`
}

type CommandSpec struct {
	Run       string                       `yaml:"run"`
	Platforms map[string]PlatformCommand   `yaml:"platforms"`
}

type PlatformCommand struct {
	Run string `yaml:"run"`
}

type VerifySpec struct {
	Command []string `yaml:"command"`
}
