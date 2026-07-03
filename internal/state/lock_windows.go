//go:build windows

package state

import "os"

// flockLock Windows 下暂不实现文件锁；多 work 进程并发时有低概率损坏 installed.json。
// 未来可通过 LockFileEx / CreateMutex 实现等价保护。
func flockLock(f *os.File, path string, how int) error {
	return nil
}

func flockUnlock(f *os.File) error {
	return nil
}

// LockFileEx 常量（how 参数未使用，仅声明避免未使用导入）。
const _ = "TODO: LockFileEx"
