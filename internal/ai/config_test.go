package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigYAML(t *testing.T, content string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := filepath.Join(home, ".work", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestLoadModelConfigDefault(t *testing.T) {
	writeConfigYAML(t, `
ai:
  models:
    default:
      url: https://api.openai.com/v1/chat/completions
      api_key: sk-test-key
      model: gpt-4o
`)
	cfg, err := LoadModelConfig("")
	if err != nil {
		t.Fatalf("LoadModelConfig: %v", err)
	}
	if cfg.URL != "https://api.openai.com/v1/chat/completions" {
		t.Fatalf("URL = %q", cfg.URL)
	}
	if cfg.APIKey != "sk-test-key" {
		t.Fatalf("APIKey = %q", cfg.APIKey)
	}
	if cfg.Model != "gpt-4o" {
		t.Fatalf("Model = %q", cfg.Model)
	}
	// 默认值
	if cfg.Timeout != "120s" {
		t.Fatalf("Timeout default = %q, want 120s", cfg.Timeout)
	}
}

func TestLoadModelConfigExplicitProfile(t *testing.T) {
	writeConfigYAML(t, `
ai:
  models:
    default:
      url: https://a.example.com
      api_key: ka
      model: ma
    deepseek:
      url: https://b.example.com
      api_key: kb
      model: mb
`)
	cfg, err := LoadModelConfig("deepseek")
	if err != nil {
		t.Fatalf("LoadModelConfig: %v", err)
	}
	if cfg.URL != "https://b.example.com" {
		t.Fatalf("URL = %q", cfg.URL)
	}
	if cfg.Model != "mb" {
		t.Fatalf("Model = %q", cfg.Model)
	}
}

func TestLoadModelConfigMissingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_, err := LoadModelConfig("")
	if err == nil {
		t.Fatal("期望错误，但成功返回")
	}
}

func TestLoadModelConfigNoAISection(t *testing.T) {
	writeConfigYAML(t, `
registry:
  url: https://x.example.com
`)
	_, err := LoadModelConfig("")
	if err == nil {
		t.Fatal("期望错误，但成功返回")
	}
}

func TestLoadModelConfigMissingRequired(t *testing.T) {
	writeConfigYAML(t, `
ai:
  models:
    default:
      url: https://api.example.com
      model: gpt-4o
`)
	_, err := LoadModelConfig("")
	if err == nil {
		t.Fatal("api_key 缺失，期望错误")
	}
}

func TestLoadModelConfigDefaultTimeout(t *testing.T) {
	writeConfigYAML(t, `
ai:
  models:
    default:
      url: https://api.example.com
      api_key: sk-key
      model: gpt-4o
`)
	cfg, err := LoadModelConfig("")
	if err != nil {
		t.Fatalf("LoadModelConfig: %v", err)
	}
	if cfg.Timeout != "120s" {
		t.Fatalf("Timeout = %q, want default 120s", cfg.Timeout)
	}
}

func TestListProfiles(t *testing.T) {
	writeConfigYAML(t, `
ai:
  models:
    default:
      url: https://a.example.com
      api_key: ka
      model: ma
    deepseek:
      url: https://b.example.com
      api_key: kb
      model: mb
`)
	names, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("len = %d, want 2", len(names))
	}
}

// AI 配置加载基准测试 — 衡量 YAML 解析 + 环境变量展开的性能。

func BenchmarkLoadModelConfig(b *testing.B) {
	home := b.TempDir()
	b.Setenv("HOME", home)
	b.Setenv("TEST_AI_KEY", "sk-bench-key-12345")
	p := filepath.Join(home, ".work", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		b.Fatal(err)
	}
	content := `
ai:
  models:
    default:
      provider: openai
      url: https://api.openai.com/v1/chat/completions
      api_key: ${TEST_AI_KEY}
      model: gpt-4o
      timeout: 120s
      max_tokens: 4096
`
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		_, err := LoadModelConfig("")
		if err != nil {
			b.Fatal(err)
		}
	}
}
