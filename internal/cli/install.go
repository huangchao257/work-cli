package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/source"
)

var installCmd = &cobra.Command{
	Use:   "install <ref>",
	Short: "安装资源套装或外部 CLI",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ref, err := source.ParseRef(args[0])
		if err != nil {
			return err
		}
		res, err := engine.Install(context.Background(), engine.Options{
			Scope:  scope,
			IDEs:   SplitIDEs(ide),
			DryRun: dryRun,
			Ref:    ref,
		})
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
	rootCmd.AddCommand(installCmd)
}
