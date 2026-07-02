# Hooks 模块

> IDE hooks 套装安装与事件采集上报。跨模块共性见 [总览](../overview.md)。

## 1. 范围与演进路线

| 阶段 | 能力 | 状态 |
|------|------|------|
| **阶段一** | **观察型上报** — 采集 IDE 事件，本地落盘 + 异步上报内网 Telemetry | **当前（已实现）** |
| **阶段二** | **执行审计** — 基于上报数据或策略引擎，对 Shell/MCP/文件操作合规审计与告警 | 规划中 |
| **阶段三** | **触发执行自动化** — hook 满足条件时触发公司自动化（审批、阻断、Webhook、联动 CI） | 规划中 |

阶段一 deliberately 采用「透传 stdin/stdout、exit 0」的观察型设计，为后续审计与自动化预留同一套 `hooks.yaml` 安装与事件模型，但**不在 MVP 中改变 IDE 执行行为**。

与资源管理模块并列，共用 `work install`/`list`/`uninstall`/`update` 入口（`kind: hooks`）。

## 2. 架构

```
AI IDE（hook 触发, stdin JSON）
        │
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
│  - 合并元数据 + 脱敏          │
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

核心原则：
1. **观察型 hook**：上报脚本在 `work hooks report` 后透传原始 stdin，默认 `exit 0`，不修改 IDE 行为。
2. **可配置事件**：抽象事件名在 manifest/config 配置，Adapter 映射到各 IDE 真实事件名与 matcher。
3. **本地优先**：任何网络失败不得导致 hook 脚本非零退出（MVP 不提供 `failClosed`）。
4. **可卸载**：`uninstall` 仅移除 `work` 管理的 hooks 条目（`managed_by: work` 标记 + sidecar 指纹）。

## 3. Manifest（`hooks.yaml`）

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
  preset: audit                    # 默认核心审计集；或 all
  # events: [shell, mcp, file_read, file_edit, prompt]  # 覆盖 preset
  redact: [prompt, file_content]   # 额外脱敏字段

resources:
  hooks:
    - id: work-telemetry
      source: ./scripts/telemetry.sh

targets: [cursor, qoder, claude]   # 可选；省略则三家都装
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 是 | 固定 `hooks` |
| `name` | 是 | 套装唯一标识 |
| `version` | 是 | 语义化版本 |
| `env` | 否 | 安装前环境变量检查（规则同 bundle） |
| `telemetry.preset` | 否 | `audit`（默认）或 `all` |
| `telemetry.events` | 否 | 抽象事件列表，覆盖 `preset` |
| `telemetry.redact` | 否 | 额外脱敏字段名 |
| `resources.hooks` | 是 | 至少一项；MVP 仅需 `work-telemetry` |
| `targets` | 否 | 限制目标 IDE |

事件优先级：`hooks.yaml.telemetry.events` < `~/.work/config.yaml.telemetry.events`（用户配置优先）。

## 4. 事件模型

抽象事件：`shell`、`mcp`、`file_read`、`file_edit`、`prompt`（可选 `session`、`tool`）。`audit` preset = 前五项；`all` = 该 IDE 支持且 Adapter 已映射的全部事件。

IDE 事件映射（Adapter 负责）：

| 抽象事件 | Cursor | Qoder / Claude Code |
|----------|--------|----------------------|
| `shell` | `beforeShellExecution`, `afterShellExecution` | `PreToolUse`/`PostToolUse` + matcher `Bash` |
| `mcp` | `beforeMCPExecution`, `afterMCPExecution` | `PreToolUse`/`PostToolUse` + matcher `Mcp.*` |
| `file_read` | `beforeReadFile` | `PreToolUse` + matcher `Read` |
| `file_edit` | `afterFileEdit` | `PostToolUse` + matcher `Write\|Edit` |
| `prompt` | `beforeSubmitPrompt` | `UserPromptSubmit` |
| `session` | `sessionStart`, `sessionEnd` | `SessionStart`, `SessionEnd`（Qoder 不支持 sessionStart） |

Qoder 不支持的抽象事件跳过并 warning。

上报记录（`EventRecord`）：

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

## 5. 命令

| 命令 | 作用 |
|------|------|
| `work install <name>` | 扩展支持 hooks 套装（`work install company-hooks`） |
| `work hooks report --ide --event` | 接收 IDE hook stdin，写入队列（由脚本调用，非用户直接执行） |
| `work hooks sync` | 手动将本地队列上报内网 |
| `work hooks status` | 查看队列积压与上次同步状态 |
| `work list --kind hooks` | 列出已安装 hooks 套装 |
| `work uninstall <name>` | 卸载 hooks 套装 |
| `work update [name]` | 更新 hooks 套装 |

`work hooks report` 参数：`--ide`（必填）、`--event`（必填，抽象或 IDE 原始事件名）、`--hooks-kit`（可选，默认从已安装状态推断）、`--stdin-file`（调试，从文件读入代替 stdin）。

行为：读 stdin → 合并元数据 → 脱敏 → 追加 `~/.work/telemetry/queue.jsonl` → 若 `telemetry.enabled` 且距上次 sync 超过阈值则后台触发一次 sync（单进程内防抖，不阻塞 hook）→ 透传 stdin 到 stdout → `exit 0`（默认 3s 超时仍 exit 0，仅写本地警告日志）。

`work hooks sync`：读 `uploaded_at` 为空的记录 → 按 `batch_size` 分批 POST `telemetry.url` → 成功归档 / 失败保留并指数退避。

`work hooks status`（human/`--json`）：待上报条数、最旧未上报事件时间、上次成功同步时间、上次错误信息、`telemetry.enabled`/`url` 配置状态。

## 6. IDE 适配器（Hooks）

路径映射（用户级）：

| IDE | hooks 配置 | 脚本目录 |
|-----|------------|----------|
| Cursor | `~/.cursor/hooks.json` | `~/.cursor/hooks/work-telemetry/` |
| Qoder | `~/.qoder/settings.json` → `hooks` | `~/.qoder/hooks/work-telemetry/` |
| Claude | `~/.claude/settings.json` → `hooks` | `~/.claude/hooks/work-telemetry/` |

项目级将 `~` 换为 `<project-root>` 对应的 `.cursor/`/`.qoder/`/`.claude/`。

配置格式：
- **Cursor**（`hooks.json`，version 1）：`hooks.<event>[]`，每项含 `command`/`timeout`/`managed_by: work`/`work_id`。
- **Qoder / Claude**（`settings.json` 内 `hooks` 键）：`hooks.<Event>[]`，每项含 `matcher` + `hooks[]`（`type: command`/`command`/`timeout`/`managed_by: work`/`work_id`）。

`managed_by`/`work_id` 为 work 扩展字段；若目标 IDE 忽略未知 JSON 字段，卸载依赖安装时写入的 sidecar 状态文件 `~/.work/hooks-installed/{name}.json` 记录条目指纹。

Merge 策略：
1. 读现有配置；不存在则创建骨架。
2. 按 `work_id` 删旧 work 条目再插新（update 幂等）。
3. 不删无 `managed_by: work` 的用户条目。
4. 同一事件多个 matcher：work 条目追加在数组末尾。

`telemetry.sh` 行为（套装脚本）：

```bash
#!/usr/bin/env bash
input=$(cat)
work hooks report --ide "${WORK_HOOKS_IDE}" --event "${WORK_HOOKS_EVENT}" || true
echo "$input"
exit 0
```

安装时 Adapter 为每个映射事件生成调用包装（或单一脚本 + 环境变量），确保 `work` 在 PATH 中；不在则安装阶段警告并写入绝对路径（`which work`）。

## 7. 本地存储

```
~/.work/
├── telemetry/
│   ├── queue.jsonl          # 待上报 / 失败重试
│   ├── archive/             # 已上报归档（按日滚动）
│   └── state.json           # last_sync, pending_count, last_error
└── hooks-installed/
    └── {name}.json          # 安装指纹（配置路径、条目列表）
```

`queue.jsonl` 行格式：

```json
{"event":{...EventRecord...},"uploaded_at":null,"retry_count":0,"last_error":""}
```

## 8. Telemetry API 契约

```
POST {telemetry.url}   # 默认 /v1/events，可在 url 中写全路径
Content-Type: application/json

{ "client": "work-cli", "client_version": "0.2.0", "events": [ { ...EventRecord }, ... ] }
```

| 状态码 | 含义 |
|--------|------|
| `202`/`200` | 接收成功，客户端标记已上传 |
| `4xx` | 客户端错误，记录 `last_error`，超 `max_retries` 后跳过并写 `dead_letter.jsonl` |
| `5xx`/网络错误 | 保留队列，指数退避重试 |

认证扩展（预留）：`TelemetryAuthenticator` 接口，MVP 默认空实现；后续可接 API Key/SSO。

## 9. 核心流程

### 9.1 install（hooks）

1. 解析 ref，识别 `hooks.yaml`
2. 检查 `env`
3. `--dry-run`：打印将写入的 IDE 路径与事件列表
4. 解析 `telemetry.events`（preset + config 覆盖）
5. 对各已检测 IDE：复制脚本 → merge hooks 配置 → 写 sidecar 指纹
6. 写入 `installed.json`（`kind: hooks`，含 `telemetry.events`）
7. 输出中文友好结果

### 9.2 uninstall

读状态与 sidecar 指纹 → 各 IDE Adapter 按 `work_id` 移除条目 → 删脚本目录（若空则保留父目录）→ 删 sidecar 与状态记录。

### 9.3 update

同 bundle：同 scope 先 uninstall 再 install。

## 10. 安全与隐私

1. 默认脱敏：`prompt`、文件全文、`tool_input` 中的大段文本截断或哈希。
2. 不上报环境变量值；仅上报变量名（若 payload 含 env）。
3. `machine_id` 使用稳定但不可逆的哈希，不含主机名明文（可配置）。
4. 本地 `queue.jsonl` 权限 `0600`，目录 `0700`。

## 11. 退出码

| 场景 | 退出码 |
|------|--------|
| `hooks report` 成功 | 0（始终，避免阻断 IDE） |
| `hooks sync` 部分失败 | 1 |
| install 失败 | 1 |
| 指定 IDE 未检测到且 `--ide` 显式指定 | 3 |

## 12. 后续扩展

- **阶段二（执行审计）**：合规规则引擎（危险命令、敏感路径、外发数据）、审计留痕与检索、告警与报表；可能扩展 `work hooks audit`、manifest 中 `telemetry.audit_rules`。审计以旁路分析为主，不要求 hook 阻断 IDE。
- **阶段三（触发执行自动化）**：阻断型 hooks（非零退出码或修改 payload）、审批流、Webhook/内部自动化平台、`failClosed` 策略。复用阶段一的 `telemetry.sh` 与事件映射，新增 `hooks.yaml` 资源类型与 `work hooks evaluate` 等命令。
- 其他：`work doctor --hooks`、项目级默认套装 IT 推送、对接 Vault、更多 IDE。
