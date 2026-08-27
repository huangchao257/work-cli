package demo

import (
	"io"
	"strings"
)

// ioNopCloser 包装字符串读取器为 io.ReadCloser。
func ioNopCloser(r *strings.Reader) io.ReadCloser { return io.NopCloser(r) }
