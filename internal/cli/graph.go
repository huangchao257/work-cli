package cli

import (
	"os"

	"github.com/huangchao257/work-cli/internal/graph"
	"github.com/spf13/cobra"
)

var graphPath string

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "代码知识图谱与 AGENTS.md（对标 codegraph init）",
	Long: `管理项目 CodeGraph 知识图谱，并自动维护各目录 AGENTS.md。

一条命令完成索引 + 自动同步配置 + 首次生成，保存代码后 AGENTS.md 会自动更新。`,
	Example: `  work graph init              初始化图谱并开启无感自动同步
  work graph sync              手动同步索引与 AGENTS.md
  work graph status            查看状态
  work install codegraph-stack   一键安装全部能力`,
}

var graphInitCmd = &cobra.Command{
	Use:   "init",
	Short: "初始化知识图谱并开启 AGENTS.md 自动同步",
	Long:  "等同 codegraph init -i，并自动配置 Cursor hooks、生成 AGENTS.md。",
	Example: `  work graph init
	  work graph init --path /path/to/project
	  work graph init --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return graph.Init(signalContext(), graph.Options{
			ProjectPath: graphPath,
			DryRun:      dryRun,
		})
	},
}

var graphSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "同步 CodeGraph 索引并更新 AGENTS.md",
	Long:  "手动执行 codegraph sync 并重新生成各目录 AGENTS.md。已开启自动同步时通常无需手动执行。",
	Example: `  work graph sync
	  work graph sync --path /path/to/project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return graph.Sync(signalContext(), graph.Options{
			ProjectPath: graphPath,
			DryRun:      dryRun,
			Quiet:       false,
		})
	},
}

var graphStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看图谱与 AGENTS 自动同步状态",
	Long:  "显示 CodeGraph 索引状态、hook 自动同步开关以及 AGENTS.md 示例数。",
	Example: `  work graph status
	  work graph status --json
	  work graph status --path /path/to/project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return graph.PrintStatus(signalContext(), graph.Options{
			ProjectPath: graphPath,
			Quiet:       asJSON,
		}, os.Stdout)
	},
}

func init() {
	graphCmd.PersistentFlags().StringVar(&graphPath, "path", "", "项目根目录（默认当前目录）")
	graphCmd.AddCommand(graphInitCmd, graphSyncCmd, graphStatusCmd)
	rootCmd.AddCommand(graphCmd)
}
