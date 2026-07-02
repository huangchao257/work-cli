package cli

import (
	"fmt"

	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/scaffold"
	"github.com/spf13/cobra"
)

var (
	initDir string
)

var initCmd = &cobra.Command{
	Use:   "init <type> <name>",
	Short: "生成套装骨架目录",
	Long: `为套装作者生成符合 manifest 规范的骨架目录，降低手写 bundle.yaml/installer.yaml/hooks.yaml 出错率。

type 可选：
  bundle  生成 bundle.yaml + skills/<name>/SKILL.md + rules/sample.md + mcp/sample.json
  cli     生成 installer.yaml + README.md
  hooks   生成 hooks.yaml + scripts/telemetry.sh（带可执行权限）

manifest 中 name/version（0.1.0）自动填入，description 留占位；字段带中文注释。`,
	Example: `  work init bundle my-kit
  work init cli my-tool --dir ./tools/my-tool
  work init hooks my-hooks --dry-run`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		t, err := scaffold.ParseType(args[0])
		if err != nil {
			return exitErr(2, err)
		}
		files, err := scaffold.Run(scaffold.Options{
			Type:   t,
			Name:   args[1],
			Dir:    initDir,
			DryRun: dryRun,
		})
		if err != nil {
			return ExitUsageErr(err)
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), map[string]any{"files": files})
		}
		w := cmd.OutOrStdout()
		label := "已创建"
		if dryRun {
			label = "预览"
		}
		for _, f := range files {
			fmt.Fprintf(w, "%s %s\n", label, f)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initDir, "dir", "", "输出目录，默认 ./<name>")
}
