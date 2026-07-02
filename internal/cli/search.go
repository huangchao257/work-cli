package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/search"
	"github.com/huangchao257/work-cli/internal/source"
)

var searchRemote bool

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "搜索可安装的资源",
	Long: `列出可安装的资源（内置 catalog + 可选 Registry 远程清单），与 work list（已安装）互补。

不带参数时列出全部内置资源；提供 query 时按子串（不区分大小写）模糊匹配 name/description。
加 --remote 时同时查询 Registry 远程清单（需在 ~/.work/config.yaml 配置 registry.url）。`,
	Example: `  work search
  work search dev
  work search codegraph --remote
  work search --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := ""
		if len(args) == 1 {
			query = args[0]
		}

		registryURL := ""
		if searchRemote {
			cfg, err := source.LoadUserConfig()
			if err != nil {
				// 配置读取失败不阻断搜索，按未配置处理并降级为仅本地。
				fmt.Fprintf(cmd.ErrOrStderr(), "⚠ 读取配置失败: %v\n", err)
			} else {
				registryURL = cfg.Registry.URL
			}
		}

		res, err := search.Run(search.Options{
			Query:       query,
			Remote:      searchRemote,
			RegistryURL: registryURL,
		})
		if err != nil {
			return exitErr(1, err)
		}

		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), res)
		}

		// human 模式：warning 输出到 stdout 顶部
		for _, w := range res.Warnings {
			fmt.Fprintf(cmd.OutOrStdout(), "⚠ %s\n", w)
		}

		if len(res.Items) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "未找到匹配资源")
			return nil
		}
		for _, it := range res.Items {
			fmt.Fprintf(cmd.OutOrStdout(), "- %s v%s [%s] (%s) — %s\n",
				it.Name, it.Version, it.Type, it.Source, it.Description)
		}
		return nil
	},
}

func init() {
	searchCmd.Flags().BoolVar(&searchRemote, "remote", false, "同时查询 Registry 远程清单")
	rootCmd.AddCommand(searchCmd)
}
