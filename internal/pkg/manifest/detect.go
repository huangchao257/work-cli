package manifest

import (
	"fmt"
	"os"
	"path/filepath"
)

type Kind string

const (
	KindBundle Kind = "bundle"
	KindCLI    Kind = "cli"
)

func DetectKind(dir string) (Kind, error) {
	if fileExists(filepath.Join(dir, "installer.yaml")) {
		return KindCLI, nil
	}
	if fileExists(filepath.Join(dir, "bundle.yaml")) {
		return KindBundle, nil
	}
	return "", fmt.Errorf("未找到 installer.yaml 或 bundle.yaml")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
