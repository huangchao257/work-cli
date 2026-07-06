// Package engine 提供共享的必需环境变量检查与报错辅助。
// 消除 bundle.go / cli_install.go / hooks.go 中重复的 env-check + hint 模式。

package engine

import (
	"fmt"
	"os"
	"strings"

	"github.com/huangchao257/work-cli/internal/platform"
)

// checkMissingEnv 检查 envNames 列表中的环境变量，若任一未设置则收集缺失名称并返回
// 包含缺失变量名称和设置提示的组合错误。envNames 为空时返回 nil。
func checkMissingEnv(envNames []string) error {
	missing := make([]string, 0, len(envNames))
	for _, name := range envNames {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("缺少必需的环境变量：")
	b.WriteString(strings.Join(missing, ", "))
	b.WriteByte('\n')
	for _, name := range missing {
		b.WriteString(platform.EnvSetHint(name))
		b.WriteByte('\n')
	}
	return fmt.Errorf("%s", b.String())
}
