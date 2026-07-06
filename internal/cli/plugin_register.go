// Package cli 的插件自动注册。
// plugin_register.go 在 init 时扫描 ~/.work/plugins/ 并将其注册为 rootCmd 的隐藏子命令。
package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/plugin"
)

func init() {
	// 尝试发现并注册插件为隐藏子命令
	plugins, err := plugin.Discover()
	if err != nil {
		// 初始化阶段插件目录不可用属于正常情况（如首次运行），静默跳过
		return
	}
	for _, m := range plugins {
		p := m
		if p.Command == "" {
			continue
		}
		registerPluginCmd(p)
	}
}

// registerPluginCmd 将插件注册为 rootCmd 的隐藏子命令。
func registerPluginCmd(m plugin.Manifest) {
	cmd := &cobra.Command{
		Use:    m.Name,
		Short:  m.Description,
		Long:   fmt.Sprintf("插件: %s (版本 %s)\n\n%s", m.Name, m.Version, m.Description),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			execCmd := exec.Command(m.Command, args...)
			execCmd.Stdin = os.Stdin
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			return execCmd.Run()
		},
		// 禁用 cobra 的自动 help/usage 标志以透传参数
		DisableFlagParsing: true,
	}
	rootCmd.AddCommand(cmd)
}
