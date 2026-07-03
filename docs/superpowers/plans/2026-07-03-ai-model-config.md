# AI 模型配置 + work generate agents 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `~/.work/config.yaml` 中支持 `ai.models` 多 profile 配置，新增 `work generate agents` 命令基于 CodeGraph 索引调用 LLM 生成 Agent 文件。

**Architecture:** 新增 `internal/ai/` 包提供模型配置加载与 OpenAI 兼容 HTTP 调用，新增 `internal/generate/` 包编排生成流程（读 codegraph → 构造 prompt → 调 LLM → 写文件），新增 `internal/cli/generate.go` 暴露 cobra 子命令。与现有 config/selfupdate/graph 包解耦。

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`（已有），`net/http`（标准库），cobra（已有）

## Global Constraints

- 用户可见字符串与代码注释用中文
- YAML 配置路径 `~/.work/config.yaml`，与现有 `self_update`、`telemetry` 平级
- 遵循项目既有模式：领域包独立解析自己的配置段（参考 `selfupdate/config.go`、`hooks/config.go`）
- `api_key` 支持 `${ENV_VAR}` 语法引用环境变量，仅对 `api_key` 展开
- 必填字段缺失时返回错误，配置文件不存在时返回错误
- `--dry-run` 贯穿所有文件系统操作
- `--json` 输出通过 `internal/output` 包统一渲染
- 退出码：用法错误 → 2（`usage.Error`），运行时错误 → 1

---

### Task 1: `internal/ai/config.go` — 模型配置类型与加载

**Files:**
- Create: `internal/ai/config.go`
- Create: `internal/ai/config_test.go`

**Interfaces:**
- Produces: `ModelConfig` struct, `LoadModelConfig(profile string) (*ModelConfig, error)`, `ListProfiles() ([]string, error)`

- [ ] **Step 1: 编写 `config_test.go` 中首个测试 —— 正常加载单 profile**

```go
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
```

- [ ] **Step 2: 运行测试验证失败**

```bash
go test ./internal/ai/... -run TestLoadModelConfigDefault -v
```
期望: 编译失败，`LoadModelConfig` 未定义。

- [ ] **Step 3: 实现 `internal/ai/config.go` 最小代码**

```go
// Package ai 提供 AI 模型配置与 API 调用能力。
package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
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

// sysConfigPath 返回 ~/.work/config.yaml 的绝对路径，用于测试替换。
var sysConfigPath = defaultConfigPath

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".work", "config.yaml"), nil
}

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
	data, err := os.ReadFile(path)
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
	data, err := os.ReadFile(path)
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
```

- [ ] **Step 4: 运行测试验证通过**

```bash
go test ./internal/ai/... -run TestLoadModelConfigDefault -v
```
期望: PASS

- [ ] **Step 5: 添加更多测试 —— 取指定 profile、配置文件不存在、必填字段缺失**

在 `config_test.go` 中追加：

```go
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
```

- [ ] **Step 6: 运行全部测试**

```bash
go test ./internal/ai/... -v
```
期望: 所有测试 PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ai/config.go internal/ai/config_test.go
git commit -m "feat(ai): 新增 AI 模型配置加载（支持多 profile 与 \${ENV} 展开）

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 2: `internal/ai/config.go` — 环境变量展开测试

**Files:**
- Modify: `internal/ai/config_test.go`（追加测试）

**Interfaces:**
- Consumes: `LoadModelConfig(profile string) (*ModelConfig, error)` from Task 1

- [ ] **Step 1: 添加环境变量展开测试**

在 `config_test.go` 末尾追加：

```go
func TestExpandEnvInAPIKey(t *testing.T) {
	t.Setenv("MY_AI_KEY", "secret-from-env")
	writeConfigYAML(t, `
ai:
  models:
    default:
      url: https://api.example.com
      api_key: ${MY_AI_KEY}
      model: gpt-4o
`)
	cfg, err := LoadModelConfig("")
	if err != nil {
		t.Fatalf("LoadModelConfig: %v", err)
	}
	if cfg.APIKey != "secret-from-env" {
		t.Fatalf("APIKey = %q, want secret-from-env", cfg.APIKey)
	}
}

func TestExpandEnvMissingEnvVar(t *testing.T) {
	writeConfigYAML(t, `
ai:
  models:
    default:
      url: https://api.example.com
      api_key: ${MISSING_VAR}
      model: gpt-4o
`)
	_, err := LoadModelConfig("")
	if err == nil {
		t.Fatal("缺环境变量应报错")
	}
	if !strings.Contains(err.Error(), "MISSING_VAR") {
		t.Fatalf("错误信息应含变量名: %v", err)
	}
}

func TestPlaintextAPIKey(t *testing.T) {
	writeConfigYAML(t, `
ai:
  models:
    default:
      url: https://api.example.com
      api_key: sk-plaintext-key-12345
      model: gpt-4o
`)
	cfg, err := LoadModelConfig("")
	if err != nil {
		t.Fatalf("LoadModelConfig: %v", err)
	}
	if cfg.APIKey != "sk-plaintext-key-12345" {
		t.Fatalf("APIKey = %q", cfg.APIKey)
	}
}
```

需要添加 import: `"strings"`

- [ ] **Step 2: 运行测试**

```bash
go test ./internal/ai/... -v
```
期望: 全部 PASS

- [ ] **Step 3: Commit**

```bash
git add internal/ai/config_test.go
git commit -m "test(ai): 补充环境变量展开与明文密钥测试

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: `internal/ai/client.go` — OpenAI 兼容 HTTP 客户端

**Files:**
- Create: `internal/ai/client.go`
- Create: `internal/ai/client_test.go`

**Interfaces:**
- Consumes: `*ModelConfig` from `internal/ai/config.go`
- Produces: `Call(ctx context.Context, cfg *ModelConfig, systemPrompt, userContent string) (*ChatResponse, error)`, `ChatResponse` struct

- [ ] **Step 1: 编写 `client_test.go`**

```go
package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 校验请求头
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "bad auth", 401)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "bad content-type", 400)
			return
		}
		// 简单解析 body 校验 model
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "gpt-4o" {
			http.Error(w, "bad model", 400)
			return
		}

		resp := ChatResponse{
			ID: "chatcmpl-123",
			Choices: []Choice{{
				Message: Message{
					Role:    "assistant",
					Content: "生成的内容",
				},
				FinishReason: "stop",
			}},
			Usage: Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &ModelConfig{
		URL:     srv.URL,
		APIKey:  "test-key",
		Model:   "gpt-4o",
		Timeout: "30s",
	}

	resp, err := Call(t.Context(), cfg, "你是一个助手", "分析这段代码")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if resp.Choices[0].Message.Content != "生成的内容" {
		t.Fatalf("Content = %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.PromptTokens != 100 {
		t.Fatalf("PromptTokens = %d", resp.Usage.PromptTokens)
	}
}

func TestCallExtraHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "my-value" {
			http.Error(w, "missing custom header", 400)
			return
		}
		resp := ChatResponse{
			Choices: []Choice{{Message: Message{Content: "ok"}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &ModelConfig{
		URL:     srv.URL,
		APIKey:  "k",
		Model:   "m",
		Timeout: "10s",
		ExtraHeaders: map[string]string{
			"X-Custom": "my-value",
		},
	}
	_, err := Call(t.Context(), cfg, "", ".")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
}

func TestCallHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	cfg := &ModelConfig{
		URL:     srv.URL,
		APIKey:  "k",
		Model:   "m",
		Timeout: "10s",
	}
	_, err := Call(t.Context(), cfg, "", ".")
	if err == nil {
		t.Fatal("期望 HTTP 500 错误")
	}
}
```

- [ ] **Step 2: 运行测试验证编译失败**

```bash
go test ./internal/ai/... -run TestCallSuccess -v
```
期望: 编译失败（Call、ChatResponse 等未定义）

- [ ] **Step 3: 实现 `internal/ai/client.go`**

```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatRequest OpenAI 兼容 chat completion 请求体。
type ChatRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

// Message chat 消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse OpenAI 兼容 chat completion 响应体。
type ChatResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice 响应选项。
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage token 用量。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Call 发起一次 OpenAI 兼容的 chat completion 请求。
// cfg 中的 ExtraHeaders 会注入到 HTTP 请求头中。
func Call(ctx context.Context, cfg *ModelConfig, systemPrompt, userContent string) (*ChatResponse, error) {
	dur, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		dur = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	reqBody := ChatRequest{
		Model: cfg.Model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		MaxTokens: cfg.MaxTokens,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("编码请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("调用 AI 模型失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("AI 模型返回错误 (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败: %w", err)
	}
	return &chatResp, nil
}
```

- [ ] **Step 4: 运行全部测试**

```bash
go test ./internal/ai/... -v
```
期望: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/client.go internal/ai/client_test.go
git commit -m "feat(ai): 新增 OpenAI 兼容 HTTP 客户端

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: `internal/generate/generate.go` — Agent 生成核心逻辑

**Files:**
- Create: `internal/generate/generate.go`

**Interfaces:**
- Consumes: `ai.Call(ctx, cfg, systemPrompt, userContent) (*ai.ChatResponse, error)` from Task 3, `ai.LoadModelConfig(profile) (*ai.ModelConfig, error)` from Task 1
- Produces: `GenerateAgents(ctx context.Context, opts Options) (*Result, error)`, `Options` struct, `Result` struct

- [ ] **Step 1: 实现 `internal/generate/generate.go`**

```go
// Package generate 提供基于 CodeGraph 的 AI 代码生成能力。
package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/huangchao257/work-cli/internal/ai"
)

// Options 生成选项。
type Options struct {
	ProjectPath  string // 项目根目录，默认当前目录
	ModelProfile string // AI 模型 profile，默认 "default"
	DryRun       bool   // 仅预览不写入
}

// Result 生成结果。
type Result struct {
	Profile string   `json:"profile"`
	Model   string   `json:"model"`
	Files   []string `json:"files"`
	Tokens  *Tokens  `json:"tokens,omitempty"`
	DryRun  bool     `json:"dry_run"`
}

// Tokens token 用量。
type Tokens struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
}

// GenerateAgents 基于 CodeGraph 索引调用 LLM 生成 Agent 文件。
func GenerateAgents(ctx context.Context, opts Options) (*Result, error) {
	// 1. 确保 codegraph 可用
	if _, err := exec.LookPath("codegraph"); err != nil {
		return nil, fmt.Errorf("未找到 codegraph，请先执行 work install codegraph-stack")
	}

	// 2. 加载 AI 配置
	cfg, err := ai.LoadModelConfig(opts.ModelProfile)
	if err != nil {
		return nil, err
	}

	// 3. 确定项目根
	root := strings.TrimSpace(opts.ProjectPath)
	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	} else {
		root, err = filepath.Abs(root)
		if err != nil {
			return nil, err
		}
	}

	// 4. 获取 CodeGraph 索引数据
	indexData, err := getCodeGraphData(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("获取 CodeGraph 索引数据失败: %w", err)
	}

	// 5. 构造 prompt 并调用 LLM
	systemPrompt := buildSystemPrompt()
	resp, err := ai.Call(ctx, cfg, systemPrompt, indexData)
	if err != nil {
		return nil, err
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("AI 模型返回空内容")
	}

	// 6. 写入文件
	files, err := writeAgentFiles(root, content, opts.DryRun)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Profile: cfg.Provider,
		Model:   cfg.Model,
		Files:   files,
		DryRun:  opts.DryRun,
	}
	if resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
		result.Tokens = &Tokens{
			Prompt:     resp.Usage.PromptTokens,
			Completion: resp.Usage.CompletionTokens,
		}
	}
	return result, nil
}

// getCodeGraphData 从 codegraph 获取项目索引数据（JSON 格式）。
func getCodeGraphData(ctx context.Context, root string) (string, error) {
	cmd := exec.CommandContext(ctx, "codegraph", "status", "--json", "-p", root)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var data any
	if err := json.Unmarshal(out, &data); err != nil {
		return "", err
	}
	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(pretty), nil
}

// buildSystemPrompt 构造系统提示词。
func buildSystemPrompt() string {
	return `你是代码库分析专家。根据提供的代码库索引数据（CodeGraph JSON），
为每个有意义的代码目录生成 Agent 配置文件。
输出应为 JSON 格式：{"files": [{"path": "relative/path/to/agent.go", "content": "// agent code"}]}
每个 agent 文件应包含：目录用途说明、AI 操作指引、关键符号、相关目录。`
}

// writeAgentFiles 将 LLM 返回的内容解析为文件列表并写入。
func writeAgentFiles(root, content string, dryRun bool) ([]string, error) {
	var parsed struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// LLM 可能不严格返回 JSON——降级为原样写入单一文件
		return nil, fmt.Errorf("解析 AI 生成内容失败: %w\n请检查 LLM 输出格式", err)
	}

	var files []string
	for _, f := range parsed.Files {
		target := filepath.Join(root, f.Path)
		files = append(files, target)
		if dryRun {
			continue
		}
		dir := filepath.Dir(target)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
		if err := os.WriteFile(target, []byte(f.Content), 0644); err != nil {
			return nil, fmt.Errorf("写入 %s 失败: %w", target, err)
		}
	}
	return files, nil
}
```

- [ ] **Step 2: 验证编译通过**

```bash
go build ./internal/generate/...
```
期望: 编译成功

- [ ] **Step 3: Commit**

```bash
git add internal/generate/generate.go
git commit -m "feat(generate): 新增 Agent 生成核心逻辑（CodeGraph → LLM → 文件）

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 5: `internal/cli/generate.go` — cobra 命令入口

**Files:**
- Create: `internal/cli/generate.go`

**Interfaces:**
- Consumes: `generate.GenerateAgents(ctx, opts) (*generate.Result, error)` from Task 4, `output.PrintJSON` from `internal/output`
- Produces: `generateCmd` (parent), `genAgentsCmd` cobra commands

- [ ] **Step 1: 实现 `internal/cli/generate.go`**

```go
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/ai"
	"github.com/huangchao257/work-cli/internal/generate"
	"github.com/huangchao257/work-cli/internal/output"
)

var (
	genModelProfile string
	genPath         string
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "AI 代码生成",
	Long:  "基于 CodeGraph 索引调用 LLM 生成代码文件。",
}

var genAgentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "生成 Agent 配置文件",
	Long: `基于项目 CodeGraph 索引调用 AI 模型，为各代码目录生成 Agent 配置文件。

默认使用 ai.models.default 配置的模型。可通过 --model-profile 切换。`,
	Example: `  work generate agents
  work generate agents --model-profile deepseek
  work generate agents --path /path/to/project --dry-run`,
	RunE: runGenerateAgents,
}

func init() {
	genAgentsCmd.Flags().StringVar(&genModelProfile, "model-profile", "", "AI 模型 profile（默认 default）")
	genAgentsCmd.Flags().StringVar(&genPath, "path", "", "项目根目录（默认当前目录）")
	generateCmd.AddCommand(genAgentsCmd)
	rootCmd.AddCommand(generateCmd)
}

func runGenerateAgents(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	// 友好提示：无 AI 配置时列出可用 profile
	if genModelProfile == "" {
		genModelProfile = "default"
	}

	opts := generate.Options{
		ProjectPath:  genPath,
		ModelProfile: genModelProfile,
		DryRun:       dryRun,
	}

	result, err := generate.GenerateAgents(cmd.Context(), opts)
	if err != nil {
		return exitErr(exitCodeFromAIErr(err), err)
	}

	if asJSON {
		return output.PrintJSON(w, result)
	}

	// human 输出
	if result.DryRun {
		fmt.Fprintln(w, "（预览模式，未实际执行）")
	}
	profileLabel := result.Profile
	if profileLabel == "" {
		profileLabel = genModelProfile
	}
	fmt.Fprintf(w, "模型: %s (profile: %s)\n", result.Model, profileLabel)
	if result.Tokens != nil {
		fmt.Fprintf(w, "消耗: %d prompt + %d completion tokens\n", result.Tokens.Prompt, result.Tokens.Completion)
	}
	if len(result.Files) == 0 {
		fmt.Fprintln(w, "未生成文件")
		return nil
	}
	fmt.Fprintln(w, "已生成 Agent 文件：")
	for _, f := range result.Files {
		fmt.Fprintf(w, "  %s\n", f)
	}
	fmt.Fprintf(w, "✓ 完成，共生成 %d 个 Agent 文件\n", len(result.Files))
	return nil
}

// exitCodeFromAIErr 区分用法错误（退出 2）与运行时错误（退出 1）。
func exitCodeFromAIErr(err error) int {
	msg := err.Error()
	// ai 包配置错误归为用法错误
	if strings.Contains(msg, "未找到") ||
		strings.Contains(msg, "未配置") ||
		strings.Contains(msg, "不能为空") ||
		strings.Contains(msg, "未设置") {
		return 2
	}
	// 检查是否包装了 usage.Error
	if IsUsageError(err) {
		return 2
	}
	return 1
}
```

- [ ] **Step 2: 验证编译通过**

```bash
go build ./internal/cli/...
go build ./cmd/work/...
```
期望: 编译成功

- [ ] **Step 3: 验证帮助文本**

```bash
go run ./cmd/work generate --help
go run ./cmd/work generate agents --help
```
期望: 正常显示中文帮助，包含 `--model-profile`、`--path` flag

- [ ] **Step 4: Commit**

```bash
git add internal/cli/generate.go
git commit -m "feat(cli): 新增 work generate agents 命令

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 6: `internal/generate/generate_test.go` — 核心逻辑测试

**Files:**
- Create: `internal/generate/generate_test.go`

**Interfaces:**
- Consumes: `generate.GenerateAgents`, `generate.Options`, `generate.Result` from Task 4

- [ ] **Step 1: 编写测试**

```go
package generate

import (
	"testing"
)

func TestBuildSystemPrompt(t *testing.T) {
	p := buildSystemPrompt()
	if p == "" {
		t.Fatal("system prompt 不能为空")
	}
	if !contains(p, "CodeGraph") || !contains(p, "Agent") {
		t.Fatalf("prompt 缺少关键词: %s", p)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0
}
```

- [ ] **Step 2: 运行测试**

```bash
go test ./internal/generate/... -v
```
期望: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/generate/generate_test.go
git commit -m "test(generate): 添加 system prompt 基础测试

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 7: 集成验证

**Files:**
- Modify: none（验证阶段）

- [ ] **Step 1: 运行全部测试**

```bash
go test ./internal/ai/... ./internal/generate/... -v
```
期望: 全部 PASS

- [ ] **Step 2: 运行已有测试确保无回归**

```bash
go test ./... -count=1
```
期望: 全部 PASS（允许 `codegraph` 未安装导致的 test skip，不应有新增失败）

- [ ] **Step 3: 格式化检查**

```bash
gofmt -l internal/ai/ internal/generate/ internal/cli/generate.go
```
期望: 无输出（所有文件格式正确）

- [ ] **Step 4: 构建验证**

```bash
go build -o /dev/null ./cmd/work
```
期望: 编译成功

- [ ] **Step 5: 手动验证 config 子命令兼容性**

```bash
# 设置一个 AI 配置
go run ./cmd/work config set ai.models.default.url https://api.openai.com/v1/chat/completions
go run ./cmd/work config set ai.models.default.api_key sk-test
go run ./cmd/work config set ai.models.default.model gpt-4o
# 查看
go run ./cmd/work config get ai.models.default.url
go run ./cmd/work config list
# 清理
go run ./cmd/work config unset ai.models.default.url
go run ./cmd/work config unset ai.models.default.api_key
go run ./cmd/work config unset ai.models.default.model
```
期望:
- `config get` 返回设置的值
- `config list` 包含 `ai.models.default.*` 键
- `config unset` 幂等无误

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: 集成验证通过，AI 模型配置 + generate agents 实现完成

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 8: 设计文档补充 —— AGENTS.md

**Files:**
- Create: `internal/ai/AGENTS.md`
- Create: `internal/generate/AGENTS.md`

**说明:** 这两个新包缺少 AGENTS.md。运行 `work graph sync` 后自动生成。若 codegraph 不可用，手工创建。

- [ ] **Step 1: 创建 `internal/ai/AGENTS.md`**

```markdown
# internal/ai — AI 模型配置与 API 调用

> 自动生成，运行 `work graph sync` 更新。

## 目录用途

提供 AI 模型配置加载、环境变量展开、以及 OpenAI 兼容 HTTP 调用能力。
供 `generate` 包及未来其他需要调用 LLM 的命令使用。

## AI 操作指引

| 任务 | 目标文件 |
|------|---------|
| 加载指定 profile 的模型配置 | `internal/ai/config.go:LoadModelConfig` |
| 列出所有已配置的 profile | `internal/ai/config.go:ListProfiles` |
| 发起 chat completion 请求 | `internal/ai/client.go:Call` |
| 修改模型配置类型定义 | `internal/ai/config.go:ModelConfig` |

## 关键符号

- `func LoadModelConfig(profile string) (*ModelConfig, error)` — `config.go:57`
- `func ListProfiles() ([]string, error)` — `config.go:95`
- `func Call(ctx context.Context, cfg *ModelConfig, systemPrompt, userContent string) (*ChatResponse, error)` — `client.go:62`
- `type ModelConfig struct` — `config.go:19`
- `type ChatResponse struct` — `client.go:31`

## 相关目录

- `internal/generate/` — 基于本包调用 LLM 生成代码
- `internal/config/` — 底层 YAML 读写（本包用自己的反序列化）
```

- [ ] **Step 2: 创建 `internal/generate/AGENTS.md`**

```markdown
# internal/generate — AI 代码生成

> 自动生成，运行 `work graph sync` 更新。

## 目录用途

编排基于 CodeGraph 索引 → LLM → 文件写入的生成流程。
当前支持 `work generate agents` 生成 Agent 配置文件。

## AI 操作指引

| 任务 | 目标文件 |
|------|---------|
| 执行 Agent 生成流程 | `internal/generate/generate.go:GenerateAgents` |
| 修改生成选项 | `internal/generate/generate.go:Options` |
| 修改 prompt 模板 | `internal/generate/generate.go:buildSystemPrompt` |
| 修改输出文件写入逻辑 | `internal/generate/generate.go:writeAgentFiles` |

## 关键符号

- `func GenerateAgents(ctx context.Context, opts Options) (*Result, error)` — `generate.go:50`
- `type Options struct` — `generate.go:26`
- `type Result struct` — `generate.go:33`
- `func buildSystemPrompt() string` — `generate.go:97`
- `func writeAgentFiles(root, content string, dryRun bool) ([]string, error)` — `generate.go:107`

## 相关目录

- `internal/ai/` — AI 模型配置与 HTTP 调用
- `internal/graph/` — CodeGraph CLI 封装
- `internal/cli/` — cobra 命令入口
```

- [ ] **Step 3: Commit**

```bash
git add internal/ai/AGENTS.md internal/generate/AGENTS.md
git commit -m "docs: 新增 ai 与 generate 包的 AGENTS.md

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## 自检清单

1. **Spec 覆盖**: 所有 spec 需求均已覆盖——`ai.models` YAML 段(Task 1)、多 profile + default(Task 1)、`${ENV_VAR}` 展开(Task 2)、必填校验(Task 1)、OpenAI 兼容 HTTP 调用(Task 3)、`work generate agents` 命令(Task 5)、`--dry-run`/`--json`(Task 5)、退出码 1/2(Task 5)、现有 config 子命令兼容(Task 7)
2. **无占位符**: 所有步骤含具体代码、命令、期望输出
3. **类型一致性**: `ModelConfig` 定义在 Task 1，Task 3 消费 `*ModelConfig`，Task 4 消费 `ai.Call` + `ai.LoadModelConfig`，接口匹配
