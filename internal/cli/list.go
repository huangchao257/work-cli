package cli

import (
	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出已安装的资源套装和 CLI",
	Long: `显示当前 scope 下已安装的资源套装、hooks 套装或外部 CLI。

不带参数时列出全部已安装项；可结合 --scope、--kind 筛选。

示例:
  work list
  work list --kind bundle
  work list --scope project
  work list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := engine.List(scope, kind)
		if err != nil {
			return err
		}
		if asJSON {
			return output.PrintListJSON(cmd.OutOrStdout(), res)
		}
		return output.PrintHumanList(cmd.OutOrStdout(), res)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
