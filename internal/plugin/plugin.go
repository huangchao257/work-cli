// Package plugin 提供 work CLI 的插件系统骨架。
// 通过在 ~/.work/plugins/ 目录下放置包含 plugin.yaml 的子目录，
// 可以注册自定义命令为 work 子命令。
package plugin

import "errors"

// ErrNotFound 表示指定名称的插件未找到。
var ErrNotFound = errors.New("插件未找到")

// Plugin 是插件必须实现的接口。
// 第三方通过实现该接口扩展 work CLI 的功能。
type Plugin interface {
	// Name 返回插件的唯一标识名称，同时作为子命令名。
	Name() string

	// Description 返回插件的简要描述，用于 help 输出。
	Description() string

	// Run 执行插件逻辑。args 是传递给插件的命令行参数（不含子命令名）。
	Run(args []string) error
}

// Manifest 描述插件的元数据与启动方式。
// 对应 plugin.yaml 文件内容。
type Manifest struct {
	// Name 插件唯一名称（必须与目录名一致）。
	Name string `yaml:"name" json:"name"`

	// Version 插件自身的版本号。
	Version string `yaml:"version" json:"version"`

	// Description 简要描述。
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Command 插件可执行文件路径（绝对路径或通过 PATH 查找）。
	Command string `yaml:"command" json:"command"`
}
