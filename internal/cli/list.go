package cli

import (
	"github.com/spf13/cobra"
	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/output"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出已安装的资源套装和 CLI",
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
