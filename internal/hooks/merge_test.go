package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeCursorHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	entries := []SidecarEntry{{
		IDEEvent: "beforeShellExecution",
		Command:  "./hooks/work-telemetry/company-hooks/run-beforeshellexecution.sh",
		WorkID:   "work-telemetry",
	}}
	if err := MergeCursorHooks(path, entries); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), "beforeShellExecution") {
		t.Fatalf("missing event in %s", data)
	}
	if err := UnmergeCursorHooks(path); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(path)
	if contains(string(data2), "work-telemetry") {
		t.Fatalf("work hooks should be removed: %s", data2)
	}
}

func TestRedactPayload(t *testing.T) {
	raw := []byte(`{"prompt":"secret","tool":"Shell"}`)
	out, err := RedactPayload(raw, []string{"prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if out["prompt"] != "[redacted]" {
		t.Fatalf("expected redacted prompt, got %v", out["prompt"])
	}
}
