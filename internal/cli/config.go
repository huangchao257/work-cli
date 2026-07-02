package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/config"
	"github.com/huangchao257/work-cli/internal/output"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
	Long: `读写 ~/.work/config.yaml 配置文件。

键为点分路径，如 registry.url、cache.dir、self_update.enabled、
self_update.check_interval、telemetry.enabled、telemetry.url、
telemetry.events、telemetry.redact 等。`,
	Example: `  # 打印配置文件绝对路径
  work config path

  # 列出全部配置（JSON 输出便于脚本解析）
  work config list --json

  # 取值（直接打印，便于脚本）
  work config get registry.url

  # 设值（自动推断类型：布尔/整数/列表/字符串）
  work config set registry.url https://registry.internal.example.com
  work config set self_update.enabled true
  work config set telemetry.events shell,mcp,file_read

  # 删除键
  work config unset registry.url`,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "打印配置文件绝对路径",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := config.Path()
		if err != nil {
			return exitErr(1, err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), p)
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出全部配置键值",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := config.List()
		if err != nil {
			return configErr(err)
		}
		w := cmd.OutOrStdout()
		if asJSON {
			return output.PrintJSON(w, m)
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "%s: %s\n", k, m[k])
		}
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "取配置值（直接打印，便于脚本）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, ok, err := config.Get(args[0])
		if err != nil {
			return configErr(err)
		}
		// 键或文件不存在时输出空行，退出码 0，便于脚本串联。
		fmt.Fprintln(cmd.OutOrStdout(), v)
		_ = ok
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "设置配置值",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Set(args[0], args[1], dryRun); err != nil {
			return configErr(err)
		}
		w := cmd.OutOrStdout()
		label := "已设置"
		if dryRun {
			label = "预览"
		}
		if asJSON {
			return output.PrintJSON(w, map[string]any{"key": args[0], "value": args[1], "ok": true, "dry_run": dryRun})
		}
		fmt.Fprintf(w, "%s %s = %s\n", label, args[0], args[1])
		return nil
	},
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "删除配置键",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Unset(args[0], dryRun); err != nil {
			return configErr(err)
		}
		w := cmd.OutOrStdout()
		label := "已删除"
		if dryRun {
			label = "预览"
		}
		if asJSON {
			return output.PrintJSON(w, map[string]any{"key": args[0], "ok": true, "dry_run": dryRun})
		}
		fmt.Fprintf(w, "%s %s\n", label, args[0])
		return nil
	},
}

// configErr 将 config 包错误映射为退出码：usage.Error→2，其余→1。
func configErr(err error) error {
	return ExitUsageErr(err)
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configPathCmd, configListCmd, configGetCmd, configSetCmd, configUnsetCmd)
}
