package cli

import (
	"context"

	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "卸载已安装项",
	Long: `卸载指定资源套装、hooks 套装或外部 CLI，并从已安装记录中移除。

资源文件和 hooks 配置会被清理；外部 CLI 会尝试执行卸载命令。

示例:
  work uninstall dev-kit
  work uninstall company-hooks
  work uninstall openspec --scope project`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := engine.Uninstall(context.Background(), args[0], scope, dryRun)
		if err != nil {
			return err
		}
		if asJSON {
			return output.PrintInstallJSON(cmd.OutOrStdout(), res)
		}
		return output.PrintHuman(cmd.OutOrStdout(), res)
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
