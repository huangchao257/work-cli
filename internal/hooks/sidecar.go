package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/huangchao257/work-cli/internal/platform"
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
	Name         string                `json:"name"`
	Version      string                `json:"version"`
	Scope        string                `json:"scope"`
	WorkBin      string                `json:"work_bin"`
	IDEs         map[string]SidecarIDE `json:"ides"`
	RedactFields []string              `json:"redact_fields,omitempty"`
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
	f, err := os.OpenFile(path, os.O_RDONLY, 0o600)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("未找到 hooks 安装记录: %s", name)
		}
		return nil, fmt.Errorf("打开 sidecar 文件失败: %w", err)
	}
	defer f.Close()
	if err := platform.FlockLock(f, path, platform.FlockSH); err != nil {
		return nil, fmt.Errorf("获取 sidecar 共享锁失败: %w", err)
	}
	defer func() { _ = platform.FlockUnlock(f) }()

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

	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 sidecar 失败: %w", err)
	}

	// 原子写入：先写临时文件，加锁后 rename，避免 truncate 中途崩溃导致文件为空。
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sidecar-*.json")
	if err != nil {
		return fmt.Errorf("创建临时 sidecar 文件失败: %w", err)
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
		return fmt.Errorf("写入临时 sidecar 文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时 sidecar 文件失败: %w", err)
	}

	// 打开目标文件加独占锁，确保 rename 时无并发读取。
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("打开 sidecar 文件失败: %w", err)
	}
	defer f.Close()
	if err := platform.FlockLock(f, path, platform.FlockEX); err != nil {
		return fmt.Errorf("获取 sidecar 独占锁失败: %w", err)
	}
	defer func() { _ = platform.FlockUnlock(f) }()

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("原子替换 sidecar 文件失败: %w", err)
	}
	cleanup = false
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
	if err := platform.FlockLock(f, path, platform.FlockEX); err != nil {
		return fmt.Errorf("获取 sidecar 独占锁失败: %w", err)
	}
	defer func() { _ = platform.FlockUnlock(f) }()

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 sidecar 文件失败: %w", err)
	}
	return nil
}

func IsWorkManagedCommand(cmd string) bool {
	return filepath.Base(filepath.Dir(cmd)) == workTelemetryDir ||
		strings.Contains(cmd, "/"+workTelemetryDir+"/") ||
		strings.Contains(cmd, `\`+workTelemetryDir+`\`)
}
