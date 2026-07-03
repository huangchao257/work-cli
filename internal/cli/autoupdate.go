package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/huangchao257/work-cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

var noAutoUpdate bool

func init() {
	rootCmd.PersistentFlags().BoolVar(&noAutoUpdate, "no-auto-update", false, "跳过本次命令的 work 自动更新检查")
	rootCmd.PersistentPreRunE = runAutoUpdate
}

// runAutoUpdate 以异步方式检查自更新。检查请求在后台 goroutine 中发起，
// 主命令不等待网络 I/O 即可继续执行。仅当确有更新时，后台下载完成后
// 会通过 reExecute 重新执行新版本（替换当前进程）。
func runAutoUpdate(cmd *cobra.Command, args []string) error {
	if shouldSkipAutoUpdate(cmd) {
		return nil
	}

	// 在后台异步执行更新检查，避免每次命令启动阻塞在网络 I/O 上。
	// 如果没有更新，goroutine 静默退出；如果有更新，下载完成后重新执行。
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		res, err := selfupdate.TryAuto(ctx, selfupdate.AutoOptions{CurrentVersion: Version})
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠ 自动更新检查失败: %v\n", err)
			return
		}
		if !res.Updated {
			return
		}

		selfupdate.NotifyAutoUpdate(os.Stderr, res)
		if err := reExecute(); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ 自动更新后重新执行失败: %v\n请重新运行原命令\n", err)
		}
	}()
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
