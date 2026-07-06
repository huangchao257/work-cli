package cli

import (
	"strings"

	"github.com/huangchao257/work-cli/internal/log"
	"github.com/spf13/cobra"
)

var (
	scope  string
	ide    string
	kind   string
	dryRun bool
	asJSON bool
)

var rootCmd = &cobra.Command{
	Use:   "work",
	Short: "公司统一 CLI 入口",
	Long: `work 是企业级命令行工具。

资源管理模块用于安装 AI IDE 资源套装（Skills / MCP / Rules），以及委托安装外部 CLI 工具。
Hooks 模块用于安装 AI IDE hooks 套装，并将 IDE 事件上报至本地队列与内网 Telemetry。
graph 模块提供代码知识图谱与 AGENTS.md 自动维护（对标 codegraph init -i）。

运行 work help 查看全部命令，或 work help <command> 查看单个命令说明。`,
	PersistentPreRunE: chainPreRunE(setupSignalPreRun, runAutoUpdate),
}

func init() {
	rootCmd.PersistentFlags().StringVar(&scope, "scope", "user", "安装范围：user 或 project（仅 bundle）")
	rootCmd.PersistentFlags().StringVar(&ide, "ide", "", "目标 IDE，逗号分隔：qoder,cursor,claude")
	rootCmd.PersistentFlags().StringVar(&kind, "kind", "", "过滤类型：bundle、cli 或 hooks（用于 list）")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "仅预览将执行的操作")
	rootCmd.PersistentFlags().BoolVar(&asJSON, "json", false, "JSON 格式输出")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "输出详细日志")
	// 将 cobra 的 verbose flag 值同步到 log 包
	cobra.OnInitialize(func() {
		if v, _ := rootCmd.Flags().GetBool("verbose"); v {
			log.SetVerbose(true)
		}
	})
}

// chainPreRunE 将多个 PersistentPreRunE 函数串行执行。
// 若前一个返回错误，则后续不执行。
func chainPreRunE(fns ...func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		for _, fn := range fns {
			if err := fn(cmd, args); err != nil {
				return err
			}
		}
		return nil
	}
}

func Execute() error {
	err := rootCmd.Execute()
	// 命令执行完毕后触发清理
	shutdownCleanup()
	return err
}

func SplitIDEs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
