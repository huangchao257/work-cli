package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func LoadSidecar(name string) (*Sidecar, error) {
	path, err := SidecarPath(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("未找到 hooks 安装记录: %s", name)
		}
		return nil, fmt.Errorf("读取 sidecar 文件失败: %w", err)
	}
	var sc Sidecar
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("解析 sidecar 文件失败: %w", err)
	}
	return &sc, nil
}

func SaveSidecar(sc *Sidecar) error {
	path, err := SidecarPath(sc.Name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 sidecar 失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("写入 sidecar 文件失败: %w", err)
	}
	return nil
}

func RemoveSidecar(name string) error {
	path, err := SidecarPath(name)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
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
