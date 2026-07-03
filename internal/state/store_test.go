package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreUpsertRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rec := BundleRecord{Name: "dev-kit", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "local:dev-kit"}
	if err := s.Upsert(rec); err != nil {
		t.Fatal(err)
	}
	got, err := s.Find("dev-kit", "user")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "1.0.0" {
		t.Fatalf("unexpected version: %s", got.Version)
	}
	if err := s.Remove("dev-kit", "user"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Find("dev-kit", "user"); err == nil {
		t.Fatal("expected not found after remove")
	}
}

func TestStoreListDeepCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	rec := BundleRecord{
		Name:        "test-pkg",
		Kind:        "bundle",
		Version:     "1.0.0",
		Scope:       "user",
		Ref:         "test-pkg",
		InstalledAt: time.Now().UTC(),
	}
	if err := s.Upsert(rec); err != nil {
		t.Fatal(err)
	}

	// List 返回切片的浅拷贝（直接返回 file.Bundles，不是深拷贝）
	// 但 loaded 缓存应该是深拷贝的
	bundles1, err := s.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles1) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(bundles1))
	}

	// 篡改 List 返回的第一个结果
	bundles1[0].Version = "CORRUPTED"
	bundles1[0].Name = "CORRUPTED"

	// 后续 Load 应该不受影响（缓存提供深拷贝）
	file, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Bundles) != 1 {
		t.Fatalf("expected 1 bundle after corrupt, got %d", len(file.Bundles))
	}
	if file.Bundles[0].Version != "1.0.0" {
		t.Fatalf("version was corrupted, got %q", file.Bundles[0].Version)
	}
	if file.Bundles[0].Name != "test-pkg" {
		t.Fatalf("name was corrupted, got %q", file.Bundles[0].Name)
	}
}

func TestStoreCacheInvalidateOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	rec := BundleRecord{
		Name:        "pkg1",
		Kind:        "bundle",
		Version:     "1.0.0",
		Scope:       "user",
		Ref:         "pkg1",
		InstalledAt: time.Now().UTC(),
	}
	if err := s.Upsert(rec); err != nil {
		t.Fatal(err)
	}

	// 触发一次 Load 填充缓存
	file1, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(file1.Bundles) != 1 {
		t.Fatalf("expected 1, got %d", len(file1.Bundles))
	}

	// 通过外部手段修改 installed.json（不经过 Store）
	rec2 := BundleRecord{
		Name:        "pkg2",
		Kind:        "bundle",
		Version:     "2.0.0",
		Scope:       "user",
		Ref:         "pkg2",
		InstalledAt: time.Now().UTC(),
	}
	// 直接写入文件绕过 Store
	rawFile := File{Bundles: []BundleRecord{rec2}}
	if err := saveFileDirect(path, &rawFile); err != nil {
		t.Fatal(err)
	}

	// 但缓存未失效，Load 仍返回旧缓存
	file2, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(file2.Bundles) != 1 || file2.Bundles[0].Name != "pkg1" {
		t.Fatalf("cache should still return old data before invalidation: %+v", file2.Bundles)
	}

	// 显式失效后应读到新数据
	s.InvalidateCache()
	file3, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(file3.Bundles) != 1 || file3.Bundles[0].Name != "pkg2" {
		t.Fatalf("expected pkg2 after invalidation, got: %+v", file3.Bundles)
	}
}

func TestStoreUpsertEmptyName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Upsert(BundleRecord{Name: "", Scope: "user"})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestStoreUpsertEmptyScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Upsert(BundleRecord{Name: "test", Scope: ""})
	if err == nil {
		t.Fatal("expected error for empty scope")
	}
}

func TestStoreRemoveNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Remove("nonexistent", "user")
	if err == nil {
		t.Fatal("expected error for removing nonexistent record")
	}
}

// saveFileDirect 直接序列化并写入状态文件，绕过 Store 接口，
// 用于模拟外部进程修改 installed.json。
func saveFileDirect(path string, f *File) error {
	// 使用简单的原子写入逻辑，避免导入循环
	tmp, err := os.CreateTemp(filepath.Dir(path), ".installed-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	tmp.Close()
	return os.Rename(tmpPath, path)
}
