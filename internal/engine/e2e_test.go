package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/huangchao257/work-cli/internal/source"
)

func TestE2EBundleInstallListUninstall(t *testing.T) {
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

	res, err := Install(context.Background(), Options{
		Scope:  "user",
		IDEs:   []string{"cursor"},
		DryRun: false,
		Ref:    ref,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.Name != "dev-kit" {
		t.Fatalf("unexpected result: %+v", res)
	}

	list, err := List("user", "bundle")
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list.Items))
	}

	_, err = Uninstall(context.Background(), "dev-kit", "user", false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestE2ECLIMockInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_EXAMPLES_DIR", filepath.Join(wd, "..", "..", "examples"))
	ref, err := source.ParseInstallName("openspec-mock")
	if err != nil {
		t.Fatal(err)
	}

	res, err := Install(context.Background(), Options{Ref: ref})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("install failed: %+v", res)
	}
	marker := filepath.Join(home, ".work", "openspec-mock-installed")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker not created: %v", err)
	}

	_, err = Uninstall(context.Background(), "openspec-mock", "user", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("marker should be removed")
	}
}

func TestE2EOpenSpecDryRun(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_EXAMPLES_DIR", filepath.Join(wd, "..", "..", "examples"))
	ref, err := source.ParseInstallName("openspec")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Install(context.Background(), Options{Ref: ref, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commands) == 0 || res.Commands[0] != "npm install -g @fission-ai/openspec@latest" {
		t.Fatalf("unexpected commands: %+v", res.Commands)
	}
}
