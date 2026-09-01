package hooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// isolateTelemetryDir 将 ~/.work 指向临时目录，使队列/状态/死信文件互不干扰。
func isolateTelemetryDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func writeQueueEntries(t *testing.T, entries []QueueEntry) {
	t.Helper()
	path, err := QueuePath()
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, e := range entries {
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(b)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readQueueRaw 读取队列文件全部条目，不做退避/已上传过滤
// （ReadPending 会过滤 RetryAfter 未到期与已上传的条目，无法用于断言保留语义）。
func readQueueRaw(t *testing.T) []QueueEntry {
	t.Helper()
	path, err := QueuePath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []QueueEntry
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e QueueEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("队列存在无法解析的行: %q", line)
		}
		out = append(out, e)
	}
	return out
}

// readPendingVisible 读取当前可上报（退避已过、未上传）的队列条目。
func readPendingVisible(t *testing.T) []QueueEntry {
	t.Helper()
	pending, err := ReadPending()
	if err != nil {
		t.Fatal(err)
	}
	return pending
}

func readDeadLetter(t *testing.T) []QueueEntry {
	t.Helper()
	path, err := DeadLetterPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []QueueEntry
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e QueueEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("dead_letter 存在无法解析的行: %v", err)
		}
		out = append(out, e)
	}
	return out
}

func queueEntry(id string, retry int) QueueEntry {
	return QueueEntry{
		Event: EventRecord{
			EventID: id,
			Payload: map[string]any{},
		},
		RetryCount: retry,
		LastError:  "boom",
	}
}

// TestDropFailedRemovesExhaustedEntries 验证超限事件被移入死信、未超限事件保留、
// 上传完成的事件不受影响、无法解析的行原样保留。
func TestDropFailedRemovesExhaustedEntries(t *testing.T) {
	isolateTelemetryDir(t)
	future := time.Now().UTC().Add(time.Hour)
	past := time.Now().UTC().Add(-time.Hour)
	uploaded := time.Now().UTC().Add(-time.Minute)

	writeQueueEntries(t, []QueueEntry{
		queueEntry("fresh", 0), // 从未失败，保留
		func() QueueEntry { // 失败但未达上限（退避未到），保留
			e := queueEntry("retrying", 2)
			e.RetryAfter = &future
			return e
		}(),
		func() QueueEntry { // 失败且退避已过但未达上限，保留
			e := queueEntry("backoff-over", 2)
			e.RetryAfter = &past
			return e
		}(),
		func() QueueEntry { // 已上传，即使 retry_count 高也保留
			e := queueEntry("uploaded", 9)
			e.UploadedAt = &uploaded
			return e
		}(),
		queueEntry("exhausted", 3),  // 达到上限，移入死信
		queueEntry("over-limit", 7), // 超过上限，移入死信
	})

	dropped, err := DropFailed(3)
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 2 {
		t.Fatalf("dropped = %d, want 2", dropped)
	}

	pending := readQueueRaw(t)
	ids := map[string]bool{}
	for _, e := range pending {
		ids[e.Event.EventID] = true
	}
	for _, want := range []string{"fresh", "retrying", "backoff-over", "uploaded"} {
		if !ids[want] {
			t.Errorf("事件 %s 应保留在队列中，剩余: %v", want, ids)
		}
	}
	for _, gone := range []string{"exhausted", "over-limit"} {
		if ids[gone] {
			t.Errorf("事件 %s 应被移除", gone)
		}
	}

	dead := readDeadLetter(t)
	if len(dead) != 2 {
		t.Fatalf("dead_letter 条数 = %d, want 2", len(dead))
	}
	deadIDs := map[string]bool{}
	for _, e := range dead {
		deadIDs[e.Event.EventID] = true
	}
	if !deadIDs["exhausted"] || !deadIDs["over-limit"] {
		t.Errorf("死信内容不符: %v", deadIDs)
	}
	// 保留可观测错误：移入死信的事件携带最后一次失败原因
	for _, e := range dead {
		if e.LastError != "boom" {
			t.Errorf("死信条目 %s 丢失 last_error: %q", e.Event.EventID, e.LastError)
		}
	}
}

// TestDropFailedNegativeMaxRetriesClampedToZero 验证 maxRetries=0 时立即清理
// 所有已失败过（RetryCount>0）的事件，但从未失败的事件不受影响。
func TestDropFailedNegativeMaxRetriesClampedToZero(t *testing.T) {
	isolateTelemetryDir(t)
	writeQueueEntries(t, []QueueEntry{
		queueEntry("fresh", 0),
		queueEntry("failed-once", 1),
	})

	dropped, err := DropFailed(0)
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 1 {
		t.Fatalf("dropped = %d, want 1", dropped)
	}
	pending := readQueueRaw(t)
	if len(pending) != 1 || pending[0].Event.EventID != "fresh" {
		t.Fatalf("剩余队列不符: %+v", pending)
	}
	if dead := readDeadLetter(t); len(dead) != 1 || dead[0].Event.EventID != "failed-once" {
		t.Fatalf("死信不符: %+v", dead)
	}
}

// TestDropFailedPersistsCorruptLines 验证无法解析的行原样保留在队列中，
// 不丢失也不进入死信。
func TestDropFailedPersistsCorruptLines(t *testing.T) {
	isolateTelemetryDir(t)
	path, err := QueuePath()
	if err != nil {
		t.Fatal(err)
	}
	valid, _ := json.Marshal(queueEntry("exhausted", 5))
	corrupt := []byte(`{"event":{"event_id":`) // 截断的 JSON
	content := string(valid) + "\n" + string(corrupt) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	dropped, err := DropFailed(3)
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 1 {
		t.Fatalf("dropped = %d, want 1", dropped)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), string(corrupt)) {
		t.Fatalf("无法解析的行应原样保留: %s", data)
	}
	if strings.Contains(string(data), "exhausted") {
		t.Fatalf("超限事件应被移除: %s", data)
	}
}

// TestDropFailedIdempotent 验证重复调用不重复写死信。
func TestDropFailedIdempotent(t *testing.T) {
	isolateTelemetryDir(t)
	writeQueueEntries(t, []QueueEntry{queueEntry("exhausted", 3)})

	if _, err := DropFailed(3); err != nil {
		t.Fatal(err)
	}
	dropped, err := DropFailed(3)
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 0 {
		t.Fatalf("第二次调用 dropped = %d, want 0", dropped)
	}
	if dead := readDeadLetter(t); len(dead) != 1 {
		t.Fatalf("死信应只有 1 条, got %d", len(dead))
	}
}

// TestDropFailedUpdatesPendingCount 验证清理后同步状态的 pending_count 被更新。
func TestDropFailedUpdatesPendingCount(t *testing.T) {
	isolateTelemetryDir(t)
	writeQueueEntries(t, []QueueEntry{
		queueEntry("keep", 0),
		queueEntry("exhausted", 3),
	})

	if _, err := DropFailed(3); err != nil {
		t.Fatal(err)
	}
	st, err := LoadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	if st.PendingCount != 1 {
		t.Fatalf("pending_count = %d, want 1", st.PendingCount)
	}
}

// TestDropFailedDeadLetterPermissions 验证死信文件权限为 0600（与队列一致，
// 脱敏后的事件 payload 仍不应被其他用户读取）。
func TestDropFailedDeadLetterPermissions(t *testing.T) {
	isolateTelemetryDir(t)
	writeQueueEntries(t, []QueueEntry{queueEntry("exhausted", 3)})

	if _, err := DropFailed(3); err != nil {
		t.Fatal(err)
	}
	deadPath, err := DeadLetterPath()
	if err != nil {
		t.Fatal(err)
	}
	di, err := os.Stat(deadPath)
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != 0o600 {
		t.Fatalf("死信文件权限 = %o, want 600", di.Mode().Perm())
	}
}

// newSyncTestServer 返回一个上报服务器与指向它的配置。
func newSyncTestServer(t *testing.T, handler http.HandlerFunc) (TelemetryConfig, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	enabled := true
	cfg := TelemetryConfig{
		Enabled:    &enabled,
		URL:        srv.URL,
		BatchSize:  50,
		MaxRetries: 2,
	}
	return cfg, srv
}

// TestSyncDropsExhaustedEventsBeforeUpload 验证 sync 会先清理超限事件：
// 已达 max_retries 的事件不再被上传，直接进入死信。
func TestSyncDropsExhaustedEventsBeforeUpload(t *testing.T) {
	isolateTelemetryDir(t)
	var uploaded atomic.Int64
	cfg, _ := newSyncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		uploaded.Add(1)
		w.WriteHeader(http.StatusAccepted)
	})
	writeQueueEntries(t, []QueueEntry{
		queueEntry("exhausted", 2), // 达到上限
		queueEntry("healthy", 0),
	})

	if err := Sync(cfg); err != nil {
		t.Fatal(err)
	}
	if got := uploaded.Load(); got != 1 {
		t.Fatalf("上传批次次数 = %d, want 1", got)
	}
	if dead := readDeadLetter(t); len(dead) != 1 || dead[0].Event.EventID != "exhausted" {
		t.Fatalf("死信不符: %+v", dead)
	}
	pending := readPendingVisible(t)
	if len(pending) != 0 {
		t.Fatalf("全部上传后队列应为空, got %+v", pending)
	}
}

// TestSyncRetryThenDrop 验证完整生命周期：失败重试递增 RetryCount，
// 达到上限后的下一次 sync 将事件移入死信，队列不再积压。
func TestSyncRetryThenDrop(t *testing.T) {
	isolateTelemetryDir(t)
	cfg, _ := newSyncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	// MaxRetries=2：两次失败后第三次 sync 前被清理
	cfg.MaxRetries = 2
	writeQueueEntries(t, []QueueEntry{queueEntry("doomed", 0)})

	// 第一次失败：RetryCount 0 -> 1
	if err := Sync(cfg); err == nil {
		t.Fatal("服务端 500 时 sync 应失败")
	}
	pending := readQueueRaw(t)
	if len(pending) != 1 || pending[0].RetryCount != 1 {
		t.Fatalf("失败后 RetryCount = %+v, want 1", pending)
	}
	if dead := readDeadLetter(t); len(dead) != 0 {
		t.Fatalf("未达上限不应进死信: %+v", dead)
	}

	// 第二次失败：RetryCount 1 -> 2。首次失败会写入 2s 退避，
	// 此处将 RetryAfter 拨回过期以模拟退避窗口结束后再次尝试。
	{
		p := time.Now().UTC().Add(-time.Minute)
		if err := rewriteQueue(func(e *QueueEntry) bool {
			if e.Event.EventID == "doomed" {
				e.RetryAfter = &p
				return true
			}
			return false
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := Sync(cfg); err == nil {
		t.Fatal("服务端 500 时 sync 应失败")
	}
	pending = readQueueRaw(t)
	if len(pending) != 1 || pending[0].RetryCount != 2 {
		t.Fatalf("二次失败后 RetryCount = %+v, want 2", pending)
	}

	// 第三次 sync：入口先清理，RetryCount=2 >= MaxRetries=2，事件进死信，
	// 队列为空，不发起上传
	if err := Sync(cfg); err != nil {
		t.Fatal(err)
	}
	if dead := readDeadLetter(t); len(dead) != 1 || dead[0].Event.EventID != "doomed" {
		t.Fatalf("超限事件应进死信: %+v", dead)
	}
	if dead := readDeadLetter(t); dead[0].RetryCount != 2 || dead[0].LastError == "" {
		t.Fatalf("死信应保留重试次数与错误: %+v", dead[0])
	}
	if pending := readQueueRaw(t); len(pending) != 0 {
		t.Fatalf("队列应被清空: %+v", pending)
	}
	st, err := LoadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	if st.PendingCount != 0 {
		t.Fatalf("pending_count = %d, want 0", st.PendingCount)
	}
}

// TestSyncDefaultMaxRetries 验证 MaxRetries<=0 时回退默认 10。
func TestSyncDefaultMaxRetries(t *testing.T) {
	isolateTelemetryDir(t)
	cfg, _ := newSyncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	cfg.MaxRetries = 0
	writeQueueEntries(t, []QueueEntry{queueEntry("doomed", 9)})

	if err := Sync(cfg); err == nil {
		t.Fatal("服务端 500 时 sync 应失败")
	}
	// RetryCount 9 -> 10，下一次 sync 前被清理（默认上限 10）
	if err := Sync(cfg); err != nil {
		t.Fatal(err)
	}
	if dead := readDeadLetter(t); len(dead) != 1 {
		t.Fatalf("默认上限 10 下超限事件应进死信: %+v", dead)
	}
}

// TestSyncDisabledSkipsDrop 验证 telemetry 禁用时 sync 直接报错，
// 不做清理（避免禁用状态下悄悄改写队列）。
func TestSyncDisabledSkipsDrop(t *testing.T) {
	isolateTelemetryDir(t)
	disabled := false
	cfg := TelemetryConfig{Enabled: &disabled, URL: "https://example.com", MaxRetries: 3}
	writeQueueEntries(t, []QueueEntry{queueEntry("exhausted", 5)})

	if err := Sync(cfg); err == nil {
		t.Fatal("telemetry 禁用时 sync 应报错")
	}
	if dead := readDeadLetter(t); len(dead) != 0 {
		t.Fatalf("禁用时不应清理队列: %+v", dead)
	}
	if pending := readQueueRaw(t); len(pending) != 1 {
		t.Fatalf("禁用时队列应保持不变: %+v", pending)
	}
}

// TestSyncDropsOnlyExhaustedMultiEntry 验证清理粒度是单事件而非整批：
// 同批中未超限事件仍正常上传，超限事件进死信。
func TestSyncDropsOnlyExhaustedMultiEntry(t *testing.T) {
	isolateTelemetryDir(t)
	var mu sync.Mutex
	seen := map[string]bool{}
	cfg, _ := newSyncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body uploadBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		for _, e := range body.Events {
			seen[e.EventID] = true
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	writeQueueEntries(t, []QueueEntry{
		queueEntry("exhausted", 2),
		queueEntry("healthy", 0),
	})

	if err := Sync(cfg); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen["exhausted"] {
		t.Fatal("超限事件不应被上传")
	}
	if !seen["healthy"] {
		t.Fatal("未超限事件应被上传")
	}
}

// TestSyncCleanupDoesNotBlockCancellation 验证清理路径在 ctx 已取消时仍能完成
// （文件操作不受 ctx 约束），且 sync 语义不变。
func TestSyncCleanupDoesNotBlockCancellation(t *testing.T) {
	isolateTelemetryDir(t)
	cfg, _ := newSyncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	writeQueueEntries(t, []QueueEntry{queueEntry("exhausted", 5)})
	cfg.MaxRetries = 2

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预先取消：上传会失败，但清理应已完成
	_ = SyncWithContext(ctx, cfg)

	// 清理发生在上传之前，即使上传被取消，死信也已写入
	if dead := readDeadLetter(t); len(dead) != 1 {
		t.Fatalf("取消上下文中清理仍应完成: %+v", dead)
	}
}

// TestSyncContextPassthroughFailedBatchRetainsError 验证失败批次的错误语义
// 未被清理逻辑破坏：LastError 仍写入 state.json，队列仍保留待重试。
func TestSyncContextPassthroughFailedBatchRetainsError(t *testing.T) {
	isolateTelemetryDir(t)
	cfg, _ := newSyncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	cfg.MaxRetries = 10
	writeQueueEntries(t, []QueueEntry{queueEntry("healthy", 0)})

	if err := Sync(cfg); err == nil {
		t.Fatal("503 时 sync 应失败")
	}
	st, err := LoadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	if st.LastError == "" {
		t.Fatal("LastError 应保留可观测错误")
	}
	pending := readQueueRaw(t)
	if len(pending) != 1 || pending[0].RetryCount != 1 {
		t.Fatalf("失败后事件应保留且 RetryCount=1: %+v", pending)
	}
}

// TestSyncEmptyQueueNoDeadLetter 验证空队列时 sync 不产生死信文件。
func TestSyncEmptyQueueNoDeadLetter(t *testing.T) {
	isolateTelemetryDir(t)
	cfg, _ := newSyncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	if err := Sync(cfg); err != nil {
		t.Fatal(err)
	}
	if dead := readDeadLetter(t); len(dead) != 0 {
		t.Fatalf("空队列不应产生死信: %+v", dead)
	}
}

// TestReportPassthroughUnaffectedByMaxRetries 验证 stdin 透传契约在超限清理
// 路径下不变：report 始终把 stdin 原样回写 stdout，同步失败不影响透传。
func TestReportPassthroughUnaffectedByMaxRetries(t *testing.T) {
	isolateTelemetryDir(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	// 预置一个超限事件，report 触发的 sync 会先清理它
	writeQueueEntries(t, []QueueEntry{queueEntry("exhausted", 10)})

	enabled := true
	cfg := TelemetryConfig{Enabled: &enabled, URL: srv.URL, MaxRetries: 10}
	// 手动调用 sync 验证清理语义：RetryCount=10 已达上限，sync 入口即清理
	// （清理不依赖退避到期）。清理后队列为空、无需上传，sync 返回成功。
	if err := Sync(cfg); err != nil {
		t.Fatalf("清理后空队列的 sync 应成功: %v", err)
	}
	if dead := readDeadLetter(t); len(dead) != 1 {
		t.Fatalf("超限事件应被清理进死信: %+v", dead)
	}

	// report 本身：stdin 透传契约
	raw := `{"session_id":"s-1","prompt":"secret-prompt"}`
	var out strings.Builder
	res, err := Report(ReportInput{
		IDE:         "cursor",
		IDEEvent:    "beforeShellExecution",
		HooksKit:    "",
		Scope:       "user",
		Stdin:       strings.NewReader(raw),
		Stdout:      &out,
		TriggerSync: true,
	})
	if err != nil {
		t.Fatalf("report 应成功（队列写入成功即成功）: %v", err)
	}
	if out.String() != raw {
		t.Fatalf("stdin 透传被破坏: got %q want %q", out.String(), raw)
	}
	if res.EventID == "" {
		t.Fatal("report 应返回事件 ID")
	}
	// 脱敏契约：payload 中不得出现明文 prompt
	rec := res.Record
	if s, ok := rec.Payload["prompt"].(string); ok && strings.Contains(s, "secret-prompt") {
		t.Fatalf("prompt 未脱敏: %q", s)
	}
}

// TestMaxRetriesFromConfigFile 验证用户配置文件中的 max_retries 生效。
func TestMaxRetriesFromConfigFile(t *testing.T) {
	home := isolateTelemetryDir(t)
	cfgDir := filepath.Join(home, ".work")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "telemetry:\n  max_retries: 1\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadTelemetryConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxRetries != 1 {
		t.Fatalf("max_retries = %d, want 1", cfg.MaxRetries)
	}
	if maxRetriesValue(cfg) != 1 {
		t.Fatalf("maxRetriesValue = %d, want 1", maxRetriesValue(cfg))
	}
}

// TestMaxRetriesInvalidFallsBackToDefault 验证非正 max_retries 不被合并、
// 回退默认 10（mergeTelemetryConfig 只接受正值，原始默认值 10 保持不变）。
func TestMaxRetriesInvalidFallsBackToDefault(t *testing.T) {
	isolateTelemetryDir(t)
	home := os.Getenv("HOME")
	cfgDir := filepath.Join(home, ".work")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "telemetry:\n  max_retries: -3\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadTelemetryConfig()
	if err != nil {
		t.Fatal(err)
	}
	// 非正值不覆盖默认：LoadTelemetryConfig 直接返回默认 10
	if cfg.MaxRetries != 10 {
		t.Fatalf("非正 max_retries 应回退默认: got %d want 10", cfg.MaxRetries)
	}
	if got := maxRetriesValue(cfg); got != 10 {
		t.Fatalf("maxRetriesValue = %d, want 10", got)
	}
	// 零值配置（未加载文件）同样回退默认
	enabled := true
	cfg2 := TelemetryConfig{Enabled: &enabled}
	if got := maxRetriesValue(cfg2); got != 10 {
		t.Fatalf("零值 maxRetriesValue = %d, want 10", got)
	}
	// 显式负值也回退（防御：绕过 merge 直接构造的配置）
	cfg2.MaxRetries = -1
	if got := maxRetriesValue(cfg2); got != 10 {
		t.Fatalf("负值 maxRetriesValue = %d, want 10", got)
	}
}

// TestDropFailedConcurrentAppend 验证清理与并发追加互斥：
// 清理期间的 append 不会丢失（旁路锁保护）。
func TestDropFailedConcurrentAppend(t *testing.T) {
	isolateTelemetryDir(t)
	writeQueueEntries(t, []QueueEntry{queueEntry("exhausted", 5)})

	done := make(chan error, 1)
	go func() {
		done <- AppendQueue(EventRecord{EventID: "concurrent", Payload: map[string]any{}})
	}()
	if _, err := DropFailed(2); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	pending := readQueueRaw(t)
	ids := map[string]bool{}
	for _, e := range pending {
		ids[e.Event.EventID] = true
	}
	if !ids["concurrent"] {
		t.Fatalf("并发追加的事件不应丢失: %v", ids)
	}
	if dead := readDeadLetter(t); len(dead) != 1 || dead[0].Event.EventID != "exhausted" {
		t.Fatalf("死信不符: %+v", dead)
	}
}
