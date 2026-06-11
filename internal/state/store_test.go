package state

import (
	"path/filepath"
	"testing"
)

func TestStoreUpsertRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rec := BundleRecord{Name: "dev-kit", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "local:dev-kit"}
	if err := s.Upsert(rec); err != nil {
		t.Fatal(err)
	}
	got, err := s.Find("dev-kit", "user")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "1.0.0" {
		t.Fatalf("unexpected version: %s", got.Version)
	}
	if err := s.Remove("dev-kit", "user"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Find("dev-kit", "user"); err == nil {
		t.Fatal("expected not found after remove")
	}
}
