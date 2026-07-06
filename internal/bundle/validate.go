package bundle

import (
	"fmt"
	"os"
	"strings"
)

func Validate(m *Manifest) error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("bundle.yaml 缺少 name 字段")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("bundle.yaml 缺少 version 字段")
	}
	for _, r := range m.Resources.Rules {
		if r.Apply == "files" && len(r.Globs) == 0 {
			return fmt.Errorf("规则 %s 的 apply=files 时必须提供 globs", r.ID)
		}
		if r.Apply != "always" && r.Apply != "manual" && r.Apply != "files" {
			return fmt.Errorf("规则 %s 的 apply 无效: %s", r.ID, r.Apply)
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

func CheckRequiredEnvVars(env []EnvVar) []string {
	var missing []string
	for _, e := range env {
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
