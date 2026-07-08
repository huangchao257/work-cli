package platform

import (
	"os"
	"path/filepath"
)

func UserHome() (string, error) {
	// HOME 环境变量优先（测试隔离 + 跨平台兼容）。
	// os.UserHomeDir() 在 Windows 上读 USERPROFILE 而非 HOME，
	// 导致 t.Setenv("HOME", ...) 在 Windows CI 上对 UserHome() 无效。
	if h := os.Getenv("HOME"); h != "" {
		return h, nil
	}
	return os.UserHomeDir()
}

func WorkConfigDir() (string, error) {
	home, err := UserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".work"), nil
}

func WorkStatePath(scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".work", "installed.json"), nil
	}
	base, err := WorkConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "installed.json"), nil
}

// WorkSubDir 返回 ~/.work/<name> 子目录的绝对路径，并按需创建（权限 0700）。
// 用于 hooks-installed、telemetry、cache 等统一子目录解析，避免各包重复
// 调用 os.UserHomeDir + filepath.Join + os.MkdirAll。
func WorkSubDir(name string) (string, error) {
	base, err := WorkConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ConfigFilePath 返回 ~/.work/config.yaml 的绝对路径。
// 作为各包（selfupdate/source/hooks 等）读取用户配置的统一入口，
// 避免每个包各自拼路径造成重复。
func ConfigFilePath() (string, error) {
	base, err := WorkConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "config.yaml"), nil
}

func ProjectRoot() (string, error) {
	return os.Getwd()
}
