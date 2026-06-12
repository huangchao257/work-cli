# Hooks 模块 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 Hooks 模块——独立 `hooks.yaml` 套装安装（Cursor/Qoder/Claude Code），以及 `work hooks report/sync/status` 本地队列 + 异步内网上报。

**Architecture:** 扩展 `DetectKind` 识别 `hooks.yaml`；`engine.installHooks` 复制脚本并 merge IDE hooks 配置（sidecar 记录指纹）；`work hooks report` 读 stdin 写 `queue.jsonl` 并透传；`sync` 批量 POST 内网 API。

**Tech Stack:** Go 1.26+、Cobra、yaml.v3、encoding/json

**Spec:** `docs/superpowers/specs/2026-06-11-hooks-module-design.md`

---

## 文件结构

| 路径 | 职责 |
|------|------|
| `internal/pkg/manifest/detect.go` | 增加 `KindHooks` |
| `internal/hooks/manifest.go` | hooks.yaml 类型 |
| `internal/hooks/parse.go` | 解析与校验 |
| `internal/hooks/events.go` | 抽象事件 preset 与 IDE 映射 |
| `internal/hooks/config.go` | telemetry 配置读取 |
| `internal/hooks/redact.go` | payload 脱敏 |
| `internal/hooks/queue.go` | queue.jsonl 读写 |
| `internal/hooks/report.go` | report 逻辑 |
| `internal/hooks/sync.go` | 上报与重试 |
| `internal/hooks/sidecar.go` | hooks-installed 指纹 |
| `internal/hooks/merge.go` | Cursor / settings.json merge |
| `internal/hooks/install.go` | 安装与卸载 |
| `internal/engine/install.go` | 分发 hooks 分支 |
| `internal/engine/uninstall.go` | hooks 卸载分支 |
| `internal/state/types.go` | resources.hooks、telemetry |
| `internal/cli/hooks.go` | hooks 子命令 |
| `examples/company-hooks/` | 示例套装 |
| `internal/hooks/*_test.go` | 单元测试 |

---

### Task 1: Manifest 与事件映射

- [ ] 扩展 `detect.go` 识别 `hooks.yaml`
- [ ] 实现 `internal/hooks/manifest.go`、`parse.go`、`events.go`
- [ ] 测试：`events_test.go` audit preset 映射三家 IDE

### Task 2: 上报管道

- [ ] 实现 `queue.go`、`redact.go`、`config.go`、`report.go`、`sync.go`
- [ ] 测试：queue 追加、redact 截断 prompt

### Task 3: 安装与 merge

- [ ] 实现 `merge.go`、`sidecar.go`、`install.go`
- [ ] 集成 `engine/install.go`、`engine/uninstall.go`
- [ ] 测试：Cursor hooks.json merge 快照

### Task 4: CLI 与示例

- [ ] `internal/cli/hooks.go`：report、sync、status
- [ ] `examples/company-hooks/`
- [ ] 更新 README 与 spec 状态为已实现

### Task 5: 验证

- [ ] `go test ./...`
- [ ] `work install ./examples/company-hooks --dry-run`
