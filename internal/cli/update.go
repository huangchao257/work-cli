package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/output"
)

var updateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "更新已安装项",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		results, err := engine.Update(context.Background(), name, scope, dryRun)
		if err != nil {
			return err
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), results)
		}
		for _, res := range results {
			if err := output.PrintHuman(cmd.OutOrStdout(), res); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
