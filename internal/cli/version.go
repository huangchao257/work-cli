package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/huangchao257/work-cli/internal/selfupdate"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), Version); err != nil {
			return err
		}
		if !versionCheckUpdate {
			return nil
		}
		res, err := selfupdate.NewUpdater(Version).Check(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "检查更新失败: %v\n", err)
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
