package hooks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/huangchao257/work-cli/internal/platform"
)

func HooksConfigPath(ide, scope string) (string, error) {
	info := platform.LookupIDE(platform.IDE(ide))
	if info == nil {
		return "", fmt.Errorf("未知 IDE: %s", ide)
	}
	base, err := ideHooksBase(ide, scope)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(base), info.HooksFile), nil
}

func HooksScriptDir(ide, scope, kitName string) (string, error) {
	base, err := ideHooksBase(ide, scope)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, workTelemetryDir, kitName), nil
}

func ideHooksBase(ide, scope string) (string, error) {
	info := platform.LookupIDE(platform.IDE(ide))
	if info == nil {
		return "", fmt.Errorf("未知 IDE: %s", ide)
	}
	if scope == "project" {
		root, err := platform.ProjectRoot()
		if err != nil {
			return "", err
		}
		return filepath.Join(root, info.DotDir, "hooks"), nil
	}
	home, err := platform.UserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, info.DotDir, "hooks"), nil
}

func commandPathForIDE(ide, scope, kitName, scriptName string) (string, error) {
	info := platform.LookupIDE(platform.IDE(ide))
	if info == nil {
		return "", fmt.Errorf("未知 IDE: %s", ide)
	}
	dir, err := HooksScriptDir(ide, scope, kitName)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filepath.Join(dir, scriptName))
	if err != nil {
		return "", err
	}
	if info.HooksFile != "hooks.json" { // 非 Cursor 格式：用绝对路径
		return abs, nil
	}
	// Cursor：返回相对路径
	var base string
	if scope == "project" {
		base, err = platform.ProjectRoot()
	} else {
		base, err = platform.UserHome()
		if err == nil {
			base = filepath.Join(base, info.DotDir)
		}
	}
	if err != nil {
		return abs, nil
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil {
		return abs, nil
	}
	return filepath.ToSlash(rel), nil
}

func writeExecutable(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建脚本目录失败: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return fmt.Errorf("写入脚本文件失败: %w", err)
	}
	return nil
}
