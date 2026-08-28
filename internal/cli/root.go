package cli

import (
	"fmt"
	"strings"

	"github.com/huangchao257/work-cli/internal/log"
	"github.com/huangchao257/work-cli/internal/usage"
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
	// 显式 Args 兜底未知子命令：cobra 的 legacyArgs（root 有子命令时对未知
	// 首参报 "unknown command"）只在 Args==nil 时启用，错误以普通 error
	// 泄漏为退出码 1。此处复刻该行为并包成 usage 错误 → 退出码 2。
	// 注意：必须同时提供 Run——cobra 对不可运行的命令在 ValidateArgs 之前
	// 就返回 flag.ErrHelp（直接打印帮助退出 0），Args 校验永远到不了。
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		msg := fmt.Sprintf("unknown command %q for %q", args[0], cmd.CommandPath())
		if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
			msg += "\n\nDid you mean this?\n"
			for _, s := range suggestions {
				msg += fmt.Sprintf("\t%v\n", s)
			}
		}
		return usage.New(msg)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
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
	// 先为整棵命令树装兜底：Args 校验/flag 解析错误发生在 RunE 之前，
	// 不会被各命令的 apiFail/exitErr 拦到，默认会以普通错误（退出码 1）
	// 泄漏。统一包成 usage 错误 → 退出码 2（契约见 docs/design/overview.md §7）。
	applyUsageErrorFuncs(rootCmd)
	err := rootCmd.Execute()
	// 命令执行完毕后触发清理
	shutdownCleanup()
	return err
}

// usageErrorFuncsApplied 记录已安装过兜底错误函数的命令（幂等）。
var usageErrorFuncsApplied = map[*cobra.Command]bool{}

// applyUsageErrorFuncs 递归为命令树安装 FlagErrorFunc 与 Args 包装，
// 将 cobra 自身产生的用法错误（未知 flag、参数数量不符）标记为 usage 错误。
// 未知子命令由 ExecuteC 的 Find 路径产生，在 Execute 内统一兜底。
func applyUsageErrorFuncs(cmd *cobra.Command) {
	if !usageErrorFuncsApplied[cmd] {
		usageErrorFuncsApplied[cmd] = true
		cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
			return usage.New(err.Error())
		})
		if inner := cmd.Args; inner != nil {
			cmd.Args = func(c *cobra.Command, args []string) error {
				if err := inner(c, args); err != nil {
					return usage.New(err.Error())
				}
				return nil
			}
		}
	}
	for _, sub := range cmd.Commands() {
		applyUsageErrorFuncs(sub)
	}
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
