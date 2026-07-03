package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkConfigDir(t *testing.T) {
	t.Setenv("HOME", "/tmp/testhome")
	dir, err := WorkConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/testhome", ".work")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
}

func TestConfigFilePath(t *testing.T) {
	t.Setenv("HOME", "/tmp/testhome")
	p, err := ConfigFilePath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/testhome", ".work", "config.yaml")
	if p != want {
		t.Fatalf("got %q want %q", p, want)
	}
}

func TestWorkSubDirCreatesDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dir, err := WorkSubDir("telemetry")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(tmp, ".work", "telemetry")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("目录未创建: %s (err=%v)", dir, err)
	}
}
