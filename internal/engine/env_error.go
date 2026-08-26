// Package engine 定义「环境不满足」类错误。
// 这类错误（缺环境变量、显式指定的 IDE 未安装）在 CLI 层映射为退出码 3，
// 与 usage.Error（→2）、一般错误（→1）区分，供脚本/CI 感知环境问题。
// 见 docs/design/overview.md §7。

package engine

import (
	"errors"
	"fmt"
)

// EnvError 标记环境不满足的错误，应映射为退出码 3。
type EnvError struct {
	msg string
}

func (e *EnvError) Error() string { return e.msg }

// envError 构造 EnvError。
func envError(format string, args ...any) error {
	return &EnvError{msg: fmt.Sprintf(format, args...)}
}

// IsEnvError 判断 err 及其包装链中是否存在 EnvError。
func IsEnvError(err error) bool {
	var ee *EnvError
	return errors.As(err, &ee)
}
