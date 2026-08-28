package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHooksAuditJSONExitCode 验证 hooks audit --json 与 human 模式的违规退出码契约一致：
// 发现违规时 human 退出 1、--json 也必须退出 1（回归：--json 分支曾无条件返回 nil，
// CI 中 `work hooks audit --json` 有违规仍退出 0，审计形同虚设）。
func TestHooksAuditJSONExitCode(t *testing.T) {
	dir := t.TempDir()
	policy := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(policy, []byte("rules:\n  - id: deny-rm\n    event: shell\n    match: 'rm -rf'\n    severity: high\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 构造含违规事件的 queue.jsonl
	queue := filepath.Join(dir, "queue.jsonl")
	line := `{"event":{"event_id":"e1","timestamp":"2026-08-28T00:00:00Z","ide":"cursor","abstract_event":"shell","ide_event":"beforeShellExecution","user":"u","machine_id":"m","payload":{"command":"rm -rf /"}},"uploaded_at":null,"retry_count":0}` + "\n"
	if err := os.WriteFile(queue, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	resetGlobals := func() func() {
		savedJSON, savedPolicy, savedFile, savedSince := asJSON, auditPolicy, auditFile, auditSince
		return func() {
			asJSON, auditPolicy, auditFile, auditSince = savedJSON, savedPolicy, savedFile, savedSince
		}
	}

	t.Run("human 有违规退出 1", func(t *testing.T) {
		defer resetGlobals()()
		asJSON = false
		auditPolicy = policy
		auditFile = queue
		err := runHooksAudit(hooksAuditCmd, nil)
		if code := ExitCode(err); code != 1 {
			t.Fatalf("human 模式有违规应退出 1，got %d (err=%v)", code, err)
		}
	})

	t.Run("json 有违规也退出 1", func(t *testing.T) {
		defer resetGlobals()()
		asJSON = true
		auditPolicy = policy
		auditFile = queue
		err := runHooksAudit(hooksAuditCmd, nil)
		if code := ExitCode(err); code != 1 {
			t.Fatalf("--json 模式有违规应退出 1（与 human 一致），got %d (err=%v)", code, err)
		}
	})

	t.Run("json 无违规退出 0", func(t *testing.T) {
		defer resetGlobals()()
		asJSON = true
		auditPolicy = policy
		// 空队列
		empty := filepath.Join(dir, "empty.jsonl")
		if err := os.WriteFile(empty, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		auditFile = empty
		if err := runHooksAudit(hooksAuditCmd, nil); err != nil {
			t.Fatalf("无违规应成功，got %v", err)
		}
	})
}
