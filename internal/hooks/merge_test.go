package hooks

import (
	"os"
	"path/filepath"
	"strings"
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
	if err := MergeCursorHooks(path, "company-hooks", entries); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "beforeShellExecution") {
		t.Fatalf("missing event in %s", data)
	}
	if err := UnmergeCursorHooks(path, "company-hooks"); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(path)
	if strings.Contains(string(data2), "work-telemetry") {
		t.Fatalf("work hooks should be removed: %s", data2)
	}
}

// TestMergeCursorHooksKitIsolation 验证多套装隔离：merge 套装 B 不得删除套装 A 的条目，
// unmerge 套装 A 也不得删除套装 B 的条目（回归：安装第二个套装曾清空第一个的注册）。
func TestMergeCursorHooksKitIsolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	kitA := []SidecarEntry{{
		IDEEvent: "beforeShellExecution",
		Command:  ".cursor/hooks/work-telemetry/kit-a/run.sh",
		WorkID:   "work-telemetry",
	}}
	kitB := []SidecarEntry{{
		IDEEvent: "afterFileEdit",
		Command:  ".cursor/hooks/work-telemetry/kit-b/run.sh",
		WorkID:   "work-telemetry",
	}}
	if err := MergeCursorHooks(path, "kit-a", kitA); err != nil {
		t.Fatal(err)
	}
	if err := MergeCursorHooks(path, "kit-b", kitB); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "kit-a/run.sh") {
		t.Fatalf("kit-a entry was clobbered by kit-b merge: %s", data)
	}
	// 卸载 kit-a 不应动 kit-b
	if err := UnmergeCursorHooks(path, "kit-a"); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(path)
	if !strings.Contains(string(data2), "kit-b/run.sh") {
		t.Fatalf("kit-b entry was removed by kit-a uninstall: %s", data2)
	}
	if strings.Contains(string(data2), "kit-a/run.sh") {
		t.Fatalf("kit-a entry should be removed: %s", data2)
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
