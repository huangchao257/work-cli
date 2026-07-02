# work-cli 设计文档

`work` 是公司内部统一 CLI 入口（Go 1.26+，module `github.com/huangchao257/work-cli`），面向全体员工。文档按模块组织：

## 目录

- **[总览](./overview.md)** — 整体架构、分层、命令、全局参数、状态与配置、跨平台与构建、错误与输出、测试、目录结构、后续扩展（跨模块共性）
- **[资源管理模块](./modules/resource.md)** — 安装 AI IDE 资源套装（Skills/MCP/Rules）与委托安装外部 CLI（bundle / cli）
- **[Hooks 模块](./modules/hooks.md)** — IDE hooks 套装安装与事件采集上报（观察型，阶段一）
- **[CodeGraph 模块](./modules/codegraph.md)** — 代码知识图谱索引与各目录 `AGENTS.md` 自动维护
- **[自更新](./modules/selfupdate.md)** — 从 GitHub Releases 检查并静默更新 `work` 自身，更新后重执行原命令
- **[扩展能力](./extensions.md)** — `work doctor` / `work init` / `work config` / `work pack` 等独立命令

## 模块状态

| 模块 | 能力 | 状态 |
|------|------|------|
| 资源管理 | 资源套装 + 外部 CLI 委托安装 | 已实现 |
| Hooks | IDE 事件采集上报（本地 + 异步内网） | 阶段一（观察型）已实现 |
| CodeGraph | 知识图谱 + AGENTS.md 自动同步 | 已实现 |
| 自更新 | GitHub Releases 静默更新 + 重执行 | 已实现 |

## 说明

- 历史设计规格与实现计划（按日期归档的变更记录）见 `docs/superpowers/specs/` 与 `docs/superpowers/plans/`。
- 本目录文档按当前实现校准；代码以 `internal/` 为准，各包附 CodeGraph 自动生成的 `AGENTS.md` 供导航。
- 用户可见文案与代码注释均为中文。
