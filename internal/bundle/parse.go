package bundle

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ManifestFileName = "bundle.yaml"

func ParseDir(dir string) (*Manifest, error) {
	return ParseFile(filepath.Join(dir, ManifestFileName))
}

func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 bundle 配置失败: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 bundle.yaml 失败: %w", err)
	}
	if m.Type == "" {
		m.Type = "bundle"
	}
	if err := Validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}
