package source

import (
	"fmt"
	"os"
	"path/filepath"

	pkgmanifest "github.com/huangchao257/work-cli/internal/pkg/manifest"
)

func ResolveLocal(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("本地路径不存在: %s", abs)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("本地路径必须是目录: %s", abs)
	}
	if _, err := pkgmanifest.DetectKind(abs); err != nil {
		return "", err
	}
	return abs, nil
}
