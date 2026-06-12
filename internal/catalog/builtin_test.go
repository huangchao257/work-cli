package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCodegraphStack(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := findExamplesUp(wd)
	if root == "" {
		t.Skip("examples 目录不可用")
	}
	t.Setenv("WORK_EXAMPLES_DIR", root)

	path, ok := Resolve("codegraph-stack")
	if !ok {
		t.Fatal("expected codegraph-stack")
	}
	if _, err := os.Stat(filepath.Join(path, "installer.yaml")); err != nil {
		t.Fatalf("missing installer.yaml: %v", err)
	}
}
