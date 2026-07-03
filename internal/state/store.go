package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

// Load 读取状态文件（加共享锁）返回快照。
func (s *Store) Load() (*File, error) {
	f, err := os.OpenFile(s.path, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开状态文件失败: %w", err)
	}
	defer f.Close()
	if err := flockLock(f, s.path, lockSH); err != nil {
		return nil, fmt.Errorf("获取状态文件共享锁失败: %w", err)
	}
	defer func() { _ = flockUnlock(f) }()

	return readFileUnlocked(f)
}

// Save 写入状态文件（加独占锁 + 原子写入）。
func (s *Store) Save(f *File) error {
	return s.withLock(func() error {
		return atomicWrite(s.path, f)
	})
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
		file, err := readStateFile(s.path)
		if err != nil {
			return err
		}
		if rec.InstalledAt.IsZero() {
			rec.InstalledAt = time.Now().UTC()
		}
		for i, existing := range file.Bundles {
			if existing.Name == rec.Name && existing.Scope == rec.Scope {
				file.Bundles[i] = rec
				return atomicWrite(s.path, file)
			}
		}
		file.Bundles = append(file.Bundles, rec)
		return atomicWrite(s.path, file)
	})
}

// Remove 从状态中移除指定 name/scope 的记录，加锁保证并发安全。
func (s *Store) Remove(name, scope string) error {
	return s.withLock(func() error {
		file, err := readStateFile(s.path)
		if err != nil {
			return err
		}
		out := make([]BundleRecord, 0, len(file.Bundles))
		found := false
		for _, b := range file.Bundles {
			if b.Name == name && b.Scope == scope {
				found = true
				continue
			}
			out = append(out, b)
		}
		if !found {
			return fmt.Errorf("未找到已安装项: %s (scope=%s)", name, scope)
		}
		file.Bundles = out
		return atomicWrite(s.path, file)
	})
}

// Find 查找指定 name/scope 的记录（加共享锁），返回深拷贝。
func (s *Store) Find(name, scope string) (*BundleRecord, error) {
	f, err := os.OpenFile(s.path, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开状态文件失败: %w", err)
	}
	defer f.Close()
	if err := flockLock(f, s.path, lockSH); err != nil {
		return nil, fmt.Errorf("获取状态文件共享锁失败: %w", err)
	}
	defer func() { _ = flockUnlock(f) }()

	file, err := readFileUnlocked(f)
	if err != nil {
		return nil, err
	}
	for _, b := range file.Bundles {
		if b.Name == name && b.Scope == scope {
			copy := b
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("未找到已安装项: %s (scope=%s)", name, scope)
}

// List 列出已安装记录（加共享锁），可按 kind 过滤。
func (s *Store) List(kindFilter string) ([]BundleRecord, error) {
	f, err := os.OpenFile(s.path, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开状态文件失败: %w", err)
	}
	defer f.Close()
	if err := flockLock(f, s.path, lockSH); err != nil {
		return nil, fmt.Errorf("获取状态文件共享锁失败: %w", err)
	}
	defer func() { _ = flockUnlock(f) }()

	file, err := readFileUnlocked(f)
	if err != nil {
		return nil, err
	}
	if kindFilter == "" {
		return file.Bundles, nil
	}
	out := make([]BundleRecord, 0)
	for _, b := range file.Bundles {
		if b.Kind == kindFilter {
			out = append(out, b)
		}
	}
	return out, nil
}

// lockSH / lockEX 用于跨平台锁类型
const (
	lockSH = 1 // shared
	lockEX = 2 // exclusive
)

// withLock 持有独占锁执行 fn，保证并发安全。
func (s *Store) withLock(fn func() error) error {
	f, err := os.OpenFile(s.path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("打开状态文件失败: %w", err)
	}
	defer f.Close()
	if err := flockLock(f, s.path, lockEX); err != nil {
		return err
	}
	defer func() { _ = flockUnlock(f) }()
	return fn()
}

// readStateFile 读状态文件（调用方需持有锁）。
func readStateFile(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, fmt.Errorf("读取状态文件失败: %w", err)
	}
	defer f.Close()
	return readFileUnlocked(f)
}

// readFileUnlocked 从已打开的文件描述符解析 File。
func readFileUnlocked(f *os.File) (*File, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取状态文件信息失败: %w", err)
	}
	if fi.Size() == 0 {
		return &File{}, nil
	}
	var file File
	if err := json.NewDecoder(f).Decode(&file); err != nil {
		return nil, fmt.Errorf("解析状态文件失败: %w", err)
	}
	return &file, nil
}

// atomicWrite 原子写入：先写临时文件再 rename，防止读到半写内容。
func atomicWrite(path string, f *File) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("编码状态文件失败: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".installed-*.json")
	if err != nil {
		return fmt.Errorf("创建临时状态文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入临时状态文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时状态文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("原子写入状态文件失败: %w", err)
	}
	cleanup = false
	return nil
}
