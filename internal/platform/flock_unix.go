//go:build !windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// FlockLock 对文件描述符 fd 加锁（阻塞式），超时 5 秒。
func FlockLock(f *os.File, path string, how int) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("加锁文件失败 %s: %w", path, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("获取文件锁超时 %s，可能有其他 work 进程正在操作", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// FlockUnlock 释放文件描述符 fd 上的锁。
func FlockUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// Lock constants
const (
	FlockSH = 1 // shared
	FlockEX = 2 // exclusive
)
