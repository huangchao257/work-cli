package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/plugin"
)

func init() {
	rootCmd.AddCommand(pluginCmd)
}

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "管理 CLI 插件",
	Long: strings.TrimSpace(`
插件命令用于管理 work CLI 的第三方扩展。

插件存放在 ~/.work/plugins/<名称>/plugin.yaml，通过 work plugin list 列出，
work plugin run <名称> -- <参数> 调用。`),
	Example: strings.TrimSpace(`
  work plugin list                  列出所有已安装的插件
  work plugin run my-plugin -- -h   调用 my-plugin 并传递 -h 参数`),
}

var pluginListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "列出所有已发现的插件",
	Long:    "扫描 ~/.work/plugins/ 目录，输出所有有效插件的名称与描述。",
	RunE: func(cmd *cobra.Command, args []string) error {
		plugins, err := plugin.Discover()
		if err != nil {
			return fmt.Errorf("扫描插件失败: %w", err)
		}

		if asJSON {
			type item struct {
				Name        string `json:"name"`
				Version     string `json:"version"`
				Description string `json:"description"`
			}
			items := make([]item, len(plugins))
			for i, p := range plugins {
				items[i] = item{Name: p.Name, Version: p.Version, Description: p.Description}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(items)
		}

		if len(plugins) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "未发现任何插件")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "发现 %d 个插件:\n", len(plugins))
		for _, p := range plugins {
			desc := p.Description
			if desc == "" {
				desc = "(无描述)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %-20s  %s  %s\n", p.Name, p.Version, desc)
		}
		return nil
	},
}

var pluginRunCmd = &cobra.Command{
	Use:                "run <插件名称> -- <参数>",
	Short:              "调用指定插件",
	Long:               "通过 plugin.yaml 中声明的 command 启动插件进程，传递 -- 之后的参数。",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// DisableFlagParsing=true 后所有参数都在 args 中，手动解析
		if len(args) == 0 {
			return fmt.Errorf("请指定插件名称")
		}
		name := args[0]

		// 跳过 -- 分隔符，收集插件参数
		var pluginArgs []string
		sepFound := false
		for _, a := range args[1:] {
			if a == "--" && !sepFound {
				sepFound = true
				continue
			}
			if sepFound {
				pluginArgs = append(pluginArgs, a)
			}
		}

		m, err := plugin.Find(name)
		if err != nil {
			return fmt.Errorf("查找插件 %q 失败: %w", name, err)
		}
		if m.Command == "" {
			return fmt.Errorf("插件 %q 未声明 command 字段", name)
		}

		execCmd := exec.Command(m.Command, pluginArgs...)
		execCmd.Stdin = os.Stdin
		execCmd.Stdout = cmd.OutOrStdout()
		execCmd.Stderr = cmd.ErrOrStderr()
		return execCmd.Run()
	},
}

func init() {
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginRunCmd)
}
