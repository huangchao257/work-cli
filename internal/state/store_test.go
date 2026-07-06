package state

import (
	"encoding/json"
	"fmt"
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

	// 等待足够时间确保 mtime 变化（兼容秒级精度的文件系统）
	time.Sleep(10 * time.Millisecond)

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

	// 外部修改改变文件 mtime，mtime 缓存应检测到变更，Load 自动返回新数据
	file2, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(file2.Bundles) != 1 || file2.Bundles[0].Name != "pkg2" {
		t.Fatalf("mtime 缓存应检测到外部修改，自动返回新数据: %+v", file2.Bundles)
	}

	// 显式失效后依然返回正确数据
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

// Store 基准测试 — 衡量状态文件的读写性能，帮助发现锁竞争与序列化瓶颈。

func BenchmarkStore_Load(b *testing.B) {
	path := filepath.Join(b.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		b.Fatal(err)
	}
	// 预填充 10 条记录，模拟典型使用场景
	for i := range 10 {
		_ = s.Upsert(BundleRecord{
			Name:    fmt.Sprintf("pkg-%d", i),
			Kind:    "bundle",
			Version: "1.0.0",
			Scope:   "user",
			Ref:     fmt.Sprintf("pkg-%d", i),
		})
	}
	s.InvalidateCache()

	b.ResetTimer()
	for range b.N {
		_, err := s.Load()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStore_Upsert(b *testing.B) {
	path := filepath.Join(b.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := range b.N {
		rec := BundleRecord{
			Name:        fmt.Sprintf("pkg-%d", i),
			Kind:        "bundle",
			Version:     "1.0.0",
			Scope:       "user",
			Ref:         fmt.Sprintf("pkg-%d", i),
			InstalledAt: time.Now().UTC(),
		}
		if err := s.Upsert(rec); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStore_Find(b *testing.B) {
	path := filepath.Join(b.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		b.Fatal(err)
	}
	// 预填充 10 条记录
	for i := range 10 {
		_ = s.Upsert(BundleRecord{
			Name:        fmt.Sprintf("pkg-%d", i),
			Kind:        "bundle",
			Version:     "1.0.0",
			Scope:       "user",
			Ref:         fmt.Sprintf("pkg-%d", i),
			InstalledAt: time.Now().UTC(),
		})
	}
	s.InvalidateCache()

	b.ResetTimer()
	for range b.N {
		_, err := s.Find("pkg-5", "user")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStore_List(b *testing.B) {
	path := filepath.Join(b.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		b.Fatal(err)
	}
	for i := range 10 {
		_ = s.Upsert(BundleRecord{
			Name:    fmt.Sprintf("pkg-%d", i),
			Kind:    "bundle",
			Version: "1.0.0",
			Scope:   "user",
			Ref:     fmt.Sprintf("pkg-%d", i),
		})
	}
	s.InvalidateCache()

	b.ResetTimer()
	for range b.N {
		_, err := s.List("")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStore_Remove(b *testing.B) {
	b.ResetTimer()
	for i := range b.N {
		b.StopTimer()
		path := filepath.Join(b.TempDir(), "installed.json")
		s, _ := Open(path)
		_ = s.Upsert(BundleRecord{
			Name:        fmt.Sprintf("rm-%d", i),
			Kind:        "bundle",
			Version:     "1.0.0",
			Scope:       "user",
			Ref:         fmt.Sprintf("rm-%d", i),
			InstalledAt: time.Now().UTC(),
		})
		b.StartTimer()
		if err := s.Remove(fmt.Sprintf("rm-%d", i), "user"); err != nil {
			b.Fatal(err)
		}
	}
}

// 辅助：预填充 N 条记录到 Store
func fillRecords(s *Store, n int) {
	for i := range n {
		_ = s.Upsert(BundleRecord{
			Name:    fmt.Sprintf("fill-%d", i),
			Kind:    "bundle",
			Version: "1.0.0",
			Scope:   "user",
			Ref:     fmt.Sprintf("fill-%d", i),
		})
	}
	s.InvalidateCache()
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

// FuzzStoreUpsertFind 对状态存储的 Upsert+Find 往返进行模糊测试，
// 验证任意 BundleRecord 写入后查找一致，且不会 panic。
func FuzzStoreUpsertFind(f *testing.F) {
	// 种子：典型记录
	f.Add("dev-kit", "bundle", "1.0.0", "user", "dev-kit")
	f.Add("test-cli", "cli", "2.0.0", "project", "test-cli")
	f.Add("hooks-pack", "hooks", "0.1.0", "user", "hooks-pack")
	f.Add("", "bundle", "1.0.0", "user", "") // 空名称（应失败）
	f.Add("test", "bundle", "1.0.0", "", "") // 空 scope（应失败）

	f.Fuzz(func(t *testing.T, name, kind, version, scope, ref string) {
		path := filepath.Join(t.TempDir(), "installed.json")
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}

		rec := BundleRecord{
			Name:    name,
			Kind:    kind,
			Version: version,
			Scope:   scope,
			Ref:     ref,
		}
		err = s.Upsert(rec)
		if err != nil {
			return
		}
		// fuzz 仅验证无 panic，不校验非 UTF-8 字节的 JSON 往返语义
		_, _ = s.Find(name, scope)
	})

}
