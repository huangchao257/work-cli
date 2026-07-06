package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverEmpty(t *testing.T) {
	// 当插件目录为空时应返回空列表
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// 创建 ~/.work/plugins 但留空
	pluginsDir := filepath.Join(dir, ".work", "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	plugins, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Fatalf("空目录应返回空列表，实际为 %d 个", len(plugins))
	}
}

func TestDiscoverWithPlugin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	pluginsDir := filepath.Join(dir, ".work", "plugins", "my-plugin")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := `name: my-plugin
version: 1.0.0
description: 演示插件
command: /usr/bin/echo
`
	if err := os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 {
		t.Fatalf("应发现 1 个插件，实际为 %d 个", len(plugins))
	}
	if plugins[0].Name != "my-plugin" {
		t.Fatalf("插件名应为 my-plugin，实际为 %s", plugins[0].Name)
	}
	if plugins[0].Version != "1.0.0" {
		t.Fatalf("版本应为 1.0.0，实际为 %s", plugins[0].Version)
	}
	if plugins[0].Command != "/usr/bin/echo" {
		t.Fatalf("command 应为 /usr/bin/echo，实际为 %s", plugins[0].Command)
	}
}

func TestDiscoverNameMismatch(t *testing.T) {
	// 目录名与 manifest 中的 name 不一致时应被跳过
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	pluginsDir := filepath.Join(dir, ".work", "plugins", "dir-name")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := `name: other-name
version: 1.0.0
command: /bin/true
`
	if err := os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Fatalf("名称不匹配的插件应被跳过，实际发现了 %d 个", len(plugins))
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	pluginsDir := filepath.Join(dir, ".work", "plugins", "findable")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := `name: findable
version: 2.0.0
description: 可被查找的插件
command: /bin/true
`
	if err := os.WriteFile(filepath.Join(pluginsDir, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := Find("findable")
	if err != nil {
		t.Fatalf("应找到插件: %v", err)
	}
	if m.Name != "findable" {
		t.Fatalf("名称应为 findable，实际为 %s", m.Name)
	}
	if m.Version != "2.0.0" {
		t.Fatalf("版本应为 2.0.0，实际为 %s", m.Version)
	}
}

func TestFindNotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// 创建空的插件目录
	pluginsDir := filepath.Join(dir, ".work", "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Find("nonexistent")
	if err == nil {
		t.Fatal("查找不存在的插件应返回错误")
	}
}

func TestParseManifestInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.yaml")
	if err := os.WriteFile(path, []byte("not: [valid: yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := parseManifest(path)
	if err == nil {
		t.Fatal("无效 YAML 应返回错误")
	}
}

func TestParseManifestMissingFile(t *testing.T) {
	_, err := parseManifest("/nonexistent/path/plugin.yaml")
	if err == nil {
		t.Fatal("不存在的文件应返回错误")
	}
}
