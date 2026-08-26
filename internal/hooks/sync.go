package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type uploadBody struct {
	Client        string        `json:"client"`
	ClientVersion string        `json:"client_version"`
	Events        []EventRecord `json:"events"`
}

func Sync(cfg TelemetryConfig) error {
	return syncWithContext(context.Background(), cfg)
}

// SyncWithContext 是 Sync 的 context 变体，允许调用方在超时时取消 HTTP 请求。
func SyncWithContext(ctx context.Context, cfg TelemetryConfig) error {
	return syncWithContext(ctx, cfg)
}

// syncWithContext 是 Sync 的 context 变体，允许调用方在超时时取消 HTTP 请求。
func syncWithContext(ctx context.Context, cfg TelemetryConfig) error {
	if cfg.URL == "" {
		return fmt.Errorf("未配置 telemetry.url")
	}
	if cfg.Enabled == nil || !*cfg.Enabled {
		return fmt.Errorf("telemetry 已禁用")
	}
	pending, err := ReadPending()
	if err != nil {
		return fmt.Errorf("读取待上报队列失败: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	var lastErr error
	for i := 0; i < len(pending); i += batchSize {
		end := i + batchSize
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[i:end]
		if err := uploadBatchWithContext(ctx, cfg, batch); err != nil {
			lastErr = err
			backoff := time.Duration(1<<min(batch[0].RetryCount, 6)) * time.Second
			for _, e := range batch {
				// 记录失败原因与退避时间，便于后续重试；写入失败只能忽略
				_ = RecordSyncError(e.Event.EventID, err.Error(), time.Now().UTC().Add(backoff))
			}
			st, err := LoadSyncState()
			if err != nil {
				_ = SaveSyncState(SyncState{LastError: err.Error()})
			} else {
				st.LastError = err.Error()
				_ = SaveSyncState(st)
			}
			return fmt.Errorf("上传 telemetry 批次失败: %w", err)
		}
		ids := map[string]bool{}
		for _, e := range batch {
			ids[e.Event.EventID] = true
		}
		if err := MarkUploaded(ids); err != nil {
			return fmt.Errorf("标记已上报事件失败: %w", err)
		}
	}

	now := time.Now().UTC()
	st, err := LoadSyncState()
	if err != nil {
		_ = SaveSyncState(SyncState{LastSync: &now})
	} else {
		st.LastSync = &now
		st.LastError = ""
		_ = SaveSyncState(st)
	}
	return lastErr
}

func uploadBatchWithContext(ctx context.Context, cfg TelemetryConfig, batch []QueueEntry) error {
	events := make([]EventRecord, 0, len(batch))
	for _, e := range batch {
		events = append(events, e.Event)
	}
	body := uploadBody{
		Client:        "work-cli",
		ClientVersion: clientVersion(),
		Events:        events,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("编码上报请求体失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("构造上报请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送 telemetry 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("telemetry 返回 HTTP %d", resp.StatusCode)
}

func ShouldAutoSync(cfg TelemetryConfig) bool {
	if cfg.Enabled == nil || !*cfg.Enabled || cfg.URL == "" {
		return false
	}
	st, err := LoadSyncState()
	if err != nil {
		return true
	}
	if st.LastSync == nil {
		return true
	}
	return time.Since(*st.LastSync) >= cfg.SyncIntervalDuration()
}

func clientVersion() string {
	if v := os.Getenv("WORK_CLI_VERSION"); v != "" {
		return v
	}
	return "dev"
}

func SyncFromEnv() error {
	cfg, err := LoadTelemetryConfig()
	if err != nil {
		return err
	}
	if u := os.Getenv("WORK_TELEMETRY_URL"); u != "" {
		cfg.URL = u
	}
	return Sync(cfg)
}
