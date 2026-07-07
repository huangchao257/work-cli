package cli

import (
	"fmt"

	"github.com/huangchao257/work-cli/internal/log"
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
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), Version); err != nil {
			return err
		}
		if !versionCheckUpdate {
			return nil
		}
		cfg, _ := selfupdate.LoadConfig()
		updater := selfupdate.NewUpdater(Version)
		updater.Channel = cfg.Channel
		res, err := updater.Check(signalContext())
		if err != nil {
			log.Warnf("[work]", "检查更新失败: %v", err)
			return nil
		}
		if res.UpdateAvailable {
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "有新版本 %s 可用，运行 work upgrade 更新\n", res.Latest)
		}
		return err
	},
}

func init() {
	versionCmd.Flags().BoolVar(&versionCheckUpdate, "check-update", true, "检查是否有新版本")
	rootCmd.AddCommand(versionCmd)
}
