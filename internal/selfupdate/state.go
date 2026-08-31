package selfupdate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/huangchao257/work-cli/internal/platform"
)

type checkState struct {
	LastCheck time.Time `json:"last_check"`
}

func statePath() (string, error) {
	dir, err := platform.WorkConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取 work 配置目录失败: %w", err)
	}
	return filepath.Join(dir, "self-update.json"), nil
}

// withStateLock 对状态文件加锁执行 fn 并返回结果。
// 用于防止多个 work 进程同时读写自更新状态文件导致竞争。
// 锁加在旁路 .lock 文件而非 self-update.json 本身：状态文件在
// saveCheckState 中被 rename 替换（换 inode），flock 绑定 inode，
// 锁在被覆盖的旧 inode 上对新 open 的进程不再互斥，锁被架空。
// 旁路锁文件自身从不被 rename/remove，inode 恒定。
func withStateLock(fn func() error) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建自更新状态目录失败: %w", err)
	}
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("打开自更新状态锁文件失败: %w", err)
	}
	defer f.Close()

	if err := platform.FlockLock(f, lockPath, platform.FlockEX); err != nil {
		return fmt.Errorf("获取自更新状态锁失败: %w", err)
	}
	defer func() { _ = platform.FlockUnlock(f) }()

	return fn()
}

func loadCheckState() (checkState, error) {
	path, err := statePath()
	if err != nil {
		return checkState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return checkState{}, nil
		}
		return checkState{}, fmt.Errorf("读取自更新状态文件失败: %w", err)
	}
	var st checkState
	if err := json.Unmarshal(data, &st); err != nil {
		// 损坏的状态文件视为未检查（零值）：下一次 markChecked 会原子覆盖
		// 修复该文件。上抛错误会让每条命令持续告警且自动更新被永久禁用
		// （错误路径永远走不到 markChecked），且该文件只是节流记录，无
		// 不可丢数据。
		return checkState{}, nil
	}
	return st, nil
}

func saveCheckState(st checkState) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建状态目录失败: %w", err)
	}
	data, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("编码状态失败: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".self-update-*.json")
	if err != nil {
		return fmt.Errorf("创建临时状态文件失败: %w", err)
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
		return fmt.Errorf("写入临时状态文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时状态文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("原子替换自更新状态文件失败: %w", err)
	}
	cleanup = false
	return nil
}

func shouldCheckNow(interval time.Duration, force bool) (bool, error) {
	if force {
		return true, nil
	}
	st, err := loadCheckState()
	if err != nil {
		return false, err
	}
	if st.LastCheck.IsZero() {
		return true, nil
	}
	return time.Since(st.LastCheck) >= interval, nil
}

// markChecked 标记自更新检查时间，使用文件锁防止多个 work 进程并发写入。
func markChecked() error {
	return withStateLock(func() error {
		return saveCheckState(checkState{LastCheck: time.Now()})
	})
}
