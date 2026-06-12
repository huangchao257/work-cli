package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/huangchao257/work-cli/internal/hooks"
	"github.com/huangchao257/work-cli/internal/output"
)

var (
	hooksIDE      string
	hooksEvent    string
	hooksKit      string
	hooksScope    string
	hooksStdinFile string
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "AI IDE hooks 事件上报与管理",
	Long:  "接收 IDE hooks 事件、同步至内网 Telemetry，以及查看上报队列状态。通常由已安装的 hooks 脚本调用，无需手动执行。",
}

var hooksReportCmd = &cobra.Command{
	Use:    "report",
	Short:  "接收 hook 事件并写入本地队列",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if hooksIDE == "" || hooksEvent == "" {
			return cmd.Help()
		}
		if hooksScope == "" {
			hooksScope = "user"
		}
		stdin := cmd.InOrStdin()
		if hooksStdinFile != "" {
			f, err := os.Open(hooksStdinFile)
			if err != nil {
				return err
			}
			defer f.Close()
			stdin = f
		}
		cfg, _ := hooks.LoadTelemetryConfig()
		_, err := hooks.Report(hooks.ReportInput{
			IDE:         hooksIDE,
			IDEEvent:    hooksEvent,
			HooksKit:    hooksKit,
			Scope:       hooksScope,
			Stdin:       stdin,
			Stdout:      cmd.OutOrStdout(),
			TriggerSync: hooks.ShouldAutoSync(cfg),
		})
		return err
	},
}

var hooksSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "将本地队列上报至内网 Telemetry",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := hooks.LoadTelemetryConfig()
		if err != nil {
			return err
		}
		if err := hooks.Sync(cfg); err != nil {
			return err
		}
		if asJSON {
			st, err := hooks.GetStatus()
			if err != nil {
				return err
			}
			return output.PrintHooksStatusJSON(cmd.OutOrStdout(), st)
		}
		_, err = cmd.OutOrStdout().Write([]byte("✓ 已同步 telemetry 队列\n"))
		return err
	},
}

var hooksStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看 hooks 上报队列状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := hooks.GetStatus()
		if err != nil {
			return err
		}
		if asJSON {
			return output.PrintHooksStatusJSON(cmd.OutOrStdout(), st)
		}
		return output.PrintHooksStatusHuman(cmd.OutOrStdout(), st)
	},
}

func init() {
	hooksReportCmd.Flags().StringVar(&hooksIDE, "ide", "", "IDE：cursor / qoder / claude")
	hooksReportCmd.Flags().StringVar(&hooksEvent, "event", "", "IDE 事件名")
	hooksReportCmd.Flags().StringVar(&hooksKit, "hooks-kit", "", "来源 hooks 套装名")
	hooksReportCmd.Flags().StringVar(&hooksScope, "scope", "user", "安装范围 user / project")
	hooksReportCmd.Flags().StringVar(&hooksStdinFile, "stdin-file", "", "调试：从文件读取 stdin")

	hooksCmd.AddCommand(hooksReportCmd)
	hooksCmd.AddCommand(hooksSyncCmd)
	hooksCmd.AddCommand(hooksStatusCmd)
	rootCmd.AddCommand(hooksCmd)
}
