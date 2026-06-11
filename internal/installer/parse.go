package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const ManifestFileName = "installer.yaml"

func ParseDir(dir string) (*Manifest, error) {
	return ParseFile(filepath.Join(dir, ManifestFileName))
}

func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 installer 配置失败: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 installer.yaml 失败: %w", err)
	}
	if err := Validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func Validate(m *Manifest) error {
	if m.Type != "cli" {
		return fmt.Errorf("installer.yaml 的 type 必须为 cli")
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("installer.yaml 缺少 name 字段")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("installer.yaml 缺少 version 字段")
	}
	if strings.TrimSpace(m.Install.Run) == "" && len(m.Install.Platforms) == 0 {
		return fmt.Errorf("installer.yaml 缺少 install 命令")
	}
	return nil
}
