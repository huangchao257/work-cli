package manifest

import (
	"fmt"
	"os"
	"path/filepath"
)

// Kind 表示套装类型。
type Kind string

const (
	KindBundle Kind = "bundle"
	KindCLI    Kind = "cli"
	KindHooks  Kind = "hooks"
)

// FileName 返回 Kind 对应的 manifest 文件名。
// 未知 Kind 返回空字符串。
func FileName(k Kind) string {
	switch k {
	case KindBundle:
		return "bundle.yaml"
	case KindCLI:
		return "installer.yaml"
	case KindHooks:
		return "hooks.yaml"
	}
	return ""
}

// KindFromFile 根据 manifest 文件名反查 Kind。不匹配时返回空字符串。
func KindFromFile(name string) (Kind, bool) {
	switch filepath.Base(name) {
	case "bundle.yaml":
		return KindBundle, true
	case "installer.yaml":
		return KindCLI, true
	case "hooks.yaml":
		return KindHooks, true
	}
	return "", false
}

// ManifestFileNames 返回所有支持的 manifest 文件名集合。
func ManifestFileNames() []string {
	return []string{"bundle.yaml", "installer.yaml", "hooks.yaml"}
}

// DetectKind 按优先级查找目录中的 manifest 文件并返回类型。
func DetectKind(dir string) (Kind, error) {
	if fileExists(filepath.Join(dir, "installer.yaml")) {
		return KindCLI, nil
	}
	if fileExists(filepath.Join(dir, "hooks.yaml")) {
		return KindHooks, nil
	}
	if fileExists(filepath.Join(dir, "bundle.yaml")) {
		return KindBundle, nil
	}
	return "", fmt.Errorf("未找到 installer.yaml、hooks.yaml 或 bundle.yaml")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
