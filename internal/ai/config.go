// Package ai 提供 AI 模型配置与 API 调用能力。
package ai

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/huangchao257/work-cli/internal/configcache"
	"github.com/huangchao257/work-cli/internal/platform"
)

// ModelConfig 表示单个 model profile 的配置。
type ModelConfig struct {
	Provider     string            `yaml:"provider"`
	URL          string            `yaml:"url"`
	APIKey       string            `yaml:"api_key"`
	Model        string            `yaml:"model"`
	Timeout      string            `yaml:"timeout"`
	MaxTokens    int               `yaml:"max_tokens"`
	ExtraHeaders map[string]string `yaml:"extra_headers"`
}

type aiFileConfig struct {
	AI struct {
		Models map[string]ModelConfig `yaml:"models"`
	} `yaml:"ai"`
}

// String 返回模型配置的概要描述，API Key 脱敏显示。
func (c *ModelConfig) String() string {
	keyDisplay := "[未设置]"
	if strings.TrimSpace(c.APIKey) != "" {
		keyDisplay = "[已设置]"
	}
	return fmt.Sprintf("ModelConfig{provider=%s, model=%s, api_key=%s}", c.Provider, c.Model, keyDisplay)
}

// sysConfigPath 返回 ~/.work/config.yaml 的绝对路径，用于测试替换。
var sysConfigPath = platform.ConfigFilePath

// LoadModelConfig 加载 profile 名称对应的模型配置。profile 为空时取 "default"。
// 自动展开 api_key 中的 ${ENV_VAR} 引用，并校验必填字段。
func LoadModelConfig(profile string) (*ModelConfig, error) {
	if strings.TrimSpace(profile) == "" {
		profile = "default"
	}
	path, err := sysConfigPath()
	if err != nil {
		return nil, fmt.Errorf("无法定位配置文件: %w", err)
	}
	data, err := configcache.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("未找到 %s，请先配置 ai.models 段", path)
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var fc aiFileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	if fc.AI.Models == nil {
		return nil, fmt.Errorf("未配置 ai.models，请在 %s 中设置", path)
	}
	cfg, ok := fc.AI.Models[profile]
	if !ok {
		names := make([]string, 0, len(fc.AI.Models))
		for n := range fc.AI.Models {
			names = append(names, n)
		}
		return nil, fmt.Errorf("未找到模型 profile %q，可用: %s", profile, strings.Join(names, ", "))
	}
	// 默认值
	if strings.TrimSpace(cfg.Timeout) == "" {
		cfg.Timeout = "120s"
	}
	// 展开 api_key 中的环境变量
	cfg.APIKey = expandEnv(cfg.APIKey)
	// 校验必填
	if err := validateModelConfig(&cfg, profile); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ListProfiles 列出所有已配置的 profile 名称。
func ListProfiles() ([]string, error) {
	path, err := sysConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := configcache.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var fc aiFileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(fc.AI.Models))
	for n := range fc.AI.Models {
		names = append(names, n)
	}
	return names, nil
}

var envPattern = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)

// expandEnv 对值做展开：若整个值匹配 ${NAME}，替换为 os.Getenv(NAME)。
// 环境变量未设置时返回含变量名的错误。明文原样返回。
func expandEnv(val string) string {
	m := envPattern.FindStringSubmatch(strings.TrimSpace(val))
	if m == nil {
		return val
	}
	return os.Getenv(m[1])
}

func validateModelConfig(cfg *ModelConfig, profile string) error {
	if strings.TrimSpace(cfg.URL) == "" {
		return fmt.Errorf("ai.models.%s.url 不能为空", profile)
	}
	raw := cfg.APIKey
	// 如果 api_key 使用的是 ${ENV_VAR} 但环境变量未设置
	if m := envPattern.FindStringSubmatch(strings.TrimSpace(raw)); m != nil {
		if os.Getenv(m[1]) == "" {
			return fmt.Errorf("环境变量 %s 未设置（在 ai.models.%s.api_key 中引用）", m[1], profile)
		}
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return fmt.Errorf("ai.models.%s.api_key 不能为空", profile)
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("ai.models.%s.model 不能为空", profile)
	}
	return nil
}
