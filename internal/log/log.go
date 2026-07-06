// Package log 提供统一的日志输出入口，写入 stderr，区分 Info/Warn/Error 三级。
// 支持子命令前缀（如 [work] 或 [work doctor]），并通过全局 verbose 标志控制 Infof 是否输出。
// Warnf 与 Errorf 始终输出；Infof 仅在 verbose 为 true 时输出。
// 所有方法线程安全（内部使用 sync.Mutex 保护输出）。
package log

import (
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	mu      sync.Mutex
	verbose bool
	out     io.Writer = os.Stderr
)

// SetVerbose 控制 Info 级别日志是否输出。Warn 与 Error 始终输出不受影响。
func SetVerbose(v bool) {
	mu.Lock()
	verbose = v
	mu.Unlock()
}

// Verbose 返回当前 verbose 标志。
func Verbose() bool {
	mu.Lock()
	defer mu.Unlock()
	return verbose
}

// Infof 仅在 verbose=true 时输出格式化信息到 stderr。
func Infof(prefix, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if !verbose {
		return
	}
	fmt.Fprintf(out, "%s "+format+"\n", append([]any{prefix}, args...)...)
}

// Warnf 始终输出格式化警告到 stderr。
func Warnf(prefix, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprintf(out, "%s "+format+"\n", append([]any{prefix}, args...)...)
}

// Errorf 始终输出格式化错误到 stderr。
func Errorf(prefix, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprintf(out, "%s "+format+"\n", append([]any{prefix}, args...)...)
}
