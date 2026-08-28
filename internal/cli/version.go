package cli

import (
	"fmt"

	"github.com/huangchao257/work-cli/internal/log"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

// Version 由构建时 -ldflags 注入，开发构建默认为 dev。
var Version = "dev"

var (
	versionCheckUpdate bool
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本号",
	Long:  "显示当前 work 版本。默认会检查 GitHub 是否有新版本可用。",
	Example: `  work version
		  work version --json
		  work version --check-update=false`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cmd.OutOrStdout()
		// 检查更新（提示只进 human 模式；--json 输出契约必须保持纯 JSON）
		var latest string
		if versionCheckUpdate {
			cfg, _ := selfupdate.LoadConfig()
			updater := selfupdate.NewUpdater(Version)
			updater.Channel = cfg.Channel
			res, err := updater.Check(signalContext())
			if err != nil {
				log.Warnf("[work]", "检查更新失败: %v", err)
			} else if res.UpdateAvailable {
				latest = res.Latest
			}
		}

		if asJSON {
			return output.PrintJSON(w, map[string]any{
				"version":          Version,
				"update_available": latest != "",
				"latest":           latest,
			})
		}
		if _, err := fmt.Fprintln(w, Version); err != nil {
			return err
		}
		if latest != "" {
			_, err := fmt.Fprintf(w, "有新版本 %s 可用，运行 work upgrade 更新\n", latest)
			return err
		}
		return nil
	},
}

func init() {
	versionCmd.Flags().BoolVar(&versionCheckUpdate, "check-update", true, "检查是否有新版本")
	rootCmd.AddCommand(versionCmd)
}
