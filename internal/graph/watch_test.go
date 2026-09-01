package graph

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// --- isSourceFile ---

func TestIsSourceFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// 各语言扩展名
		{"main.go", true},
		{"a/defaults.yaml", true},
		{"a/defaults.yml", true},
		{"conf/app.json", true},
		{"README.md", true},
		{"scripts/gen.py", true},
		{"src/lib.rs", true},
		{"src/app.ts", true},
		{"src/app.tsx", true},
		{"src/app.js", true},
		{"src/app.jsx", true},
		{"Main.java", true},
		{"Main.kt", true},
		{"View.swift", true},
		{"main.c", true},
		{"main.cpp", true},
		{"header.h", true},
		{"header.hpp", true},
		{"app.rb", true},
		{"app.php", true},
		{"App.scala", true},
		{"App.cs", true},
		{"api.proto", true},
		// 非源码扩展名
		{"logo.png", false},
		{"binary.o", false},
		{"notes.txt", false},
		{"archive.tar.gz", false}, // Ext 只取最后一段 ".gz"
		{"Makefile", false},       // 无扩展名
		{"noext", false},
		// 大小写敏感：.GO 不是源码扩展名（Ext 原样匹配）
		{"FILE.GO", false},
	}
	for _, tt := range tests {
		if got := isSourceFile(tt.path); got != tt.want {
			t.Errorf("isSourceFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// --- isGeneratedArtifact ---

func TestIsGeneratedArtifact(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"AGENTS.md", true},
		{"a/b/agents.md", true},  // EqualFold：大小写不敏感
		{"a/b/Agents.MD", true},  // EqualFold：大小写不敏感
		{"AGENTS.mdx", false},    // 必须精确 .md
		{"AGENTS.md.bak", false}, // Base 是完整文件名
		{"AGENTS", false},
		{"README.md", false},
		{"a/AGENTS.md.orig", false},
	}
	for _, tt := range tests {
		if got := isGeneratedArtifact(tt.path); got != tt.want {
			t.Errorf("isGeneratedArtifact(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// --- shortPath ---

func TestShortPath(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		full string
		want string
	}{
		{"根内文件返回相对路径", filepath.Join(root, "a", "b.go"), filepath.Join("a", "b.go")},
		{"根自身", root, "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortPath(root, tt.full); got != tt.want {
				t.Fatalf("shortPath(%q) = %q, want %q", tt.full, got, tt.want)
			}
		})
	}
}

// TestShortPathUnrelated 根外路径 Rel 成功时返回 ../ 形式的相对路径（锁定当前行为）。
func TestShortPathUnrelated(t *testing.T) {
	root := t.TempDir()
	sibling := filepath.Join(filepath.Dir(root), "unrelated.go")
	got := shortPath(root, sibling)
	if !strings.HasPrefix(got, "..") {
		t.Fatalf("根外路径应相对化为 ../ 开头, got %q", got)
	}
}

// --- addDirs ---

// TestAddDirsSkipsExcluded 验证排除目录不被监控。
func TestAddDirsSkipsExcluded(t *testing.T) {
	excluded := []string{
		".git", ".codegraph", "node_modules", "vendor", "dist", "build", "target",
		".goreleaser", "__pycache__",
	}
	for _, name := range excluded {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			inner := filepath.Join(root, name, "deep")
			if err := os.MkdirAll(inner, 0o755); err != nil {
				t.Fatal(err)
			}
			w, err := fsnotify.NewWatcher()
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			if err := addDirs(w, root); err != nil {
				t.Fatalf("addDirs 报错: %v", err)
			}
			for _, dir := range w.WatchList() {
				if strings.HasPrefix(dir, inner) {
					t.Fatalf("排除目录不应被监控: %s (watched: %v)", inner, w.WatchList())
				}
			}
		})
	}
}

// TestAddDirsWatchesNormalDirs 验证普通目录与深层子目录被纳入监控。
func TestAddDirsWatchesNormalDirs(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "pkg", "graph", "internal")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := addDirs(w, root); err != nil {
		t.Fatalf("addDirs 报错: %v", err)
	}
	watched := w.WatchList()
	if !containsPath(watched, root) {
		t.Fatalf("根目录应被监控, watched: %v", watched)
	}
	if !containsPath(watched, deep) {
		t.Fatalf("深层普通子目录应被监控, watched: %v", watched)
	}
}

// TestAddDirsSkipsAnyHiddenDir 验证任意 . 开头的隐藏目录（未列名的）也被跳过。
func TestAddDirsSkipsAnyHiddenDir(t *testing.T) {
	root := t.TempDir()
	hidden := filepath.Join(root, ".idea")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := addDirs(w, root); err != nil {
		t.Fatalf("addDirs 报错: %v", err)
	}
	if containsPath(w.WatchList(), hidden) {
		t.Fatalf("任意 . 开头目录不应被监控: %s", hidden)
	}
}

// containsPath 在路径列表中做精确匹配（WatchList 各平台分隔符大小写可能不同，统一 Clean 比较）。
func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if filepath.Clean(p) == filepath.Clean(want) {
			return true
		}
	}
	return false
}

// --- Watch 冒烟 ---

// TestWatchSmoke 临时目录冒烟：写入 .go 源码触发 generate-agents.sh 执行；
// AGENTS.md 自身重写不得再次触发（防自触发循环回归）。
func TestWatchSmoke(t *testing.T) {
	skipOnWindows(t)
	installFakeCodegraph(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := t.TempDir()
	marker := filepath.Join(root, "gen.marker")

	// 项目级 generate-agents.sh：追加一行标记证明自己被调用
	writeFakeScript(t, filepath.Join(root, ".claude", "skills", skillID, "scripts", "generate-agents.sh"),
		"echo ran >> \""+marker+"\"")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, WatchOptions{ProjectPath: root, Debounce: 100 * time.Millisecond})
	}()

	// 轮询写入源码直到标记出现（等待 watcher 就绪 + 100ms 防抖 + 脚本执行）。
	src := filepath.Join(root, "main.go")
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err := os.WriteFile(src, []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if content, err := os.ReadFile(marker); err == nil && len(content) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if content, err := os.ReadFile(marker); err != nil || len(content) == 0 {
		t.Fatalf("写入源码文件后 generate-agents.sh 未执行（标记文件未出现）: err=%v", err)
	}

	// 等待事件流静默：轮询阶段的反复写入可能留下待触发的防抖回调，
	// 必须等它落定后再取基线，否则会把 pending 的同步算到 AGENTS.md 头上。
	countRuns := func() int {
		data, err := os.ReadFile(marker)
		if err != nil {
			return 0
		}
		return strings.Count(string(data), "ran")
	}
	quietDeadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(quietDeadline) {
		prev := countRuns()
		time.Sleep(600 * time.Millisecond) // 覆盖防抖(100ms) + 脚本执行
		if countRuns() == prev {
			break
		}
	}
	before := countRuns()

	// AGENTS.md 重写（自身产物）不应再次触发脚本。
	agents := filepath.Join(root, "AGENTS.md")
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(agents, []byte("# updated\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	time.Sleep(800 * time.Millisecond) // 覆盖多个防抖周期
	if after := countRuns(); after != before {
		t.Fatalf("AGENTS.md 重写不应触发同步: 执行次数 %d -> %d", before, after)
	}

	// ctx 取消后 Watch 应正常退出（返回 nil）。
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Watch 退出时报错: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ctx 取消后 Watch 未退出")
	}
}

// TestWatchMissingScript 脚本缺失时 Watch 应直接返回可操作的错误。
func TestWatchMissingScript(t *testing.T) {
	skipOnWindows(t)
	installFakeCodegraph(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir() // 无任何 generate-agents.sh

	err := Watch(context.Background(), WatchOptions{ProjectPath: root})
	if err == nil || !strings.Contains(err.Error(), "codegraph-kit") {
		t.Fatalf("Watch 应提示安装 codegraph-kit: %v", err)
	}
}

// TestWatchMissingCodegraph codegraph 缺失时 Watch 应直接报错。
func TestWatchMissingCodegraph(t *testing.T) {
	skipOnWindows(t)
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	err := Watch(context.Background(), WatchOptions{ProjectPath: root})
	if err == nil || !strings.Contains(err.Error(), "codegraph-stack") {
		t.Fatalf("Watch 应提示安装 codegraph-stack: %v", err)
	}
}
