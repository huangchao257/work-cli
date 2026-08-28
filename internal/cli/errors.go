package cli

import (
	"errors"

	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/usage"
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

func ExitCode(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	// 环境不满足（缺 env、指定 IDE 未安装）→ 3（见 docs/design/overview.md §7）。
	if engine.IsEnvError(err) {
		return 3
	}
	// 用法错误（含 cobra 的未知 flag/参数数量错误、未知子命令）→ 2。
	// 未包成 exitError 的裸 usage 错误在此兜底，避免泄漏为退出码 1。
	if usage.Is(err) {
		return 2
	}
	return 1
}

func exitErr(code int, err error) error {
	return &exitError{code: code, err: err}
}

// IsUsageError 判断 err 是否为用法错误（应映射为退出码 2）。
func IsUsageError(err error) bool {
	return usage.Is(err)
}

// ExitUsageErr 将 usage.Error 映射为退出码 2，其他错误透传。
func ExitUsageErr(err error) error {
	if usage.Is(err) {
		return exitErr(2, err)
	}
	return err
}
