package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/usage"
)

// apiFail 是 api 子树统一的错误出口：打印错误信封（human 或 JSON）并返回带退出码的错误。
// 所有 api 命令的 RunE 失败路径都应经由此函数，保证 SilenceErrors 下错误仍然可见且只输出一次。
func apiFail(cmd *cobra.Command, err error) error {
	code := 1
	switch {
	case IsUsageError(err):
		code = 2
	case isEnvironmentError(err):
		code = 3
	}
	printAPIError(cmd, err)
	return exitErr(code, err)
}

// apiUnknownTarget 输出"未知系统/命令"错误并返回 exit 2，替代静默落到父命令帮助。
func apiUnknownTarget(cmd *cobra.Command, kind, name string, hint string) error {
	message := fmt.Sprintf("未知%s %q", kind, name)
	if hint != "" {
		message += "，" + hint
	}
	return apiFail(cmd, usage.New(message))
}
