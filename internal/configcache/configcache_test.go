package configcache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileCacheHit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("registry:\n  url: https://example.com\n")

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// 第一次读取：应命中磁盘
	data1, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data1) != string(content) {
		t.Fatalf("expected %q, got %q", content, data1)
	}

	// 第二次读取：文件未变，应命中缓存
	data2, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data2) != string(content) {
		t.Fatalf("expected %q, got %q", content, data2)
	}
}

func TestReadFileCacheMissAfterMtimeChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content1 := []byte("registry:\n  url: https://old.example.com\n")

	if err := os.WriteFile(path, content1, 0o644); err != nil {
		t.Fatal(err)
	}

	// 第一次读取
	data1, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data1) != string(content1) {
		t.Fatalf("expected %q, got %q", content1, data1)
	}

	// 修改文件内容
	content2 := []byte("registry:\n  url: https://new.example.com\n")
	if err := os.WriteFile(path, content2, 0o644); err != nil {
		t.Fatal(err)
	}

	// 第二次读取：mtime 已变，应重新读盘
	data2, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data2) != string(content2) {
		t.Fatalf("expected %q, got %q", content2, data2)
	}
}

func TestReadFileInvalidateForcesReread(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content1 := []byte("key: old\n")

	if err := os.WriteFile(path, content1, 0o644); err != nil {
		t.Fatal(err)
	}

	// 首次读取，填充缓存
	data1, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data1) != string(content1) {
		t.Fatalf("expected %q, got %q", content1, data1)
	}

	// 外部修改文件（不经过本包）
	content2 := []byte("key: new\n")
	if err := os.WriteFile(path, content2, 0o644); err != nil {
		t.Fatal(err)
	}

	// 显式失效缓存
	Invalidate(path)

	// 此时应强制重新读盘
	data2, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data2) != string(content2) {
		t.Fatalf("expected %q, got %q", content2, data2)
	}
}

func TestReadFileNonexistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.yaml")

	_, err := ReadFile(path)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestReadFileCacheIsolation(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.yaml")
	pathB := filepath.Join(dir, "b.yaml")
	contentA := []byte("file: a\n")
	contentB := []byte("file: b\n")

	if err := os.WriteFile(pathA, contentA, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, contentB, 0o644); err != nil {
		t.Fatal(err)
	}

	// 分别读取两个文件
	da, err := ReadFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	db, err := ReadFile(pathB)
	if err != nil {
		t.Fatal(err)
	}

	if string(da) != string(contentA) {
		t.Fatalf("pathA: expected %q, got %q", contentA, da)
	}
	if string(db) != string(contentB) {
		t.Fatalf("pathB: expected %q, got %q", contentB, db)
	}

	// 修改 A，不影响 B 的缓存
	newContentA := []byte("file: a-modified\n")
	if err := os.WriteFile(pathA, newContentA, 0o644); err != nil {
		t.Fatal(err)
	}

	da2, err := ReadFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	db2, err := ReadFile(pathB)
	if err != nil {
		t.Fatal(err)
	}

	if string(da2) != string(newContentA) {
		t.Fatalf("pathA after modify: expected %q, got %q", newContentA, da2)
	}
	if string(db2) != string(contentB) {
		t.Fatalf("pathB should still be cached: expected %q, got %q", contentB, db2)
	}
}

func TestReadFile_ConcurrentModification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Write initial content
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v1" {
		t.Fatalf("expected v1, got %s", string(data))
	}

	// Modify file externally (simulating external tool)
	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Read again — should get v2, NOT stale v1
	data, err = ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v2" {
		t.Fatalf("expected v2, got %s", string(data))
	}
}

func TestInvalidateNonexistentKey(t *testing.T) {
	// 应该不 panic
	Invalidate("/nonexistent/path/foo.yaml")
}
