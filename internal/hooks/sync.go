package hooks

import (
	"bytes"
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
	if cfg.URL == "" {
		return fmt.Errorf("未配置 telemetry.url")
	}
	if !cfg.Enabled {
		return fmt.Errorf("telemetry 已禁用")
	}
	pending, err := ReadPending()
	if err != nil {
		return err
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
		if err := uploadBatch(cfg, batch); err != nil {
			lastErr = err
			backoff := time.Duration(1<<min(batch[0].RetryCount, 6)) * time.Second
			for _, e := range batch {
				_ = RecordSyncError(e.Event.EventID, err.Error(), time.Now().UTC().Add(backoff))
			}
			st, _ := LoadSyncState()
			st.LastError = err.Error()
			_ = SaveSyncState(st)
			return err
		}
		ids := map[string]bool{}
		for _, e := range batch {
			ids[e.Event.EventID] = true
		}
		if err := MarkUploaded(ids); err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	st, _ := LoadSyncState()
	st.LastSync = &now
	st.LastError = ""
	_ = SaveSyncState(st)
	return lastErr
}

func uploadBatch(cfg TelemetryConfig, batch []QueueEntry) error {
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
		return err
	}
	req, err := http.NewRequest(http.MethodPost, cfg.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("telemetry 返回 HTTP %d", resp.StatusCode)
}

func ShouldAutoSync(cfg TelemetryConfig) bool {
	if !cfg.Enabled || cfg.URL == "" {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
