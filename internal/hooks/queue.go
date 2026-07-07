package hooks

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	sc := bufio.NewScanner(bytes.NewReader(data))
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
	now := time.Now().UTC()
	return rewriteQueue(func(e *QueueEntry) (rewrite bool) {
		if eventIDs[e.Event.EventID] {
			e.UploadedAt = &now
			e.LastError = ""
			return true
		}
		return false
	})
}

func RecordSyncError(eventID, msg string, retryAfter time.Time) error {
	return rewriteQueue(func(e *QueueEntry) (rewrite bool) {
		if e.Event.EventID == eventID {
			e.RetryCount++
			e.LastError = msg
			t := retryAfter
			e.RetryAfter = &t
			return true
		}
		return false
	})
}

// rewriteQueue 逐行处理队列文件，通过临时文件+rename实现原子写入，
// 避免将整个文件读入内存。只有被 mutate 修改的行才会重新marshal。
func rewriteQueue(mutate func(*QueueEntry) bool) error {
	path, err := QueuePath()
	if err != nil {
		return err
	}

	// 打开原文件用于读取
	in, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("打开队列文件失败: %w", err)
	}
	defer in.Close()

	// 在同目录创建临时文件用于写入
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".queue-*.jsonl")
	if err != nil {
		return fmt.Errorf("创建临时队列文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	var modified bool
	sc := bufio.NewScanner(in)
	// 增大scanner缓冲区以处理长行
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e QueueEntry
		if err := json.Unmarshal(line, &e); err != nil {
			// 无法解析的行原样保留
			if _, werr := tmp.Write(line); werr != nil {
				return fmt.Errorf("写入临时队列文件失败: %w", werr)
			}
			if _, werr := tmp.Write([]byte{'\n'}); werr != nil {
				return fmt.Errorf("写入临时队列文件失败: %w", werr)
			}
			continue
		}
		if mutate(&e) {
			modified = true
		}
		b, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("编码队列条目失败: %w", err)
		}
		if _, werr := tmp.Write(b); werr != nil {
			return fmt.Errorf("写入临时队列文件失败: %w", werr)
		}
		if _, werr := tmp.Write([]byte{'\n'}); werr != nil {
			return fmt.Errorf("写入临时队列文件失败: %w", werr)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("扫描队列文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时队列文件失败: %w", err)
	}
	if !modified {
		cleanup = true
		return nil
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("原子替换队列文件失败: %w", err)
	}
	cleanup = false
	// 尽力更新同步状态元数据；失败不阻塞（队列数据已正确持久化）。
	// 下一次 AppendQueue 或 ReadPending 调用会自动修正 PendingCount。
	_ = updatePendingCount()
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
