package cli

import (
	"os"
	"time"

	"github.com/huangchao257/work-cli/internal/graph"
	"github.com/spf13/cobra"
)

var (
	graphPath     string
	graphDebounce time.Duration
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "代码知识图谱与 AGENTS.md（对标 codegraph init）",
	Long: `管理项目 CodeGraph 知识图谱，并自动维护各目录 AGENTS.md。

work graph init    初始化索引 + 首次生成 AGENTS.md
work graph sync    手动同步索引并更新 AGENTS.md
work graph watch   启动文件监控守护进程，源码变更后自动更新 AGENTS.md
work graph status  查看图谱状态`,
	Example: `  work graph init              初始化图谱
  work graph watch             启动文件监控自动同步
  work graph sync              手动同步
  work graph status            查看状态
  work install codegraph-stack   一键安装全部能力`,
}

var graphInitCmd = &cobra.Command{
	Use:   "init",
	Short: "初始化知识图谱并首次生成 AGENTS.md",
	Long:  "等同 codegraph init，并首次生成 AGENTS.md。运行 work graph watch 后可自动更新。",
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
	Long:  "手动执行 codegraph sync 并重新生成各目录 AGENTS.md。已开启 watch 时通常无需手动执行。",
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
	Short: "查看图谱与 AGENTS 文件监控状态",
	Long:  "显示 CodeGraph 索引状态、generate-agents.sh 安装状态。",
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

var graphWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "启动文件监控，源码变更时自动更新 AGENTS.md",
	Long: `不依赖 IDE hooks 的文件系统监控守护进程。
源码文件变更后防抖等待（默认 2s），然后自动执行 codegraph sync + 重新生成 AGENTS.md。

Ctrl+C 停止。`,
	Example: `  work graph watch
  work graph watch --path /path/to/project
  work graph watch --debounce 3s`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return graph.Watch(signalContext(), graph.WatchOptions{
			ProjectPath: graphPath,
			Debounce:    graphDebounce,
		})
	},
}

func init() {
	graphCmd.PersistentFlags().StringVar(&graphPath, "path", "", "项目根目录（默认当前目录）")
	graphCmd.AddCommand(graphInitCmd, graphSyncCmd, graphStatusCmd, graphWatchCmd)
	graphWatchCmd.Flags().DurationVar(&graphDebounce, "debounce", 2*time.Second, "防抖等待时间")
	rootCmd.AddCommand(graphCmd)
}
