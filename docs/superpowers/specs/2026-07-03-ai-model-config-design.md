# AI 模型配置 + 生成 Agent 功能设计

> 状态：待评审（2026-07-03）
> 范围：支持在 `~/.work/config.yaml` 配置多个 AI 模型 profile，新增 `work generate agents` 命令基于 CodeGraph 调用 LLM 生成 Agent 文件

## 1. 目标

1. 在 `~/.work/config.yaml` 中配置 AI 模型（多 profile），供 work CLI 各命令调用 LLM 时统一读取
2. API 密钥支持 `${ENV_VAR}` 引用 + 明文两种方式
3. 新增 `work generate agents` 命令，基于 CodeGraph 索引调用 LLM，为项目生成 Agent 文件

## 2. 已确认决策

| 维度 | 决策 |
|------|------|
| 配置文件 | 复用 `~/.work/config.yaml`，新增 `ai.models` 顶级段 |
| 多 profile | 支持多个命名 profile，`default` 为默认 |
| 必填字段 | `url`、`api_key`、`model` |
| 可选字段 | `provider`、`timeout`、`max_tokens`、`extra_headers` |
| 环境变量展开 | `${ENV_VAR}` 语法，仅对 `api_key` 字段展开，未设置即报错 |
| CLI 入口 | 新增 `work generate agents` 独立命令（非 `graph` 子命令） |
| LLM 调用方式 | OpenAI 兼容 API 格式，标准 HTTP 请求 |
| `graph init/sync` | 保持现有行为，不加 AI |

## 3. YAML 配置结构

```yaml
# ~/.work/config.yaml
ai:
  models:
    default:
      provider: openai
      url: https://api.openai.com/v1/chat/completions
      api_key: ${OPENAI_API_KEY}
      model: gpt-4o
      timeout: 120s
      max_tokens: 4096
      extra_headers:
        X-Custom: value
    deepseek:
      provider: deepseek
      url: https://api.deepseek.com/v1/chat/completions
      api_key: ${DEEPSEEK_API_KEY}
      model: deepseek-chat
      timeout: 60s
      max_tokens: 8192
```

### 3.1 字段说明

| 字段 | 必填 | 类型 | 说明 |
|------|------|------|------|
| `provider` | 否 | string | 厂商标识，如 `openai`、`deepseek`、`anthropic` |
| `url` | **是** | string | API 地址，兼容 OpenAI 格式 `.../v1/chat/completions` |
| `api_key` | **是** | string | API 密钥，支持 `${ENV_VAR}` 语法引用环境变量 |
| `model` | **是** | string | 模型名称，如 `gpt-4o`、`deepseek-chat` |
| `timeout` | 否 | string | 请求超时，Go `time.ParseDuration` 格式，默认 `"120s"` |
| `max_tokens` | 否 | int | 默认最大输出 token 数，0 表示不设 |
| `extra_headers` | 否 | map[string]string | 额外 HTTP 头，注入到 API 请求中 |

### 3.2 与既有配置的共存

`ai.models` 与现有 `registry`、`self_update`、`telemetry`、`cache` 平级，不冲突。`work config get/set/list/unset` 天然兼容，无需改动 `internal/config/config.go`。

## 4. 包结构与 Go 类型

### 4.1 新包 `internal/ai/`

```
internal/ai/
├── config.go          # 类型定义 + LoadModelConfig / ListProfiles
└── config_test.go     # 单元测试
```

### 4.2 类型定义

```go
package ai

type ModelConfig struct {
    Provider     string            `yaml:"provider"`
    URL          string            `yaml:"url"`
    APIKey       string            `yaml:"api_key"`
    Model        string            `yaml:"model"`
    Timeout      string            `yaml:"timeout"`
    MaxTokens    int               `yaml:"max_tokens"`
    ExtraHeaders map[string]string `yaml:"extra_headers"`
}

type modelsFileConfig struct {
    AI struct {
        Models map[string]ModelConfig `yaml:"models"`
    } `yaml:"ai"`
}
```

### 4.3 核心函数

```go
// LoadModelConfig 加载指定 profile，profile 为空时取 "default"。
// 自动展开 api_key 中的 ${ENV_VAR}。必填字段缺失时返回错误。
func LoadModelConfig(profile string) (*ModelConfig, error)

// ListProfiles 列出所有已配置的 profile 名称，用于 help / 诊断。
func ListProfiles() ([]string, error)
```

### 4.4 实现要点

1. 从 `~/.work/config.yaml` 反序列化为 `modelsFileConfig`，取 `.AI.Models`
2. profile 为空 → 取 `"default"`；profile 不存在 → 返回明确错误
3. 配置文件不存在 → 返回错误：`未找到 ~/.work/config.yaml，请先配置`
4. 必填字段校验（`url`、`api_key`、`model`），缺失 → 返回错误
5. `${ENV_VAR}` 展开：正则匹配 `${...}`，调 `os.Getenv`；环境变量未设 → 返回错误，提示具体 profile 和变量名
6. 忽略 `api_key` 明文中的空白（`strings.TrimSpace`）
7. timeout 解析失败 → 使用默认 `120s`

### 4.5 环境变量展开细节

仅对 `api_key` 字段展开。支持的语法：

```
api_key: ${OPENAI_KEY}           # 纯环境变量
api_key: sk-xxx                  # 纯明文
```

不支持的语法（保持简单）：
- 混合写法如 `${BASE}-suffix`
- 多变量引用如 `${A}${B}`
- 默认值如 `${VAR:-default}`

## 5. CLI 命令设计

### 5.1 命令结构

```bash
work generate agents [--model-profile <name>] [--path <dir>] [--dry-run] [--json]
```

新增文件：
- `internal/cli/generate.go` — `generateCmd` + `genAgentsCmd`
- `internal/generate/` — 核心生成逻辑

### 5.2 命令流程

```
work generate agents
  │
  ├─ 1. 确保 codegraph 可用（exec.LookPath）
  │
  ├─ 2. ai.LoadModelConfig(profile)，profile 空 → "default"
  │     无配置 → 友好提示并退出
  │
  ├─ 3. 调 codegraph status（或 files --json）获取索引数据
  │
  ├─ 4. 构造 chat completion prompt：
  │     system: "根据代码库索引生成 Agent 配置..."
  │     user: JSON 格式的索引数据
  │
  ├─ 5. POST <url> with JSON body（OpenAI 兼容格式）
  │     Authorization: Bearer <api_key>
  │
  ├─ 6. 解析响应，写入 Agent 文件到对应目录
  │     --dry-run 时仅预览
  │
  └─ 7. 输出结果（human / --json）
```

### 5.3 Flag 说明

| Flag | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--model-profile` | string | `""` → `default` | 使用的 AI 模型 profile |
| `--path` | string | `""` → cwd | 项目根目录 |
| `--dry-run` | bool | false | 仅预览不写入（继承全局） |
| `--json` | bool | false | JSON 输出（继承全局） |

### 5.4 输出示例

**human 模式：**
```
正在分析项目索引...
模型: gpt-4o (profile: default)
已生成 Agent 文件：
  internal/cli/agent.go
  internal/engine/agent.go
✓ 完成，共生成 5 个 Agent 文件
```

**--json 模式：**
```json
{
  "profile": "default",
  "model": "gpt-4o",
  "files": ["internal/cli/agent.go", "internal/engine/agent.go"],
  "tokens": {"prompt": 1200, "completion": 450}
}
```

### 5.5 错误处理

| 错误场景 | 退出码 | 提示 |
|----------|--------|------|
| 无 AI 配置 | 2 | `未配置 AI 模型，请在 ~/.work/config.yaml 中设置 ai.models.default.*` |
| profile 不存在 | 2 | `未找到模型 profile "xxx"，可用: default, deepseek` |
| 环境变量未设 | 2 | `环境变量 OPENAI_API_KEY 未设置（在 ai.models.default.api_key 中引用）` |
| codegraph 未安装 | 1 | `未找到 codegraph，请先执行 work install codegraph-stack` |
| LLM API 调用失败 | 1 | `调用 AI 模型失败: <详情>` |

## 6. OpenAI 兼容 API 调用

### 6.1 请求格式

```json
POST <url>
Authorization: Bearer <api_key>
Content-Type: application/json

{
  "model": "<model>",
  "messages": [
    {"role": "system", "content": "<system prompt>"},
    {"role": "user", "content": "<index data>"}
  ],
  "max_tokens": <max_tokens>,
  "temperature": 0.3
}
```

### 6.2 响应解析

```json
{
  "id": "chatcmpl-xxx",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "<generated agent code>"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 1200,
    "completion_tokens": 450
  }
}
```

### 6.3 HTTP 客户端

- 新建 `internal/ai/client.go` 封装 HTTP 调用
- 使用 `context.WithTimeout`，超时取自 `ModelConfig.Timeout`
- `ModelConfig.ExtraHeaders` 注入到 HTTP 请求头
- 非 2xx 响应 → 返回 body 前 512 字节作为错误详情

## 7. 现有配置体系兼容

`internal/config` 包基于 `yaml.Node` 的点分路径读写，天然支持 `ai.models`：

```bash
work config get ai.models.default.model          # gpt-4o
work config set ai.models.default.model gpt-5    # 改动模型
work config set ai.models.claude.url https://... # 新增 profile
work config list | grep ai.models                # 查看全部 AI 配置
```

无需修改 `internal/config/config.go`。其他包（`selfupdate`、`source`）各自独立解析自己的配置段，不受影响。

## 8. 非目标

- 不实现 `work config set` 之外的"AI 配置管理"子命令（如 `work ai config`）
- 不实现 API key 加密存储（依赖 OS 文件权限 `0600` + 环境变量引用两种方式）
- 不接入非 OpenAI 兼容格式的 LLM API（如 Anthropic Messages API）
- 不在本次实现 `work generate rules`、`work generate mcp` 等其他生成类型
- 不在本次实现流式输出（streaming），按标准 chat completion 一次性请求/响应

## 9. 设计文档索引

- 总体设计：`docs/superpowers/specs/2026-06-11-work-cli-design.md`
- CodeGraph 套装：`docs/superpowers/specs/2026-06-11-codegraph-agents-design.md`
- Hooks 模块：`docs/superpowers/specs/2026-06-11-hooks-module-design.md`
