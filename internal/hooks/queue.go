package hooks

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/huangchao257/work-cli/internal/platform"
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

// queueLockPath 返回队列文件的稳定旁路锁路径。
// 锁加在 .lock 文件而非 queue.jsonl 本身：队列文件在 rewriteQueue 中被
// rename 替换——Windows 上 rename 覆盖正被本进程打开的文件必然失败，
// 且 Unix flock 绑定 inode，rename 换 inode 后锁互斥即失效。
// Append（追加）与 rewrite（读-改-写）都须持该锁，否则并发 hook 触发时
// rewrite 的 rename 会用不含并发追加内容的快照覆盖队列，无声丢失事件。
func queueLockPath() (string, error) {
	path, err := QueuePath()
	if err != nil {
		return "", err
	}
	return path + ".lock", nil
}

// withQueueLock 持队列独占锁执行 fn。
func withQueueLock(fn func() error) error {
	lockPath, err := queueLockPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return fmt.Errorf("创建队列目录失败: %w", err)
	}
	lf, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("打开队列锁文件失败: %w", err)
	}
	defer lf.Close()
	if err := platform.FlockLock(lf, lockPath, platform.FlockEX); err != nil {
		return fmt.Errorf("获取队列锁失败: %w", err)
	}
	return fn()
}

func AppendQueue(rec EventRecord) error {
	entry := QueueEntry{Event: rec}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("编码队列条目失败: %w", err)
	}
	return withQueueLock(func() error {
		path, err := QueuePath()
		if err != nil {
			return err
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
	})
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
	// 与 rewriteQueue 相同的长行缓冲（16 MiB）：超长事件 payload（未截断的
	// 数组内容）会超过默认 64K 上限，导致 sync/report/GetStatus 永久瘫痪。
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
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
// 全程持队列旁路锁（与 AppendQueue 互斥），且 rename 前已关闭原文件
// 读句柄——Windows 上 rename 覆盖正被本进程打开的文件必然失败。
func rewriteQueue(mutate func(*QueueEntry) bool) error {
	path, err := QueuePath()
	if err != nil {
		return err
	}

	return withQueueLock(func() error {
		// 先整体读入并关闭读句柄，再写临时文件与 rename。
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("读取队列文件失败: %w", err)
		}

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
		sc := bufio.NewScanner(bytes.NewReader(data))
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
	})
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

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sync-state-*.json")
	if err != nil {
		return fmt.Errorf("创建临时同步状态文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("写入临时同步状态文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时同步状态文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("原子替换同步状态文件失败: %w", err)
	}
	cleanup = false
	return nil
}

func updatePendingCount() error {
	pending, err := ReadPending()
	if err != nil {
		return fmt.Errorf("统计待上报条目失败: %w", err)
	}
	st, err := LoadSyncState()
	if err != nil {
		_ = SaveSyncState(SyncState{PendingCount: len(pending)})
	} else {
		st.PendingCount = len(pending)
		_ = SaveSyncState(st)
	}
	return nil
}

func CountPending() (int, error) {
	pending, err := ReadPending()
	if err != nil {
		return 0, err
	}
	return len(pending), nil
}
