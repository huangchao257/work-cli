package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/huangchao257/work-cli/internal/selfupdate"
)

var noAutoUpdate bool

func init() {
	rootCmd.PersistentFlags().BoolVar(&noAutoUpdate, "no-auto-update", false, "跳过本次命令的 work 自动更新检查")
	rootCmd.PersistentPreRunE = runAutoUpdate
}

func runAutoUpdate(cmd *cobra.Command, args []string) error {
	if shouldSkipAutoUpdate(cmd) {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	res, err := selfupdate.TryAuto(ctx, selfupdate.AutoOptions{CurrentVersion: Version})
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ 自动更新检查失败: %v\n", err)
		return nil
	}
	if !res.Updated {
		return nil
	}

	selfupdate.NotifyAutoUpdate(os.Stderr, res)
	if err := reExecute(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ 自动更新后重新执行失败: %v\n请重新运行原命令\n", err)
	}
	return nil
}

func shouldSkipAutoUpdate(cmd *cobra.Command) bool {
	if noAutoUpdate || dryRun || asJSON {
		return true
	}
	name := cmd.Name()
	switch name {
	case "upgrade", "version", "help", "completion", "work":
		return true
	}
	if strings.HasPrefix(name, "help") {
		return true
	}
	return false
}
