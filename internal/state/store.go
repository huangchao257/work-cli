package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	path string
}

func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{path: path}, nil
}

func (s *Store) Load() (*File, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{}, nil
		}
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("解析状态文件失败: %w", err)
	}
	return &f, nil
}

func (s *Store) Save(f *File) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *Store) Upsert(rec BundleRecord) error {
	f, err := s.Load()
	if err != nil {
		return err
	}
	if rec.InstalledAt.IsZero() {
		rec.InstalledAt = time.Now().UTC()
	}
	for i, existing := range f.Bundles {
		if existing.Name == rec.Name && existing.Scope == rec.Scope {
			f.Bundles[i] = rec
			return s.Save(f)
		}
	}
	f.Bundles = append(f.Bundles, rec)
	return s.Save(f)
}

func (s *Store) Remove(name, scope string) error {
	f, err := s.Load()
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
	return s.Save(f)
}

func (s *Store) Find(name, scope string) (*BundleRecord, error) {
	f, err := s.Load()
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

func (s *Store) List(kindFilter string) ([]BundleRecord, error) {
	f, err := s.Load()
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
