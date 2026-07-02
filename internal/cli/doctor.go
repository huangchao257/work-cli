package cli

import (
	"fmt"

	"github.com/huangchao257/work-cli/internal/doctor"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "体检本机运行环境",
	Long: `work doctor 一键体检本机 work 运行环境。

依次检查：IDE 探测、work 是否在 PATH、config.yaml 合法性、
installed.json 可读性、各 IDE 的 MCP 配置合法性、codegraph/jq 可用性、
自更新配置概况。任一 error 项失败将以退出码 1 返回，便于 CI 集成。`,
	Example: `  work doctor                 # 体检当前环境
  work doctor --ide qoder     # 指定 IDE 后额外校验是否检测到
  work doctor --json          # JSON 格式输出`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := doctor.Options{
			Scope: scope,
			IDEs:  SplitIDEs(ide),
		}
		results := doctor.Run(opts)
		hasErr := doctor.HasError(results)

		if asJSON {
			_ = output.PrintJSON(cmd.OutOrStdout(), map[string]any{
				"checks": results,
			})
			if hasErr {
				return exitErr(1, fmt.Errorf("体检未通过"))
			}
			return nil
		}

		printHumanDoctor(cmd, results)
		if hasErr {
			return exitErr(1, fmt.Errorf("体检未通过"))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

// printHumanDoctor 以人类可读清单形式输出诊断结果。
func printHumanDoctor(cmd *cobra.Command, results []doctor.CheckResult) {
	w := cmd.OutOrStdout()
	for _, r := range results {
		mark := "✓"
		if !r.OK {
			mark = "✗"
		}
		fmt.Fprintf(w, "%s %s — %s\n", mark, r.Name, r.Detail)
	}
	passed, failed := doctor.Summary(results)
	fmt.Fprintf(w, "\n%d 项通过 / %d 项失败\n", passed, failed)
}
