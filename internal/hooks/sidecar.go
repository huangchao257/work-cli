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

// sidecarLockPath 返回 sidecar 稳定锁文件路径。
// 锁必须加在旁路 .lock 文件而非 sidecar 文件本身：sidecar 在 Save 中被
// rename 替换（Windows 上 rename 覆盖正被本进程打开的文件必然失败），
// 且 Unix flock 绑定 inode，rename 换 inode 后锁互斥即失效。
// 锁文件自身从不被 rename/remove。
func sidecarLockPath(name string) (string, error) {
	base, err := SidecarPath(name)
	if err != nil {
		return "", err
	}
	return base + ".lock", nil
}

// lockSidecar 打开旁路锁文件并加锁，返回待关闭的句柄。
func lockSidecar(name string, how int) (*os.File, error) {
	lockPath, err := sidecarLockPath(name)
	if err != nil {
		return nil, err
	}
	lf, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("打开 sidecar 锁文件失败: %w", err)
	}
	if err := platform.FlockLock(lf, lockPath, how); err != nil {
		_ = lf.Close()
		return nil, fmt.Errorf("获取 sidecar 文件锁失败: %w", err)
	}
	return lf, nil
}

// LoadSidecar 读取 sidecar 文件，使用共享文件锁防止读脏数据。
func LoadSidecar(name string) (*Sidecar, error) {
	lf, err := lockSidecar(name, platform.FlockSH)
	if err != nil {
		return nil, err
	}
	defer lf.Close()

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
	if len(data) == 0 {
		return nil, fmt.Errorf("未找到 hooks 安装记录: %s", name)
	}
	var sc Sidecar
	if err := json.Unmarshal(data, &sc); err != nil {
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

	// 锁加在旁路 .lock 文件上（见 sidecarLockPath），先取锁再写临时文件，
	// 保证与 LoadSidecar / RemoveSidecar 串行化。
	lf, err := lockSidecar(sc.Name, platform.FlockEX)
	if err != nil {
		return err
	}
	defer lf.Close()

	// 原子写入：写临时文件后 rename。全程不持有 sidecar 文件自身的句柄，
	// 避免 Windows 上 rename 覆盖被本进程打开的文件失败。
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
	// 锁加在旁路 .lock 文件上；不预先打开 sidecar 文件本身，
	// 避免 Windows 上删除被本进程打开的文件失败。
	lf, err := lockSidecar(name, platform.FlockEX)
	if err != nil {
		return err
	}
	defer lf.Close()

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 sidecar 文件失败: %w", err)
	}
	return nil
}

// IsWorkManagedCommand 判断命令是否 work 管理的 hook 条目。
// kitName 为空时匹配任意套装（向后兼容旧调用）；非空时仅匹配该套装——
// work 管理的脚本路径形如 .../hooks/work-telemetry/<kitName>/...，
// merge/unmerge 必须按套装隔离，否则安装第二个套装会删掉第一个的条目。
func IsWorkManagedCommand(cmd string, kitName string) bool {
	if kitName == "" {
		return filepath.Base(filepath.Dir(cmd)) == workTelemetryDir ||
			strings.Contains(cmd, "/"+workTelemetryDir+"/") ||
			strings.Contains(cmd, `\`+workTelemetryDir+`\`)
	}
	// kit 级匹配：work-telemetry/<kit> 段（正/反斜杠两种分隔符）
	for _, sep := range []string{"/", `\`} {
		if strings.Contains(cmd, sep+workTelemetryDir+sep+kitName+sep) {
			return true
		}
	}
	// 兼容 cmd 恰好以 kit 目录为结尾（无尾部分隔符）的情况
	return filepath.Base(cmd) != "" && filepath.Base(filepath.Dir(filepath.Dir(cmd))) == workTelemetryDir &&
		filepath.Base(filepath.Dir(cmd)) == kitName
}
