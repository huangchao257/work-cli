package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const workTelemetryDir = "work-telemetry"

type SidecarEntry struct {
	IDEEvent string `json:"ide_event"`
	Matcher  string `json:"matcher,omitempty"`
	Command  string `json:"command"`
	WorkID   string `json:"work_id"`
}

type SidecarIDE struct {
	ConfigPath string         `json:"config_path"`
	ScriptDir  string         `json:"script_dir"`
	Entries    []SidecarEntry `json:"entries"`
}

type Sidecar struct {
	Name    string                `json:"name"`
	Version string                `json:"version"`
	Scope   string                `json:"scope"`
	WorkBin string                `json:"work_bin"`
	IDEs    map[string]SidecarIDE `json:"ides"`
}

func SidecarPath(name string) (string, error) {
	dir, err := HooksInstalledDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

// LoadSidecar 读取 sidecar 文件，使用共享文件锁防止读脏数据。
func LoadSidecar(name string) (*Sidecar, error) {
	path, err := SidecarPath(name)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("打开 sidecar 文件失败: %w", err)
	}
	defer f.Close()
	if err := flockSH(f); err != nil {
		return nil, fmt.Errorf("获取 sidecar 共享锁失败: %w", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取 sidecar 文件信息失败: %w", err)
	}
	if fi.Size() == 0 {
		return nil, fmt.Errorf("未找到 hooks 安装记录: %s", name)
	}
	var sc Sidecar
	if err := json.NewDecoder(f).Decode(&sc); err != nil {
		return nil, fmt.Errorf("解析 sidecar 文件失败: %w", err)
	}
	return &sc, nil
}

// SaveSidecar 写入 sidecar 文件，使用独占文件锁防止并发写入损坏。
func SaveSidecar(sc *Sidecar) error {
	if sc == nil {
		return fmt.Errorf("不能保存空的 sidecar")
	}
	path, err := SidecarPath(sc.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建 sidecar 目录失败: %w", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("打开 sidecar 文件失败: %w", err)
	}
	defer f.Close()
	if err := flockEX(f); err != nil {
		return fmt.Errorf("获取 sidecar 独占锁失败: %w", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 sidecar 失败: %w", err)
	}
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("清空 sidecar 文件失败: %w", err)
	}
	if _, err := f.WriteAt(data, 0); err != nil {
		return fmt.Errorf("写入 sidecar 文件失败: %w", err)
	}
	return nil
}

// RemoveSidecar 删除 sidecar 文件，使用独占文件锁防止与并发写入冲突。
func RemoveSidecar(name string) error {
	path, err := SidecarPath(name)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("打开 sidecar 文件失败: %w", err)
	}
	defer f.Close()
	if err := flockEX(f); err != nil {
		return fmt.Errorf("获取 sidecar 独占锁失败: %w", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 sidecar 文件失败: %w", err)
	}
	return nil
}

func IsWorkManagedCommand(cmd string) bool {
	return filepath.Base(filepath.Dir(cmd)) == workTelemetryDir ||
		contains(cmd, "/"+workTelemetryDir+"/") ||
		contains(cmd, `\`+workTelemetryDir+`\`)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// flockSH 对文件加共享锁（阻塞式），超时 5 秒。
func flockSH(f *os.File) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("加共享锁失败: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("获取共享锁超时，可能有其他 work 进程正在写入")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// flockEX 对文件加独占锁（阻塞式），超时 5 秒。
func flockEX(f *os.File) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("加独占锁失败: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("获取独占锁超时，可能有其他 work 进程正在操作")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
