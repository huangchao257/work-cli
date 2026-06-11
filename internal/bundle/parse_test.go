package bundle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDir(t *testing.T) {
	dir := t.TempDir()
	content := `name: test
version: 1.0.0
resources:
  skills:
    - id: s1
      source: ./skills/s1
`
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "test" || m.Version != "1.0.0" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}
