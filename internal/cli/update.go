package cli

import (
	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "更新本机已安装的资源",
	Long: `根据 ~/.work/installed.json（或项目 .work/installed.json）中的记录，重新拉取并安装最新版本。

不指定名称时更新当前 scope 下的全部已安装资源；指定名称时只更新该项。

示例:
  work update
  work update dev-kit
  work update openspec`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		results, err := engine.Update(signalContext(), name, scope, dryRun)
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
