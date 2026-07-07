package platform

import (
	"os"
	"path/filepath"
)

// DetectCursor 检测当前系统是否安装了 Cursor IDE。
func DetectCursor() bool {
	home, err := UserHome()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".cursor"))
	return err == nil
}

// DetectQoder 检测当前系统是否安装了 Qoder IDE。
func DetectQoder() bool {
	home, err := UserHome()
	if err != nil {
		return false
	}
	return dirExists(filepath.Join(home, ".qoder"))
}

// DetectClaude 检测当前系统是否安装了 Claude Code。
// 同时检查 ~/.claude/（标准安装路径）和 XDG_CONFIG_HOME/claude/。
func DetectClaude() bool {
	home, err := UserHome()
	if err != nil {
		return false
	}
	// 标准安装路径
	if dirExists(filepath.Join(home, ".claude")) {
		return true
	}
	// 也可能以文件形式存在（~/.claude.json）
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); err == nil {
		return true
	}
	// XDG 配置路径
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	if dirExists(filepath.Join(configHome, "claude")) {
		return true
	}
	return false
}

// dirExists 检查路径是否存在且为目录。
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
