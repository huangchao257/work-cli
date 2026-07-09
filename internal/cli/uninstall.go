package cli

import (
	"fmt"

	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	uninstallAll bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <name...>",
	Short: "卸载已安装项",
	Long: `卸载指定资源套装、hooks 套装或外部 CLI，并从已安装记录中移除。

资源文件和 hooks 配置会被清理；外部 CLI 会尝试执行卸载命令。
支持一次卸载多个资源，或使用 --all 卸载全部。

示例:
  work uninstall dev-kit
  work uninstall company-hooks openspec
  work uninstall --all
  work uninstall --all --kind cli
  work uninstall dev-kit company-hooks --scope project`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// --all 标志：卸载全部已安装资源
		if uninstallAll {
			br, err := engine.UninstallAll(signalContext(), scope, kind, dryRun)
			if err != nil {
				return err
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), br)
			}
			return output.PrintHumanBatch(cmd.OutOrStdout(), br)
		}

		if len(args) == 0 && !uninstallAll {
			_ = cmd.Help()
			return fmt.Errorf("至少需要指定一个卸载名称，或使用 --all")
		}

		// 快速路径：单个卸载保持原有行为
		if len(args) == 1 {
			res, err := engine.Uninstall(signalContext(), args[0], scope, dryRun)
			if err != nil {
				return err
			}
			if asJSON {
				return output.PrintInstallJSON(cmd.OutOrStdout(), res)
			}
			return output.PrintHumanUninstall(cmd.OutOrStdout(), res)
		}

		// 批量卸载
		br, err := engine.UninstallBatch(signalContext(), args, scope, dryRun)
		if err != nil {
			return err
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), br)
		}
		return output.PrintHumanBatch(cmd.OutOrStdout(), br)
	},
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallAll, "all", false, "卸载所有已安装的资源")
	rootCmd.AddCommand(uninstallCmd)
}
