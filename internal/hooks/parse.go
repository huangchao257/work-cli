package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const ManifestFileName = "hooks.yaml"

func ParseDir(dir string) (*Manifest, error) {
	return ParseFile(filepath.Join(dir, ManifestFileName))
}

func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 hooks 配置失败: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 hooks.yaml 失败: %w", err)
	}
	if m.Type == "" {
		m.Type = "hooks"
	}
	if err := Validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func Validate(m *Manifest) error {
	if m.Type != "hooks" {
		return fmt.Errorf("hooks.yaml type 必须为 hooks，当前为 %q", m.Type)
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("hooks.yaml 缺少 name")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("hooks.yaml 缺少 version")
	}
	if len(m.Resources.Hooks) == 0 {
		return fmt.Errorf("hooks.yaml 至少需要一个 resources.hooks 项")
	}
	for _, h := range m.Resources.Hooks {
		if strings.TrimSpace(h.ID) == "" {
			return fmt.Errorf("hooks 资源缺少 id")
		}
		if strings.TrimSpace(h.Source) == "" {
			return fmt.Errorf("hooks 资源 %s 缺少 source", h.ID)
		}
	}
	return nil
}

func CheckRequiredEnv(m *Manifest) []string {
	var missing []string
	for _, e := range m.Env {
		if !e.Required {
			continue
		}
		if os.Getenv(e.Name) == "" {
			missing = append(missing, e.Name)
		}
	}
	return missing
}

// RequiredEnvNames 返回标记为 required 的环境变量名称列表。
// 共享给 engine 包用于生成缺失环境变量的报错信息。
func RequiredEnvNames(env []EnvVar) []string {
	var names []string
	for _, e := range env {
		if e.Required {
			names = append(names, e.Name)
		}
	}
	return names
}
