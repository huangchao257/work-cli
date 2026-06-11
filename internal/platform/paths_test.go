package platform

import (
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
