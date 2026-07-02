package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEvaluateEventFilter(t *testing.T) {
	policy := Policy{Rules: []Rule{
		{ID: "shell-deny", Event: "shell", Match: "rm\\s+-rf", Severity: High},
	}}
	cp := policy.Compile()
	events := []EventRecord{
		{EventID: "e1", AbstractEvent: "file_edit", Payload: map[string]any{"cmd": "rm -rf /"}},
		{EventID: "e2", AbstractEvent: "shell", Payload: map[string]any{"cmd": "rm -rf /"}},
	}
	got := Evaluate(events, cp)
	if len(got) != 1 || got[0].EventID != "e2" {
		t.Fatalf("期望仅 e2 命中，实际 %+v", got)
	}
}

func TestEvaluateMatchHitAndMiss(t *testing.T) {
	policy := Policy{Rules: []Rule{
		{ID: "deny-rm-rf", Match: "rm\\s+-rf", Severity: High},
	}}
	cp := policy.Compile()
	events := []EventRecord{
		{EventID: "hit", AbstractEvent: "shell", Payload: map[string]any{"command": "rm -rf /tmp"}},
		{EventID: "miss", AbstractEvent: "shell", Payload: map[string]any{"command": "ls -la"}},
	}
	got := Evaluate(events, cp)
	if len(got) != 1 || got[0].EventID != "hit" {
		t.Fatalf("期望仅 hit 命中，实际 %+v", got)
	}
	if got[0].Severity != High {
		t.Errorf("期望 severity high，实际 %s", got[0].Severity)
	}
}

func TestEvaluatePathRegexHitAndMiss(t *testing.T) {
	policy := Policy{Rules: []Rule{
		{ID: "sensitive-write", Event: "file_edit", PathRegex: "(/etc/|\\.env|credentials)", Severity: Medium},
	}}
	cp := policy.Compile()
	events := []EventRecord{
		{EventID: "sensitive", AbstractEvent: "file_edit", Payload: map[string]any{"path": "/etc/passwd"}},
		{EventID: "benign", AbstractEvent: "file_edit", Payload: map[string]any{"path": "/home/user/main.go"}},
	}
	got := Evaluate(events, cp)
	if len(got) != 1 || got[0].EventID != "sensitive" {
		t.Fatalf("期望仅 sensitive 命中，实际 %+v", got)
	}
}

func TestEvaluatePathRegexFallbackToText(t *testing.T) {
	// payload 无显式 path 字段时，path_regex 回退到整段文本匹配
	policy := Policy{Rules: []Rule{
		{ID: "cred-in-text", PathRegex: "credentials", Severity: Medium},
	}}
	cp := policy.Compile()
	events := []EventRecord{
		{EventID: "e1", AbstractEvent: "shell", Payload: map[string]any{"cmd": "cat credentials.yaml"}},
	}
	got := Evaluate(events, cp)
	if len(got) != 1 {
		t.Fatalf("期望回退文本匹配命中，实际 %+v", got)
	}
}

func TestEvaluateSeverityDefaultMedium(t *testing.T) {
	policy := Policy{Rules: []Rule{
		{ID: "no-sev", Match: "danger"},
	}}
	cp := policy.Compile()
	events := []EventRecord{
		{EventID: "e1", AbstractEvent: "shell", Payload: map[string]any{"cmd": "danger zone"}},
	}
	got := Evaluate(events, cp)
	if len(got) != 1 {
		t.Fatalf("期望命中，实际 %+v", got)
	}
	if got[0].Severity != Medium {
		t.Errorf("期望默认 severity medium，实际 %s", got[0].Severity)
	}
}

func TestEvaluateSinceFilter(t *testing.T) {
	// since 过滤由 ReadEvents 负责，此处验证 ReadEvents 的过滤效果。
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	path := filepath.Join(dir, "queue.jsonl")
	recent := now.Add(-1 * time.Hour).Format(time.RFC3339Nano)
	old := now.Add(-48 * time.Hour).Format(time.RFC3339Nano)
	lines := joinLines([]string{
		`{"event":{"event_id":"recent","timestamp":"` + recent + `","abstract_event":"shell","payload":{"v":"x"}},"uploaded_at":null}`,
		`{"event":{"event_id":"old","timestamp":"` + old + `","abstract_event":"shell","payload":{"v":"x"}},"uploaded_at":null}`,
	})
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	events, _, err := ReadEvents(path, now, 24*time.Hour)
	if err != nil {
		t.Fatalf("ReadEvents 失败: %v", err)
	}
	if len(events) != 1 || events[0].EventID != "recent" {
		t.Fatalf("期望 since 过滤后仅 recent 命中，实际 %+v", events)
	}
}

func TestEvaluateSinceZeroTimestampDroppedWhenSinceSet(t *testing.T) {
	// since>0 且时间戳为零 → ReadEvents 丢弃
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	path := filepath.Join(dir, "queue.jsonl")
	lines := `{"event":{"event_id":"no-ts","abstract_event":"shell","payload":{"v":"x"}},"uploaded_at":null}` + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	events, _, err := ReadEvents(path, now, 24*time.Hour)
	if err != nil {
		t.Fatalf("ReadEvents 失败: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("since>0 且时间戳为零应丢弃，实际 %+v", events)
	}
	// since==0 时不做过滤，零时间戳保留
	events, _, err = ReadEvents(path, now, 0)
	if err != nil {
		t.Fatalf("ReadEvents 失败: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("since==0 时应保留，实际 %+v", events)
	}
}

func TestEvaluateNoRulesPolicy(t *testing.T) {
	policy := Policy{}
	cp := policy.Compile()
	events := []EventRecord{
		{EventID: "e1", AbstractEvent: "shell", Payload: map[string]any{"cmd": "rm -rf /"}},
	}
	got := Evaluate(events, cp)
	if len(got) != 0 {
		t.Fatalf("无规则策略应无违规，实际 %+v", got)
	}
}

func TestEvaluateMatchAndPathEitherHits(t *testing.T) {
	// 同一规则同时有 match 与 path_regex，任一命中即违规
	policy := Policy{Rules: []Rule{
		{ID: "dual", Match: "rm\\s+-rf", PathRegex: "/etc/", Severity: High},
	}}
	cp := policy.Compile()
	events := []EventRecord{
		{EventID: "by-match", AbstractEvent: "shell", Payload: map[string]any{"cmd": "rm -rf /home"}},
		{EventID: "by-path", AbstractEvent: "file_edit", Payload: map[string]any{"path": "/etc/shadow"}},
		{EventID: "neither", AbstractEvent: "shell", Payload: map[string]any{"cmd": "ls", "path": "/home"}},
	}
	got := Evaluate(events, cp)
	if len(got) != 2 {
		t.Fatalf("期望 2 条命中（match 或 path），实际 %+v", got)
	}
}

func TestReadEventsFileNotExist(t *testing.T) {
	events, warnings, err := ReadEvents(filepath.Join(t.TempDir(), "missing.jsonl"), time.Now(), 0)
	if err != nil {
		t.Fatalf("文件不存在应返回 nil 错误: %v", err)
	}
	if events != nil || warnings != nil {
		t.Fatalf("期望 nil,nil，实际 %+v / %+v", events, warnings)
	}
}

func TestReadEventsDecodesAndFilters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "queue.jsonl")
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-1 * time.Hour).Format(time.RFC3339Nano)
	old := now.Add(-48 * time.Hour).Format(time.RFC3339Nano)
	lines := []string{
		`{"event":{"event_id":"e1","timestamp":"` + recent + `","abstract_event":"shell","payload":{"cmd":"ls"}},"uploaded_at":null}`,
		`this is not json`,
		`{"event":{"event_id":"e2","timestamp":"` + old + `","abstract_event":"shell","payload":{"cmd":"ls"}},"uploaded_at":null}`,
	}
	if err := os.WriteFile(path, []byte(joinLines(lines)), 0o600); err != nil {
		t.Fatal(err)
	}
	events, warnings, err := ReadEvents(path, now, 24*time.Hour)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if len(events) != 1 || events[0].EventID != "e1" {
		t.Fatalf("期望仅 e1 通过 since 过滤，实际 %+v", events)
	}
	if len(warnings) != 1 {
		t.Fatalf("期望 1 条解码 warning，实际 %+v", warnings)
	}
}

func TestReadEventsNoSinceKeepsAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "queue.jsonl")
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour).Format(time.RFC3339Nano)
	lines := []string{
		`{"event":{"event_id":"e1","timestamp":"` + old + `","abstract_event":"shell","payload":{}},"uploaded_at":null}`,
	}
	if err := os.WriteFile(path, []byte(joinLines(lines)), 0o600); err != nil {
		t.Fatal(err)
	}
	events, _, err := ReadEvents(path, now, 0)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("since==0 应保留全部，实际 %+v", events)
	}
}

// joinLines 用换行拼接多行（每行不带尾随换行，拼接时加 \n）。
func joinLines(lines []string) string {
	out := make([]byte, 0, len(lines)*64)
	for _, l := range lines {
		out = append(out, l...)
		out = append(out, '\n')
	}
	return string(out)
}
