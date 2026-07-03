package hooks

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EventRecord struct {
	EventID         string         `json:"event_id"`
	Timestamp       time.Time      `json:"timestamp"`
	IDE             string         `json:"ide"`
	AbstractEvent   string         `json:"abstract_event"`
	IDEEvent        string         `json:"ide_event"`
	HooksKit        string         `json:"hooks_kit,omitempty"`
	HooksKitVersion string         `json:"hooks_kit_version,omitempty"`
	Scope           string         `json:"scope"`
	User            string         `json:"user"`
	MachineID       string         `json:"machine_id"`
	ProjectRoot     string         `json:"project_root,omitempty"`
	SessionID       string         `json:"session_id,omitempty"`
	Payload         map[string]any `json:"payload"`
}

type QueueEntry struct {
	Event      EventRecord `json:"event"`
	UploadedAt *time.Time  `json:"uploaded_at"`
	RetryCount int         `json:"retry_count"`
	LastError  string      `json:"last_error"`
	RetryAfter *time.Time  `json:"retry_after,omitempty"`
}

type SyncState struct {
	LastSync     *time.Time `json:"last_sync"`
	PendingCount int        `json:"pending_count"`
	LastError    string     `json:"last_error"`
}

func QueuePath() (string, error) {
	dir, err := TelemetryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "queue.jsonl"), nil
}

func StatePath() (string, error) {
	dir, err := TelemetryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

func AppendQueue(rec EventRecord) error {
	path, err := QueuePath()
	if err != nil {
		return err
	}
	entry := QueueEntry{Event: rec}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("编码队列条目失败: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("打开队列文件失败: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("写入队列文件失败: %w", err)
	}
	return updatePendingCount()
}

func ReadPending() ([]QueueEntry, error) {
	path, err := QueuePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取队列文件失败: %w", err)
	}
	var out []QueueEntry
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e QueueEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if e.UploadedAt != nil {
			continue
		}
		if e.RetryAfter != nil && e.RetryAfter.After(time.Now().UTC()) {
			continue
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

func MarkUploaded(eventIDs map[string]bool) error {
	path, err := QueuePath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取队列文件失败: %w", err)
	}
	now := time.Now().UTC()
	var lines []byte
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e QueueEntry
		if err := json.Unmarshal(line, &e); err != nil {
			lines = append(lines, append(line, '\n')...)
			continue
		}
		if eventIDs[e.Event.EventID] {
			e.UploadedAt = &now
			e.LastError = ""
		}
		b, _ := json.Marshal(e)
		lines = append(lines, append(b, '\n')...)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("扫描队列文件失败: %w", err)
	}
	if err := os.WriteFile(path, lines, 0o600); err != nil {
		return fmt.Errorf("写入队列文件失败: %w", err)
	}
	return updatePendingCount()
}

func RecordSyncError(eventID, msg string, retryAfter time.Time) error {
	path, err := QueuePath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取队列文件失败: %w", err)
	}
	var lines []byte
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Bytes()
		var e QueueEntry
		if err := json.Unmarshal(line, &e); err != nil {
			lines = append(lines, append(line, '\n')...)
			continue
		}
		if e.Event.EventID == eventID {
			e.RetryCount++
			e.LastError = msg
			t := retryAfter
			e.RetryAfter = &t
		}
		b, _ := json.Marshal(e)
		lines = append(lines, append(b, '\n')...)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("读取队列行失败: %w", err)
	}
	if err := os.WriteFile(path, lines, 0o600); err != nil {
		return fmt.Errorf("写入队列文件失败: %w", err)
	}
	return nil
}

func LoadSyncState() (SyncState, error) {
	path, err := StatePath()
	if err != nil {
		return SyncState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SyncState{}, nil
		}
		return SyncState{}, fmt.Errorf("读取同步状态文件失败: %w", err)
	}
	var st SyncState
	if err := json.Unmarshal(data, &st); err != nil {
		return SyncState{}, fmt.Errorf("解析同步状态文件失败: %w", err)
	}
	return st, nil
}

func SaveSyncState(st SyncState) error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("编码同步状态失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("写入同步状态文件失败: %w", err)
	}
	return nil
}

func updatePendingCount() error {
	pending, err := ReadPending()
	if err != nil {
		return fmt.Errorf("统计待上报条目失败: %w", err)
	}
	st, _ := LoadSyncState()
	st.PendingCount = len(pending)
	return SaveSyncState(st)
}

func CountPending() (int, error) {
	pending, err := ReadPending()
	if err != nil {
		return 0, err
	}
	return len(pending), nil
}
