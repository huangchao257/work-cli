package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveCommandDefaultRun(t *testing.T) {
	spec := CommandSpec{
		Run: "curl -fsSL https://example.com/install.sh | sh",
	}
	cmd, err := ResolveCommand(spec)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "curl -fsSL https://example.com/install.sh | sh" {
		t.Fatalf("expected default run, got %q", cmd)
	}
}

func TestResolveCommandPlatformSpecific(t *testing.T) {
	spec := CommandSpec{
		Run: "default-command",
		Platforms: map[string]PlatformCommand{
			runtime.GOOS: {Run: "platform-specific-command"},
		},
	}
	cmd, err := ResolveCommand(spec)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "platform-specific-command" {
		t.Fatalf("expected platform-specific command, got %q", cmd)
	}
}

func TestResolveCommandPlatformFallback(t *testing.T) {
	// 为不匹配的平台设置命令，期望 fallback 到默认 Run
	spec := CommandSpec{
		Run: "default-command",
		Platforms: map[string]PlatformCommand{
			"nonexistent-os": {Run: "never-used"},
		},
	}
	cmd, err := ResolveCommand(spec)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "default-command" {
		t.Fatalf("expected default run, got %q", cmd)
	}
}

func TestResolveCommandEmptyPlatformRun(t *testing.T) {
	// 平台条目存在但 Run 为空，应 fallback 到默认
	spec := CommandSpec{
		Run: "default-command",
		Platforms: map[string]PlatformCommand{
			runtime.GOOS: {Run: ""},
		},
	}
	cmd, err := ResolveCommand(spec)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "default-command" {
		t.Fatalf("expected default run for empty platform command, got %q", cmd)
	}
}

func TestResolveCommandNoCommand(t *testing.T) {
	spec := CommandSpec{
		Platforms: map[string]PlatformCommand{
			"nonexistent-os": {Run: "unreachable"},
		},
	}
	_, err := ResolveCommand(spec)
	if err == nil {
		t.Fatal("expected error when no command is available")
	}
}

func TestResolveCommandEmptySpec(t *testing.T) {
	spec := CommandSpec{}
	_, err := ResolveCommand(spec)
	if err == nil {
		t.Fatal("expected error for empty spec")
	}
}

func TestValidateManifest(t *testing.T) {
	tests := []struct {
		name    string
		m       Manifest
		wantErr bool
	}{
		{
			name:    "最小有效 manifest",
			m:       Manifest{Type: "cli", Name: "test-cli", Version: "1.0.0", Install: CommandSpec{Run: "echo hi"}},
			wantErr: false,
		},
		{
			name:    "有效 manifest 带平台",
			m:       Manifest{Type: "cli", Name: "test-cli", Version: "1.0.0", Install: CommandSpec{Platforms: map[string]PlatformCommand{"linux": {Run: "echo linux"}}}},
			wantErr: false,
		},
		{
			name:    "错误 type",
			m:       Manifest{Type: "hooks", Name: "test", Version: "1.0.0", Install: CommandSpec{Run: "echo hi"}},
			wantErr: true,
		},
		{
			name:    "空 type",
			m:       Manifest{Type: "", Name: "test", Version: "1.0.0", Install: CommandSpec{Run: "echo hi"}},
			wantErr: true,
		},
		{
			name:    "缺少 name",
			m:       Manifest{Type: "cli", Name: "", Version: "1.0.0", Install: CommandSpec{Run: "echo hi"}},
			wantErr: true,
		},
		{
			name:    "缺少 version",
			m:       Manifest{Type: "cli", Name: "test", Version: "", Install: CommandSpec{Run: "echo hi"}},
			wantErr: true,
		},
		{
			name:    "缺少 install 命令",
			m:       Manifest{Type: "cli", Name: "test", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "install run 和 platforms 都为空",
			m:       Manifest{Type: "cli", Name: "test", Version: "1.0.0", Install: CommandSpec{}},
			wantErr: true,
		},
		{
			name:    "仅空格 name",
			m:       Manifest{Type: "cli", Name: "   ", Version: "1.0.0", Install: CommandSpec{Run: "echo hi"}},
			wantErr: true,
		},
		{
			name:    "仅空格 version",
			m:       Manifest{Type: "cli", Name: "test", Version: "   ", Install: CommandSpec{Run: "echo hi"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.m)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestFileName)

	content := `type: cli
name: test-cli
version: "1.0.0"
description: 测试 CLI
install:
  run: echo "hello world"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "test-cli" {
		t.Fatalf("expected name test-cli, got %q", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %q", m.Version)
	}
	if m.Install.Run != `echo "hello world"` {
		t.Fatalf("unexpected install command: %q", m.Install.Run)
	}
	if m.Description != "测试 CLI" {
		t.Fatalf("unexpected description: %q", m.Description)
	}
}

func TestParseFileNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestFileName)
	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseFileInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestFileName)
	if err := os.WriteFile(path, []byte("{{{invalid yaml!!!"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestFileName)

	content := `type: cli
name: dir-cli
version: "2.0.0"
install:
  run: echo dir
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "dir-cli" {
		t.Fatalf("expected name dir-cli, got %q", m.Name)
	}
}

func TestDefaultShell(t *testing.T) {
	shell, flag := defaultShell()
	if runtime.GOOS == "windows" {
		if shell != "cmd.exe" || flag != "/C" {
			t.Fatalf("expected cmd.exe /C, got %s %s", shell, flag)
		}
	} else {
		if shell != "sh" || flag != "-c" {
			t.Fatalf("expected sh -c, got %s %s", shell, flag)
		}
	}
}
