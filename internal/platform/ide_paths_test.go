package platform

import (
	"path/filepath"
	"testing"
)

func TestCursorUserSkillDir(t *testing.T) {
	t.Setenv("HOME", "/tmp/testhome")
	got, err := SkillDir(IDECursor, "user", "code-review")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/testhome", ".cursor", "skills", "code-review")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
