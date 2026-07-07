// Package configcache 提供 config.yaml 文件内容的进程内缓存，
// 以减少重复的文件读取与解析。
// 缓存以文件路径 + 修改时间（mtime）为键，避免在多子系统（自更新、来源解析、
// config 命令、AI 模型配置、hooks 遥测）各自重复读盘。
package configcache

import (
	"os"
	"sync"
)

type entry struct {
	data    []byte
	modTime int64 // UnixNano，避免 time.Time 跨平台精度差异
}

var (
	mu    sync.RWMutex
	store = map[string]entry{}
)

// ReadFile 读取 path 的内容。若缓存的 mtime 与当前文件 mtime 一致则直接
// 返回缓存数据，否则重新读盘并更新缓存。
// 并发安全：读路径使用 RLock，写路径使用 Lock；写入前进行双重检查，
// 避免在释放 RLock 与获取 Lock 之间因其他 goroutine 已更新缓存而覆盖。
func ReadFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	mtime := info.ModTime().UnixNano()

	mu.RLock()
	e, ok := store[path]
	mu.RUnlock()
	if ok && e.modTime == mtime {
		// 快速路径：缓存命中且 mtime 一致，直接返回
		return e.data, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	mu.Lock()
	// 双重检查：释放 RLock 与获取 Lock 之间可能有其他 goroutine
	// 基于更新的 mtime 写入了更新的缓存，此时应保留较新的缓存。
	// 注意：始终使用读盘前的 mtime 作为缓存键，避免 os.Stat 与 os.ReadFile
	// 之间文件被外部修改导致旧内容被存入新 mtime（TOCTOU）。
	if existing, ok := store[path]; ok && existing.modTime >= mtime {
		mu.Unlock()
		return data, nil
	}
	store[path] = entry{data: data, modTime: mtime}
	mu.Unlock()
	return data, nil
}

// Invalidate 清除 path 对应的缓存条目，用于配置写入后强制重读。
func Invalidate(path string) {
	mu.Lock()
	delete(store, path)
	mu.Unlock()
}
