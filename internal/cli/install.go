package cli

import (
	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/source"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "安装已配置的资源套装、hooks 套装或外部 CLI",
	Long: `安装公司内部已配置的资源，不支持手动指定本地路径或 git 引用。

可用资源名称见内置目录，或在 ~/.work/config.yaml 配置 registry.url 后从 Registry 拉取。

示例:
  work install dev-kit
  work install codegraph-stack
  work install openspec`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ref, err := source.ParseInstallName(args[0])
		if err != nil {
			return err
		}
		if err := source.ValidateInstallName(ref.Name); err != nil {
			return err
		}
		res, err := engine.Install(signalContext(), engine.Options{
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
