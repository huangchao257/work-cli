package bundle

import (
	"fmt"
	"os"
	"strings"

	"github.com/huangchao257/work-cli/internal/pkg/manifest"
)

func Validate(m *Manifest) error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("bundle.yaml 缺少 name 字段")
	}
	if err := manifest.ValidateID(m.Name); err != nil {
		return fmt.Errorf("bundle.yaml 名称非法: %w", err)
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("bundle.yaml 缺少 version 字段")
	}
	for _, s := range m.Resources.Skills {
		if err := manifest.ValidateID(s.ID); err != nil {
			return fmt.Errorf("skill id 非法: %w", err)
		}
		if strings.TrimSpace(s.Source) == "" {
			return fmt.Errorf("skill %s 缺少 source", s.ID)
		}
	}
	for _, r := range m.Resources.Rules {
		if err := manifest.ValidateID(r.ID); err != nil {
			return fmt.Errorf("rule id 非法: %w", err)
		}
		if r.Apply == "files" && len(r.Globs) == 0 {
			return fmt.Errorf("规则 %s 的 apply=files 时必须提供 globs", r.ID)
		}
		if r.Apply != "always" && r.Apply != "manual" && r.Apply != "files" {
			return fmt.Errorf("规则 %s 的 apply 无效: %s", r.ID, r.Apply)
		}
	}
	for _, mc := range m.Resources.MCP {
		if err := manifest.ValidateID(mc.ID); err != nil {
			return fmt.Errorf("mcp id 非法: %w", err)
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
