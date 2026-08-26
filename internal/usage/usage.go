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
		if e.msg == "" {
			return e.err.Error()
		}
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

// AsError 将任意 error 透传为一个 usage Error（若 err 本身已是 usage Error 则原样返回其语义）。
// 用于「该错误已在当前上下文被判定为用法错误，需要将其标记为 usage 错误」的场景，
// 避免用 %w 构造导致消息重复。
func AsError(err error) *Error {
	if err == nil {
		return New("")
	}
	var ue *Error
	if errors.As(err, &ue) {
		return ue
	}
	return &Error{err: err}
}

// Wrapf 用格式化字符串创建 Error（等价 Newf）。
// 本包不支持在 Wrapf 中使用 %w：透传一个已是 usage 错误的底层错误请用 AsError，
// 避免 Error() 将内层错误消息重复两次。
func Wrapf(format string, a ...any) *Error {
	return Newf(format, a...)
}

// Is 判断 err 及其包装链中是否存在 Error。
func Is(err error) bool {
	var ue *Error
	return errors.As(err, &ue)
}
