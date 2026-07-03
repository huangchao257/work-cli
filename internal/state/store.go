package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// exclusiveLock 对文件描述符 fd 加独占锁（阻塞式），防止并发写入损坏 installed.json。
// 阻塞超时 5 秒，避免死锁导致命令永久挂起。
func exclusiveLock(f *os.File, path string) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
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

// exclusiveUnlock 释放文件描述符 fd 上的独占锁。
func exclusiveUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

type Store struct {
	path string
}

func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建状态目录失败: %w", err)
	}
	return &Store{path: path}, nil
}

// loadUnlocked 读取并解析状态文件（调用方需持有锁）。
func loadUnlocked(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{}, nil
		}
		return nil, err
	}
	// 处理空文件（可能是 O_CREATE 新建的空文件）
	if len(bytes.TrimSpace(data)) == 0 {
		return &File{}, nil
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("解析状态文件失败: %w", err)
	}
	return &f, nil
}

// saveUnlocked 序列化并写入状态文件（调用方需持有锁）。
func saveUnlocked(path string, f *File) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("编码状态文件失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入状态文件失败: %w", err)
	}
	return nil
}

// withLock 持有文件锁执行 fn，保证并发安全。
func (s *Store) withLock(fn func() error) error {
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("打开状态文件失败: %w", err)
	}
	defer f.Close()
	if err := exclusiveLock(f, s.path); err != nil {
		return err
	}
	defer func() {
		// 释放锁失败仅记录，不影响主流程
		_ = exclusiveUnlock(f)
	}()
	return fn()
}

// Load 读取并解析状态文件（外部访问，不加锁；仅读取快照）。
// 需要并发安全的读取-修改-写入时，请使用 Store 的其他方法。
func (s *Store) Load() (*File, error) {
	return loadUnlocked(s.path)
}

// Save 写入状态文件（外部访问，不加锁）。
// 需要并发安全的写入时，请使用 Store 的其他方法。
func (s *Store) Save(f *File) error {
	return saveUnlocked(s.path, f)
}

// Upsert 插入或更新一条 BundleRecord。采用文件锁保证并发安全。
func (s *Store) Upsert(rec BundleRecord) error {
	if strings.TrimSpace(rec.Name) == "" {
		return fmt.Errorf("记录名称不能为空")
	}
	if strings.TrimSpace(rec.Scope) == "" {
		return fmt.Errorf("记录范围不能为空")
	}
	return s.withLock(func() error {
		f, err := loadUnlocked(s.path)
		if err != nil {
			return err
		}
		if rec.InstalledAt.IsZero() {
			rec.InstalledAt = time.Now().UTC()
		}
		for i, existing := range f.Bundles {
			if existing.Name == rec.Name && existing.Scope == rec.Scope {
				f.Bundles[i] = rec
				return saveUnlocked(s.path, f)
			}
		}
		f.Bundles = append(f.Bundles, rec)
		return saveUnlocked(s.path, f)
	})
}

// Remove 从状态中移除指定 name/scope 的记录，加锁保证并发安全。
func (s *Store) Remove(name, scope string) error {
	return s.withLock(func() error {
		f, err := loadUnlocked(s.path)
		if err != nil {
			return err
		}
		out := make([]BundleRecord, 0, len(f.Bundles))
		found := false
		for _, b := range f.Bundles {
			if b.Name == name && b.Scope == scope {
				found = true
				continue
			}
			out = append(out, b)
		}
		if !found {
			return fmt.Errorf("未找到已安装项: %s (scope=%s)", name, scope)
		}
		f.Bundles = out
		return saveUnlocked(s.path, f)
	})
}

// Find 查找指定 name/scope 的记录，返回深拷贝避免外部修改污染内存缓存。
// 读取操作不加锁（数据快照），调用方不应假设强一致性。
func (s *Store) Find(name, scope string) (*BundleRecord, error) {
	f, err := loadUnlocked(s.path)
	if err != nil {
		return nil, err
	}
	for _, b := range f.Bundles {
		if b.Name == name && b.Scope == scope {
			copy := b
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("未找到已安装项: %s (scope=%s)", name, scope)
}

// List 列出已安装记录，可按 kind 过滤。
// 读取操作不加锁（数据快照），调用方不应假设强一致性。
func (s *Store) List(kindFilter string) ([]BundleRecord, error) {
	f, err := loadUnlocked(s.path)
	if err != nil {
		return nil, err
	}
	if kindFilter == "" {
		return f.Bundles, nil
	}
	out := make([]BundleRecord, 0)
	for _, b := range f.Bundles {
		if b.Kind == kindFilter {
			out = append(out, b)
		}
	}
	return out, nil
}
