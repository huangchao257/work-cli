# work-cli Hooks 模块设计规格书

> 状态：已实现（2026-06-11）— **阶段一：观察型上报**  
> 范围：**Hooks 模块** — AI IDE hooks 事件采集上报（本地 + 异步内网）与 hooks 套装自动安装

## 1. 概述

### 1.0 演进路线

| 阶段 | 能力 | 状态 |
|------|------|------|
| **阶段一** | **观察型上报** — 采集 IDE 事件，本地落盘 + 异步上报内网 Telemetry | **当前（MVP）** |
| **阶段二** | **执行审计** — 基于上报数据或策略引擎，对 Shell/MCP/文件操作等进行合规审计与告警 | 规划中 |
| **阶段三** | **触发执行自动化** — hook 在满足条件时触发公司自动化流程（审批、阻断、回调 Webhook、联动 CI 等） | 规划中 |

阶段一 deliberately 采用「透传 stdin/stdout、exit 0」的观察型设计，为后续审计与自动化预留同一套 `hooks.yaml` 安装与事件模型，但**不在 MVP 中改变 IDE 执行行为**。

### 1.1 目标

在 `work` 统一 CLI 上新增 **Hooks 模块**，帮助公司全体员工：

1. **自动安装 hooks 能力** — 通过独立 `hooks.yaml` 套装，一键写入 Cursor、Qoder、Claude Code 的 hooks 配置与脚本
2. **（阶段一）采集并上报 AI IDE 事件** — IDE hook 触发时经 `work hooks report` 落盘本地队列，再异步上报内网 Telemetry 服务；内网不可达时仅保留本地，不阻断 IDE 使用

与现有**资源管理模块**（bundle / cli）并列，共用 `work install` / `list` / `uninstall` / `update` 入口。

### 1.2 已确认决策

| 维度 | 决策 |
|------|------|
| 上报策略 | 本地先落盘，再异步上报内网；不可达时仅本地保留 |
| IDE 范围 | Cursor、Qoder、Claude Code（与资源模块对齐） |
| 安装方式 | 独立 `hooks.yaml` 套装，**不**并入 `bundle.yaml` |
| 事件范围 | 可配置；默认「核心审计集」 |
| 实现方案 | 统一上报脚本 + `work hooks report`（方案 A） |
| 安装范围 | `--scope user\|project`，默认 `user` |
| 认证 | MVP 不做；Telemetry API 预留 Header 扩展点 |

### 1.3 非目标（阶段一 / MVP）

- **执行审计**（服务端策略分析、合规告警）— 阶段二
- **触发执行自动化**（阻断、审批、Webhook、联动外部系统）— 阶段三
- 实时阻断/审批类 hooks（阶段一仅观察型上报，透传 stdin/stdout）
- 内网 Telemetry 服务端实现（仅定义客户端 API 契约）
- HTTP hook 类型 / 本地常驻 daemon
- 事件全文存储（默认脱敏 prompt、文件内容等敏感字段）
- Windows arm64
- 用户认证与权限控制

---

## 2. 整体架构

```
AI IDE（hook 触发）
        │ stdin JSON
        ▼
┌─────────────────────────────┐
│  telemetry.sh（套装脚本）     │
│  调用 work hooks report       │
└─────────────────────────────┘
        │
        ▼
┌─────────────────────────────┐
│  work hooks report           │
│  - 解析/校验 payload          │
│  - 脱敏                       │
│  - 写入本地队列               │
│  - 触发异步 sync（非阻塞）     │
└─────────────────────────────┘
        │
   ┌────┴────┐
   ▼         ▼
本地队列    Telemetry Client
queue.jsonl  POST /v1/events
             （失败保留队列重试）
```

### 2.1 与现有架构的关系

```
work install <ref>
        │
        ▼
Install Engine（扩展）
  - hooks.yaml → KindHooks → Hooks Engine
  - bundle.yaml → KindBundle（不变）
  - installer.yaml → KindCLI（不变）
        │
        ▼
Hooks Engine + IDE Adapters
  - 复制脚本到 IDE hooks 目录
  - merge hooks 配置（按 id，不覆盖用户自有 hooks）
```

### 2.2 核心原则

1. **观察型 hook**：上报脚本在 `work hooks report` 后透传原始 stdin，默认 `exit 0`，不修改 IDE 行为。
2. **可配置事件**：抽象事件名在 manifest / config 中配置，Adapter 映射到各 IDE 真实事件名与 matcher。
3. **本地优先**：任何网络失败不得导致 hook 脚本非零退出（除非用户显式设置 `failClosed`，MVP 不提供）。
4. **可卸载**：`uninstall` 仅移除 `work` 管理的 hooks 条目（`managed_by: work` 标记）。

---

## 3. 命令设计

### 3.1 新增/扩展命令

| 命令 | 作用 | 示例 |
|------|------|------|
| `work install <ref>` | 扩展支持 hooks 套装 | `work install company-hooks` |
| `work hooks report` | 接收 IDE hook stdin，写入队列 | （由脚本调用，非用户直接执行） |
| `work hooks sync` | 手动将本地队列上报内网 | `work hooks sync` |
| `work hooks status` | 查看队列积压与上次同步状态 | `work hooks status` |
| `work list` | 扩展显示 `kind: hooks` | `work list --kind hooks` |
| `work uninstall <name>` | 扩展卸载 hooks 套装 | `work uninstall company-hooks` |
| `work update [name]` | 扩展更新 hooks 套装 | `work update company-hooks` |

### 3.2 `work hooks report` 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `--ide` | 是 | `cursor` / `qoder` / `claude` |
| `--event` | 是 | 抽象事件名（见 §5.2）或 IDE 原始事件名 |
| `--hooks-kit` | 否 | 来源套装名，默认从已安装状态推断 |
| `--stdin-file` | 否 | 调试：从文件读入代替 stdin |

行为：

1. 从 stdin 读取 IDE 传入的 JSON
2. 合并元数据（时间戳、机器 ID、用户、项目根目录）
3. 按配置脱敏
4. 追加写入 `~/.work/telemetry/queue.jsonl`
5. 若 `telemetry.enabled` 且距上次 sync 超过阈值，后台触发一次 `sync`（单进程内防抖，不阻塞 hook）
6. 将原始 stdin 写入 stdout（透传），退出码 0

超时：默认 3 秒；超时仍 exit 0，仅写本地警告日志。

### 3.3 `work hooks sync`

1. 读取 `queue.jsonl` 中 `uploaded_at` 为空的记录
2. 按 `batch_size` 分批 `POST` 至 `telemetry.url`
3. 成功：标记已上传或移至 `archive/`
4. 失败：保留队列，更新 `last_error` 与 `retry_after`（指数退避）

### 3.4 `work hooks status`

输出（人类可读 / `--json`）：

- 待上报条数
- 最旧未上报事件时间
- 上次成功同步时间
- 上次错误信息
- `telemetry.enabled` / `telemetry.url` 是否配置

---

## 4. Manifest 格式（`hooks.yaml`）

hooks 套装根目录包含 `hooks.yaml`：

```yaml
type: hooks
name: company-hooks
version: 1.0.0
description: 公司 AI IDE 事件上报 hooks

env:
  - name: WORK_TELEMETRY_URL
    description: 可选，覆盖 config 中的 telemetry.url
    required: false

telemetry:
  preset: audit                    # 默认核心审计集
  # events: [shell, mcp, file_read, file_edit, prompt]  # 覆盖 preset
  redact: [prompt, file_content]   # 额外脱敏字段

resources:
  hooks:
    - id: work-telemetry
      source: ./scripts/telemetry.sh

targets: [qoder, cursor, claude]   # 可选；省略则三家都装
```

### 4.1 字段说明

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 是 | 固定 `hooks` |
| `name` | 是 | 套装唯一标识 |
| `version` | 是 | 语义化版本 |
| `description` | 否 | 人类可读描述 |
| `env` | 否 | 安装前环境变量检查（规则同 bundle） |
| `telemetry.preset` | 否 | `audit`（默认）或 `all` |
| `telemetry.events` | 否 | 抽象事件列表，覆盖 `preset` |
| `telemetry.redact` | 否 | 额外脱敏字段名 |
| `resources.hooks` | 是 | 至少一项；MVP 仅需 `work-telemetry` |
| `targets` | 否 | 限制目标 IDE |

### 4.2 Hook 资源项

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | 稳定标识，用于 merge / uninstall |
| `source` | 是 | 相对套装根目录的脚本路径 |

### 4.3 本地目录识别

扩展 `DetectKind` 优先级：

1. `installer.yaml` → `cli`
2. `hooks.yaml` → `hooks`
3. `bundle.yaml` → `bundle`

---

## 5. 事件模型

### 5.1 抽象事件（配置用）

| 抽象名 | 含义 |
|--------|------|
| `shell` | Shell / Bash 命令执行前后 |
| `mcp` | MCP 工具调用 |
| `file_read` | 文件读取 |
| `file_edit` | 文件写入/编辑 |
| `prompt` | 用户提交 Prompt |
| `session` | 会话开始/结束（可选，默认不在 audit 集） |
| `tool` | 通用工具前后（可选） |

### 5.2 预设

**`audit`（默认）**：`shell`, `mcp`, `file_read`, `file_edit`, `prompt`

**`all`**：该 IDE 支持且 Adapter 已实现映射的全部事件

### 5.3 IDE 事件映射表

| 抽象事件 | Cursor | Qoder | Claude Code |
|----------|--------|-------|-------------|
| `shell` | `beforeShellExecution`, `afterShellExecution` | `PreToolUse`/`PostToolUse` + matcher `Bash` | 同 Qoder |
| `mcp` | `beforeMCPExecution`, `afterMCPExecution` | `PreToolUse`/`PostToolUse` + matcher `MCP.*` 或 `mcp__.*` | 同 Qoder |
| `file_read` | `beforeReadFile` | `PreToolUse` + matcher `Read` | 同 Qoder |
| `file_edit` | `afterFileEdit` | `PostToolUse` + matcher `Write\|Edit` | 同 Qoder |
| `prompt` | `beforeSubmitPrompt` | `UserPromptSubmit` | `UserPromptSubmit` |
| `session` | `sessionStart`, `sessionEnd` | *不支持 sessionStart*；可用 `Notification` 等替代时文档说明 | `SessionStart`, `SessionEnd` |
| `tool` | `preToolUse`, `postToolUse` | `PreToolUse`, `PostToolUse` | 同 Qoder |

Adapter 负责：根据 `telemetry.events` 生成各 IDE 的 hook 配置条目；Qoder 不支持的抽象事件跳过并输出 warning。

### 5.4 上报记录（EventRecord）

```json
{
  "event_id": "uuid-v4",
  "timestamp": "2026-06-11T10:00:00Z",
  "ide": "cursor",
  "abstract_event": "shell",
  "ide_event": "beforeShellExecution",
  "hooks_kit": "company-hooks",
  "hooks_kit_version": "1.0.0",
  "scope": "user",
  "user": "zhangsan",
  "machine_id": "sha256-of-stable-id",
  "project_root": "/home/user/project",
  "session_id": "from-payload-if-present",
  "payload": { }
}
```

`payload` 为脱敏后的 IDE 原始 JSON（移除或替换 `redact` 列表中的字段）。

---

## 6. IDE 适配器（Hooks）

### 6.1 接口扩展

```go
type HooksAdapter interface {
    Name() string
    Detect() bool
    HooksConfigPath(scope Scope) (string, error)
    HooksScriptDir(scope Scope, kitID string) (string, error)
    InstallHooks(ctx context.Context, cfg HooksInstallConfig, scope Scope) error
    UninstallHooks(ctx context.Context, rec HooksInstallState, scope Scope) error
}
```

各 IDE 实现可嵌入现有 `Adapter`，或独立 `internal/adapter/hooks/` 子包。

### 6.2 路径映射（用户级）

| IDE | hooks 配置 | 脚本目录 |
|-----|------------|----------|
| Cursor | `~/.cursor/hooks.json` | `~/.cursor/hooks/work-telemetry/` |
| Qoder | `~/.qoder/settings.json` → `hooks` | `~/.qoder/hooks/work-telemetry/` |
| Claude | `~/.claude/settings.json` → `hooks` | `~/.claude/hooks/work-telemetry/` |

项目级（`--scope project`）：

| IDE | hooks 配置 | 脚本目录 |
|-----|------------|----------|
| Cursor | `<root>/.cursor/hooks.json` | `<root>/.cursor/hooks/work-telemetry/` |
| Qoder | `<root>/.qoder/settings.json` | `<root>/.qoder/hooks/work-telemetry/` |
| Claude | `<root>/.claude/settings.json` | `<root>/.claude/hooks/work-telemetry/` |

### 6.3 配置格式差异

**Cursor**（`hooks.json`，version 1）：

```json
{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {
        "command": "./hooks/work-telemetry/telemetry.sh",
        "managed_by": "work",
        "work_id": "work-telemetry",
        "timeout": 3
      }
    ]
  }
}
```

**Qoder / Claude**（`settings.json` 内 `hooks` 键）：

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "~/.qoder/hooks/work-telemetry/telemetry.sh",
            "timeout": 3,
            "managed_by": "work",
            "work_id": "work-telemetry"
          }
        ]
      }
    ]
  }
}
```

说明：

- `managed_by` / `work_id` 为 work 扩展字段；写入时保留，卸载时按 `work_id` 精确删除。
- 若目标 IDE 忽略未知 JSON 字段，卸载依赖安装时写入的 sidecar 状态文件 `~/.work/hooks-installed/{name}.json` 记录条目指纹。

### 6.4 Merge 策略

1. 读取现有配置；不存在则创建骨架。
2. 按 `work_id` 删除旧 work 条目，再插入新条目（update 时幂等）。
3. 不删除无 `managed_by: work` 的用户条目。
4. 同一事件多个 matcher：work 条目追加在数组末尾。

### 6.5 telemetry.sh 行为（套装脚本）

```bash
#!/usr/bin/env bash
# 从环境变量或参数获取 IDE / EVENT（安装时由 Adapter 写入包装脚本或硬编码）
input=$(cat)
work hooks report --ide "${WORK_HOOKS_IDE}" --event "${WORK_HOOKS_EVENT}" || true
echo "$input"
exit 0
```

安装时 Adapter 为每个映射事件生成调用包装（或单一脚本 + 环境变量），确保 `work` 在 PATH 中；若不在 PATH，安装阶段警告并写入绝对路径（`which work`）。

---

## 7. 配置与本地存储

### 7.1 用户配置（`~/.work/config.yaml` 扩展）

```yaml
telemetry:
  enabled: true
  url: https://telemetry.internal.example.com/v1/events
  batch_size: 50
  sync_interval: 5m
  max_retries: 10
  events: [shell, mcp, file_read, file_edit, prompt]  # 覆盖套装默认
  redact:
    - prompt
    - file_content
    - tool_input.content
    - env_secrets
```

优先级：`hooks.yaml` 的 `telemetry.events` < `config.yaml` 的 `telemetry.events`（用户配置优先）。

### 7.2 本地文件布局

```
~/.work/
├── config.yaml
├── installed.json
├── telemetry/
│   ├── queue.jsonl          # 待上报 / 失败重试
│   ├── archive/             # 已上报归档（按日滚动）
│   └── state.json           # last_sync, pending_count, last_error
└── hooks-installed/
    └── company-hooks.json   # 安装指纹（配置路径、条目列表）
```

### 7.3 queue.jsonl 行格式

```json
{"event":{...EventRecord...},"uploaded_at":null,"retry_count":0,"last_error":""}
```

---

## 8. Telemetry API 契约（MVP）

### 8.1 上报

```
POST {telemetry.url}   # 默认路径 /v1/events，可在 url 中写全路径
Content-Type: application/json
Accept: application/json

{
  "client": "work-cli",
  "client_version": "0.2.0",
  "events": [ { ...EventRecord }, ... ]
}
```

### 8.2 响应

| 状态码 | 含义 |
|--------|------|
| `202` / `200` | 接收成功，客户端标记已上传 |
| `4xx` | 客户端错误，记录 `last_error`，超过 `max_retries` 后跳过并写 `dead_letter.jsonl` |
| `5xx` / 网络错误 | 保留队列，按指数退避重试 |

### 8.3 认证扩展（预留）

```go
type TelemetryAuthenticator interface {
    HTTPHeaders() map[string]string
}
```

MVP 默认空实现；后续可接 API Key / SSO。

---

## 9. 状态文件扩展

`installed.json` 中 `kind: "hooks"` 记录示例：

```json
{
  "name": "company-hooks",
  "kind": "hooks",
  "version": "1.0.0",
  "scope": "user",
  "ref": "registry:company-hooks",
  "installed_at": "2026-06-11T10:00:00Z",
  "ides": ["cursor", "qoder", "claude"],
  "resources": {
    "hooks": ["work-telemetry"]
  },
  "telemetry": {
    "events": ["shell", "mcp", "file_read", "file_edit", "prompt"]
  }
}
```

---

## 10. 核心流程

### 10.1 install（hooks）

1. 解析 ref，识别 `hooks.yaml`
2. 检查 `env`
3. `--dry-run`：打印将写入的 IDE 路径与事件列表
4. 解析 `telemetry.events`（preset + config 覆盖）
5. 对各已检测 IDE：复制脚本 → merge hooks 配置 → 写 sidecar 指纹
6. 写入 `installed.json`（`kind: hooks`）
7. 输出中文友好结果

### 10.2 uninstall

1. 读取状态与 sidecar 指纹
2. 各 IDE Adapter 按 `work_id` 移除条目
3. 删除脚本目录（若空则保留父目录）
4. 删除 sidecar 与状态记录

### 10.3 update

同 bundle：先 uninstall 再 install（同 scope）。

---

## 11. Monorepo 目录结构（新增）

```
work-cli/
├── internal/
│   ├── hooks/
│   │   ├── manifest.go       # hooks.yaml 解析
│   │   ├── events.go         # 抽象事件与 preset
│   │   ├── report.go         # report 命令逻辑
│   │   ├── queue.go          # 本地队列读写
│   │   ├── sync.go           # 上报与重试
│   │   ├── redact.go         # 脱敏
│   │   └── merge/            # 各 IDE hooks JSON merge
│   ├── engine/
│   │   └── hooks.go          # hooks 安装分支
│   ├── adapter/
│   │   ├── hooks_cursor.go
│   │   ├── hooks_qoder.go
│   │   └── hooks_claude.go
│   └── cli/
│       └── hooks.go          # hooks 子命令
├── examples/
│   └── company-hooks/
│       ├── hooks.yaml
│       └── scripts/
│           └── telemetry.sh
└── docs/superpowers/specs/
    └── 2026-06-11-hooks-module-design.md
```

---

## 12. 错误处理与输出

### 12.1 退出码

| 场景 | 退出码 |
|------|--------|
| `hooks report` 成功 | 0（始终，避免阻断 IDE） |
| `hooks sync` 部分失败 | 1 |
| install 失败 | 1 |
| 指定 IDE 未检测到且 `--ide` 显式指定 | 3 |

### 12.2 人类可读输出示例

- 成功：`✓ 已安装 company-hooks v1.0.0 → cursor, qoder（事件：shell, mcp, file_read, file_edit, prompt）`
- 警告：`⚠ Qoder 不支持 session 事件，已跳过`
- 状态：`待上报 42 条 · 上次同步 10 分钟前 · 内网可达`

---

## 13. 测试策略

| 层级 | 内容 |
|------|------|
| 单元测试 | 事件映射、脱敏、queue 读写、JSON merge（三家格式） |
| 集成测试 | `examples/company-hooks` 安装到临时 HOME，`--dry-run` 与 merge 快照 |
| 手工清单 | 三家 IDE 各触发 shell / 编辑 / prompt，检查 `queue.jsonl` 与 `hooks status` |

---

## 14. Registry 扩展

Registry 包类型增加 `hooks`：

```json
{
  "name": "company-hooks",
  "type": "hooks",
  "version": "1.0.0",
  "download_url": "https://.../company-hooks-1.0.0.zip",
  "checksum": "sha256:..."
}
```

---

## 15. 安全与隐私

1. 默认脱敏：`prompt`、文件全文、`tool_input` 中的大段文本截断或哈希。
2. 不上报环境变量值；仅上报变量名（若 payload 含 env）。
3. `machine_id` 使用稳定但不可逆的哈希，不含主机名明文（可配置）。
4. 本地 `queue.jsonl` 权限 `0600`，目录 `0700`。

---

## 16. 后续扩展

### 16.1 阶段二：执行审计

在阶段一上报数据基础上，于 Telemetry 服务端或本地策略引擎实现：

| 能力 | 说明 |
|------|------|
| 合规规则引擎 | 对 `shell` / `mcp` / `file_edit` 等事件匹配公司策略（危险命令、敏感路径、外发数据等） |
| 审计留痕与检索 | 按用户、项目、时间、事件类型查询；与现有 `EventRecord` 模型对齐 |
| 告警与报表 | 违规/高风险操作通知安全或直属负责人 |

客户端可能扩展：`work hooks audit`（本地策略预览）、manifest 中 `telemetry.audit_rules` 引用等；**不要求** hook 脚本阻断 IDE（审计以旁路分析为主）。

### 16.2 阶段三：触发执行自动化

hook 从「只观察」升级为「可干预、可联动」：

| 能力 | 说明 |
|------|------|
| 阻断型 hooks | 危险命令 deny、敏感文件写保护；非零退出码或修改 payload 阻止 IDE 继续 |
| 审批流 | 高风险操作挂起，等待内网审批 API 放行后再透传 |
| 自动化触发 | Webhook / 内部自动化平台（如触发 CI、工单、通知机器人） |
| `failClosed` 策略 | 网络或策略服务不可达时的默认行为（允许 / 拒绝）可配置 |

实现上可复用阶段一的 `telemetry.sh` 与事件映射，新增 `hooks.yaml` 资源类型（如 `audit-guard.sh`）及 `work hooks evaluate` 等命令；需单独评审对 IDE 体验的影响。

### 16.3 其他（P1+）

| 功能 | 说明 |
|------|------|
| `work doctor --hooks` | 检查 hooks 是否加载、work 是否在 PATH |
| 项目级默认套装 | IT 推送 + `work install` |
| 对接 Vault | 上报 / 审计 / 自动化 API 自动带短期 token |
| 更多 IDE | VS Code Copilot 等 |
