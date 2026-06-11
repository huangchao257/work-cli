package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/output"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "卸载已安装项",
	Args:  cobra.ExactArgs(1),
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
