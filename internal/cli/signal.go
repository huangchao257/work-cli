// Package cli 信号处理与优雅退出。
package cli

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	signalCtx       context.Context
	signalCancel    context.CancelFunc
	signalMu        sync.Mutex
	signalCleanupCh chan struct{} // 关闭时表示信号处理 goroutine 已退出
	signalOnce      int32         // atomic: 0=未初始化, 1=初始化中, 2=已初始化
)

// setupSignalPreRun 作为 PersistentPreRunE 链的一环，
// 在命令执行前启动信号监听，创建可取消的 context。
func setupSignalPreRun(cmd *cobra.Command, args []string) error {
	signalMu.Lock()
	defer signalMu.Unlock()

	if atomic.LoadInt32(&signalOnce) != 0 {
		// 已经初始化过（例如 PersistentPreRunE 被多次调用），跳过
		return nil
	}
	atomic.StoreInt32(&signalOnce, 1)

	ctx, cancel := context.WithCancel(context.Background())
	signalCtx = ctx
	signalCancel = cancel
	signalCleanupCh = make(chan struct{})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		defer close(signalCleanupCh)

		select {
		case <-sigCh:
			// 收到中断信号，取消 context 通知各 goroutine 清理
			cancel()
			// 等待命令正常结束或 context 取消
			// 命令执行完毕后 shutdownCleanup 会第二次 cancel，这里检测到后退出
		case <-ctx.Done():
			// 命令已正常结束，无需强制处理
		}
		signal.Stop(sigCh)
	}()

	atomic.StoreInt32(&signalOnce, 2)
	return nil
}

// shutdownCleanup 在命令执行完毕后调用，取消 context 并等待信号处理 goroutine 退出。
func shutdownCleanup() {
	signalMu.Lock()
	cancel := signalCancel
	ch := signalCleanupCh
	signalMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if ch != nil {
		<-ch // 等待信号处理 goroutine 退出
	}
}

// signalContext 返回信号感知的 context。
// 收到 SIGINT/SIGTERM 时该 context 会被取消。
// 若未初始化则返回 context.Background()。
func signalContext() context.Context {
	signalMu.Lock()
	defer signalMu.Unlock()
	if signalCtx != nil {
		return signalCtx
	}
	return context.Background()
}
