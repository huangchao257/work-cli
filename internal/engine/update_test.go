package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/huangchao257/work-cli/internal/source"
)

func TestUpdateUsesInstalledRecordName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_EXAMPLES_DIR", filepath.Join(wd, "..", "..", "examples"))

	ref, err := source.ParseInstallName("dev-kit")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Install(context.Background(), Options{
		Scope:  "user",
		IDEs:   []string{"cursor"},
		DryRun: false,
		Ref:    ref,
	}); err != nil {
		t.Fatal(err)
	}

	results, err := Update(context.Background(), "dev-kit", "user", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].Success || results[0].Name != "dev-kit" {
		t.Fatalf("unexpected update result: %+v", results)
	}
}
