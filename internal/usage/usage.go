// Package usage 提供共享的 UsageError 类型，各领域包返回该错误，
// CLI 层通过 Is / ExitCode 统一映射为退出码 2。
package usage

import (
	"errors"
	"fmt"
)

// Error 表示用法错误（非法参数、缺少必填字段等），应映射为退出码 2。
// 可用 New / Newf / Wrap / Wrapf 创建。
type Error struct {
	msg string
	err error // 可选，底层错误
}

func (e *Error) Error() string {
	if e.err != nil {
		return e.msg + ": " + e.err.Error()
	}
	return e.msg
}

func (e *Error) Unwrap() error {
	return e.err
}

// As 让 errors.As 能匹配到 *Error。所有构造函数返回 *Error 或包含它的
// fmt.Errorf 包装链，此方法确保 Is() 可穿透。

// New 创建一个新的 Error。
func New(msg string) *Error {
	return &Error{msg: msg}
}

// Newf 用格式化字符串创建 Error（不含 %w）。
func Newf(format string, a ...any) *Error {
	return &Error{msg: fmt.Sprintf(format, a...)}
}

// Wrap 包装一个底层错误为 Error。
func Wrap(err error, msg string) *Error {
	return &Error{msg: msg, err: err}
}

// Wrapf 用格式化字符串创建 Error。如果 format 不含 %w，等价 Newf；
// 如果含单个 %w，提取对应 error 作为底层错误，消息其余部分按 fmt.Sprintf。
func Wrapf(format string, a ...any) *Error {
	// 尝试用 fmt.Errorf 构建，再提取消息和内层错误。
	e := fmt.Errorf(format, a...)
	inner := errors.Unwrap(e)
	msg := e.Error()
	return &Error{msg: msg, err: inner}
}

// Is 判断 err 及其包装链中是否存在 Error。
func Is(err error) bool {
	var ue *Error
	return errors.As(err, &ue)
}
