package cli

import (
	"errors"

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
