// Package state 管理 installed.json 持久化状态，提供并发安全的读写操作。

//go:build !windows

package state

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// flockLock 对文件描述符 fd 加锁（阻塞式），防止并发损坏 installed.json。
// 阻塞超时 5 秒，避免死锁导致命令永久挂起。
func flockLock(f *os.File, path string, how int) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("加锁状态文件失败 %s: %w", path, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("获取状态文件锁超时 %s，可能有其他 work 进程正在操作", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func flockUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
