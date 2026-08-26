//go:build windows

package platform

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// FlockLock Windows — 使用 LockFileEx 实现跨进程文件锁（阻塞式，超时 5 秒）。
// how 取 FlockSH（共享）或 FlockEX（独占）。LockFileEx 的锁绑定文件区域而非 inode，
// 而我们约定所有调用方都锁固定区域 [0,1)，且锁文件从不 rename/remove，故跨进程互斥有效。
func FlockLock(f *os.File, path string, how int) error {
	var flags uint32 = windows.LOCKFILE_FAIL_IMMEDIATELY
	if how == FlockEX {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		err := lockFileEx(f, flags)
		if err == nil {
			return nil
		}
		if err != windows.ERROR_LOCK_VIOLATION && err != windows.ERROR_IO_PENDING {
			return fmt.Errorf("加锁文件失败 %s: %w", path, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("获取文件锁超时 %s，可能有其他 work 进程正在操作", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// FlockUnlock Windows — 释放文件锁。how 对 unlock 无意义，直接对整个区域解锁。
func FlockUnlock(f *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}

// lockFileEx 对文件 [0,1) 区域加锁。
func lockFileEx(f *os.File, flags uint32) error {
	var ol windows.Overlapped
	// 加锁区域 [0,1)：锁定文件起始 1 字节。不传 FAIL_IMMEDIATELY 时会阻塞；
	// 这里 flags 已含 LOCKFILE_FAIL_IMMEDIATELY，冲突立即返回 ERROR_LOCK_VIOLATION。
	if err := windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, &ol); err != nil {
		return err
	}
	return nil
}

// Lock constants
const (
	FlockSH = 1
	FlockEX = 2
)
