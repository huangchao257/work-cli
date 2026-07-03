// Package cli 提供 cobra 命令定义、持久化 flags、帮助定制与自更新钩子。

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

// runAutoUpdate 在命令执行前同步检查自更新。
// 因已通过 configcache 缓存配置读取，检查开销很低（网络请求受 2h 节流控制）。
// 若有新版本则下载、替换二进制，并重新执行同一 argv。
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
