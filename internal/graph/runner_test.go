package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupCursorHook(t *testing.T) {
	dir := t.TempDir()
	hook := filepath.Join(dir, "on-file-edit.sh")
	if err := os.WriteFile(hook, []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := setupCursorHook(dir, hook); err != nil {
		t.Fatal(err)
	}
	if !hookConfigured(dir) {
		t.Fatal("expected hook configured")
	}
	data, err := os.ReadFile(filepath.Join(dir, ".cursor", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), filepath.ToSlash(hook)) {
		t.Fatalf("hook path missing: %s", data)
	}
}
