---
name: codegraph-agents
description: 项目代码知识图谱与 AGENTS.md 自动维护。安装 codegraph-stack 或 codegraph-kit 后，使用 work graph init 一键完成；保存代码后无感自动更新。
---

# CodeGraph + AGENTS.md

## 一条命令（推荐）

```bash
work install codegraph-stack
```

自动完成：安装 CodeGraph → 配置 IDE MCP → 建立索引 → 开启 AGENTS.md 无感同步。

## 常用命令（对标 codegraph）

| 命令 | 作用 |
|------|------|
| `work graph init` | 等同 `codegraph init -i` + 自动同步配置 + 生成 AGENTS.md |
| `work graph sync` | 手动同步索引与 AGENTS.md |
| `work graph status` | 查看图谱与自动同步状态 |

## 生成范围

仅对 **CodeGraph 已索引的代码目录** 生成或更新 `AGENTS.md`：

- 依据 `codegraph files --json`：`nodeCount > 0` 且为代码语言（go/ts/js/py 等）
- 排除 `docs/`、`examples/`、测试目录与 yaml/md 配置
- **简单代码链路不生成**：单文件且符号少于 12 个的叶子目录（如小型 util）；`cmd/*` 入口始终保留
- 阈值可通过 `AGENTS_MIN_SYMBOLS`、`AGENTS_MIN_CODE_FILES` 环境变量调整

## 无感自动更新

`work graph init` 会写入 `.cursor/hooks.json`。之后在 Cursor 中**保存源码**，约 2 秒内自动：

1. `codegraph sync` 更新知识图谱
2. 按上述规则重新生成各目录 `AGENTS.md`（并清理不再符合条件的旧文件）

与 CodeGraph MCP 的自动索引节奏一致，无需手动执行脚本。

## 故障排查

| 问题 | 处理 |
|------|------|
| `未找到 codegraph` | `work install codegraph-stack` |
| `未找到 codegraph-agents 技能` | `work install codegraph-kit --scope project` |
| MCP 未生效 | 重启 IDE |
| 查看同步日志 | `.codegraph/agents-sync/sync.log` |
