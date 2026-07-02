# 资源管理模块

> bundle（Skills/MCP/Rules 套装）与 cli（外部 CLI 委托安装）。跨模块共性见 [总览](../overview.md)。

## 1. 范围与状态

帮助员工：
1. **安装资源套装** — Skills、MCP、Rules，兼容 Qoder、Cursor、Claude Code。
2. **安装外部 CLI** — 通过 `work install <name>` 执行受信来源定义的官方安装命令（如 `work install openspec`）。

统一入口、统一 list/uninstall/update，降低员工记忆成本。状态：已实现。

## 2. Manifest 格式

### 2.1 bundle.yaml

```yaml
name: dev-kit
version: 1.0.0
description: 公司通用 AI 技能包
env:                         # 可选，安装前校验
  - name: MY_API_KEY
    required: true
resources:
  skills:
    - id: code-review
      source: ./skills/code-review
  rules:
    - id: go-style
      source: ./rules/go-style.md
      apply: always          # always | manual | files
      globs: ["**/*.go"]     # apply=files 时必填
  mcp:
    - id: internal-mysql
      source: ./mcp/mysql.json
      env:
        - API_KEY: ${MY_API_KEY}   # 支持 ${VAR} 占位
targets: [cursor]            # 可选；省略则三家都装
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | 是 | 套装唯一标识，用于 uninstall/update |
| `version` | 是 | 语义化版本 |
| `description` | 否 | 人类可读描述 |
| `env` | 否 | 安装前需存在的环境变量列表 |
| `resources.skills` | 否 | Skill 资源列表（`id` + `source` 目录，含 `SKILL.md`） |
| `resources.rules` | 否 | Rule 资源列表（`id` + `source` + `apply` + 可选 `globs`） |
| `resources.mcp` | 否 | MCP 服务配置列表（`id` + `source` + 可选 `env` 映射） |
| `targets` | 否 | 限制目标 IDE |

`internal/bundle/validate.go::CheckRequiredEnv` 在安装前校验 `env.required`，缺失则报错并由 `platform.EnvSetHint` 给出按 OS 的设置示例。

### 2.2 installer.yaml

```yaml
type: cli
name: openspec
version: 1.0.0
install:
  run: npm install -g @fission-ai/openspec@latest
  # 或按平台: platforms.darwin.run / platforms.linux.run / platforms.windows.run
verify:
  command: [openspec, --version]
uninstall:
  run: npm uninstall -g @fission-ai/openspec
update:
  run: npm install -g @fission-ai/openspec@latest   # 缺失则回退为重新 install
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 是 | 固定 `cli` |
| `name` | 是 | CLI 唯一标识，对应 `work install <name>` |
| `version` | 是 | 安装包版本（非被安装 CLI 的运行时版本） |
| `install` | 是 | `run`（单行 shell）或 `platforms.{darwin,linux,windows}.run`（按 GOOS，优先于全局 `run`） |
| `verify` | 否 | 安装后验证命令；失败仅警告不阻断 |
| `uninstall` | 否 | 卸载命令；缺失则提示手动卸载，仅删状态记录 |
| `update` | 否 | 更新命令；缺失则回退为重新执行 `install` |
| `env` | 否 | 执行前检查的环境变量（规则同 bundle） |

执行规则：
- `internal/installer/runner.go` 按 `GOOS` 选命令，在**用户环境**（继承 PATH/env）中经 `os/exec` 执行。
- 仅接受来自 catalog/Registry 的 manifest，**不接受** `work install 'npm i -g foo'` 这类任意 shell（防注入）。
- 安装成功后写入 `installed.json`，`kind: "cli"`。
- `--scope project` 对 cli 忽略（始终用户级全局 CLI）；若传则警告并继续。

## 3. 来源解析与内置 Catalog

`internal/source/resolver.go::Resolve` 按 `Ref` 类型解析。`ref.go::ParseInstallName` / `ValidateInstallName` **仅接受名称**，拒绝本地路径与 git 引用（`git.go`/`local.go` 解析器存在但公开安装路径不使用）。

`internal/catalog/builtin.go` 硬编码名称 → `examples/<dir>`，内置项：`dev-kit`、`codegraph-stack`、`codegraph-kit`、`codegraph`、`company-hooks`、`openspec`、`openspec-mock`。

`examplesRoot()` 解析顺序：`WORK_EXAMPLES_DIR` 环境变量 → `~/.work/examples` → 相对二进制的 `examples`/`../examples`/`../share/work/examples` → 从 cwd 向上找（开发/测试）。`examples/` 既是随包发布的内置资源，也是测试夹具。

内部 Registry（需在 `~/.work/config.yaml` 配 `registry.url`）：

```
GET /bundles/{name}/latest      → { name, type, version, download_url, checksum }
```

`type` 为 `bundle`/`cli`/`hooks`，包内分别含对应 manifest。

## 4. IDE 适配器（`internal/adapter/`）

```go
type Adapter interface {
    Name() string
    Detect() bool
    InstallSkill(ctx, bundleRoot, skill, scope) (string, error)
    InstallRule(ctx, bundleRoot, rule, scope) (string, error)
    InstallMCP(ctx, bundleRoot, mcp, scope) (string, error)
    Uninstall(ctx, rec state.BundleRecord, scope) error
}
```

实现：`cursor.go`、`qoder.go`、`claude.go`，由 `adapter.go::All`/`ByName` 注册。`common.go` 共享路径/front-matter 工具，`mcp_merge.go` 合并 MCP server 配置。

路径经 `platform.IDEPaths(ide, scope)` 解析（macOS/Linux 参考；Windows 用 `%USERPROFILE%`）：

| 资源 (user) | Cursor | Qoder | Claude Code |
|------|--------|-------|-------------|
| Skill | `~/.cursor/skills/{id}/` | `~/.qoder/skills/{id}/` | `~/.claude/skills/{id}/` |
| Rule | `~/.cursor/rules/` | `~/.qoder/rules/` | `~/.claude/` |
| MCP | `~/.cursor/mcp.json` | Qoder MCP 配置 | Claude MCP 配置 |

项目级将 `~` 换为 `<project-root>` 对应的 `.cursor/`/`.qoder/`/`.claude/`。

| 资源类型 | 合并策略 |
|----------|----------|
| Skills | 按 `id` 复制整个目录；uninstall 删除该目录 |
| Rules | 按 `id` 写独立文件；uninstall 删除对应文件 |
| MCP | **merge**：按 server `id` 合并进现有 JSON，不整文件覆盖；uninstall 移除对应条目 |

Rule `apply` 映射到各 IDE：`always` → 始终生效；`manual` → 手动引用；`files` → globs 映射。具体 front-matter/元数据在 Adapter 内封装转换。

IDE 未检测到：默认跳过并 warning（`--json` 写入 `warnings`）；`--ide` 显式指定但未检测到时返回错误（退出码 3）。

## 5. 核心流程（`internal/engine/`）

### 5.1 install（`install.go::Install`）

1. `source.Resolve(opts.Ref)` 得包目录
2. `pkg/manifest.DetectKind` 探测类型
3. 分发：
   - `KindBundle` → `bundle.go::installBundle`：解析 `--scope`/`--ide` → 各 Adapter 写 Skills/Rules/MCP → 写状态
   - `KindCLI` → `cli_install.go`：选当前 GOOS 命令 → 执行 → 可选 `verify` → 写状态（`kind: cli`）
4. `--dry-run`：bundle 打印将写入路径，cli 打印将执行命令
5. 输出中文友好结果（human 或 `--json`）

### 5.2 list / uninstall / update

- **list**（`list.go`）：读 `installed.json`，输出 name/kind/version/scope/ref/installed_at；bundle 额外显示目标 IDE 与资源；`--kind bundle|cli` 过滤，`--ide` 过滤（仅 bundle）。
- **uninstall**（`uninstall.go`）：按 name（bundle 还需 scope）查记录 → bundle 各 Adapter 回滚 → cli 执行 `uninstall.run`（缺失则提示手动卸载，仅删状态）→ 删状态记录。
- **update**（`update.go`）：读 `installed.json` 记录的 name 重新解析安装；bundle 同 scope 先 uninstall 再 install；cli 有 `update.run` 则执行，否则重新 `install`。

## 6. 示例包

| 名称 | 类型 | 说明 |
|------|------|------|
| `dev-kit` | bundle | 公司通用 AI 技能包（示例 skills/rules） |
| `openspec` | cli | OpenSpec 官方 CLI 安装（`npm install -g @fission-ai/openspec@latest`） |
| `openspec-mock` | cli | mock 命令，用于 CI 集成测试 |
