# work-cli 设计规格书

> 状态：已评审通过（2026-06-11）  
> 范围：**资源管理模块** — AI 技能 / MCP / Rules 分发与安装，以及**外部 CLI 委托安装**

## 1. 概述

### 1.1 目标

`work` 是一个企业级统一 CLI 入口，面向**公司全体员工**。**资源管理模块**（首个业务模块）帮助员工：

1. **安装资源套装** — Skills、MCP、Rules，兼容 Qoder、Cursor、Claude Code  
2. **安装外部 CLI** — 通过 `work install <name>` 执行受信来源定义的官方安装命令（如 `work install openspec` 等价于执行 OpenSpec 的安装流程）

统一入口、统一 list / uninstall / update，降低员工记忆成本。

### 1.1.1 模块命名

| 模块 | 说明 | 状态 |
|------|------|------|
| **资源管理模块** | 资源套装（Skills/MCP/Rules）+ 外部 CLI 委托安装 | MVP（本文档范围） |
| **Hooks 模块** | IDE 事件采集上报；后续：执行审计 → 触发自动化 | 阶段一已实现（见 [Hooks 规格](./2026-06-11-hooks-module-design.md)） |
| **CodeGraph 模块** | 代码知识图谱索引与 Agent 文档同步 | 已实现（见 [CodeGraph 规格](./2026-06-11-codegraph-agents-design.md)） |

### 1.2 已确认决策

| 维度 | 决策 |
|------|------|
| 当前模块 | 资源管理模块 |
| 定位 | 平台型统一入口 `work` |
| 主要用户 | 公司全体员工（含研发、运维、产品、运营等非技术岗位） |
| 体验原则 | 错误提示通俗可执行、帮助文档完整、安装尽量一键化（兼顾非技术用户） |
| 代码组织 | Monorepo 单仓库 |
| 认证 | MVP 不做；预留认证接口 |
| 实现语言 | Go 1.26+（当前最新稳定版 1.26.4） |
| 架构模式 | Manifest + IDE 适配器 |
| 套装来源 | 官方 → 内部 Registry；团队自定义 → Git |
| 安装范围 | `--scope user\|project`，默认 `user` |
| P0 命令 | `install` / `list` / `uninstall` / `update` |
| 安装类型 | `bundle`（资源套装）与 `cli`（外部 CLI 委托安装） |
| MCP 密钥 | 环境变量占位 `${VAR}`，安装前校验并提示 |
| 跨平台 | macOS (arm64/amd64)、Linux (amd64/arm64)、Windows (amd64) |

### 1.3 非目标（MVP）

- 用户认证与权限控制
- `pack` / `publish` / `doctor` / `init` 命令
- Windows arm64
- WSL 与 Windows 原生环境自动合并
- 对接公司 Vault / 密钥服务（仅预留扩展点）
- 自动检测 IDE 非默认安装路径

---

## 2. 整体架构

```
用户 (work install / list / uninstall / update)
        │
        ▼
┌───────────────────────────────────────┐
│  CLI 层 (Cobra)                        │
└───────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────┐
│  Install Engine（统一编排）             │
│  - 解析 ref → 拉取包                    │
│  - 识别 type: bundle | cli              │
│  - 分发到 Bundle Engine 或 CLI Runner   │
└───────────────────────────────────────┘
        │
   ┌────┴────┐
   ▼         ▼
Bundle     CLI Runner
Engine     (执行 install/update/uninstall 命令)
   │              │
   ▼              ▼
Adapters      os/exec + 跨平台脚本
(Qoder/Cursor/Claude)
        │
   Source + State + Platform
  installed.json（含 kind 字段）
```

### 2.1 核心原则

1. **一份 manifest 描述安装包**，与来源（Registry / Git / 本地）无关；`type` 区分资源套装与外部 CLI。
2. **bundle 类型**：三个 IDE Adapter 负责路径映射、文件写入、MCP 合并。
3. **cli 类型**：CLI Runner 按 manifest 执行受信安装命令（支持按 OS 分平台）。
4. **统一状态文件** 记录两类安装，支撑 list / uninstall / update。
5. **跨平台路径** 统一经 `internal/platform` 解析，禁止硬编码 OS 路径。

---

## 3. 命令设计

### 3.1 P0 命令

| 命令 | 作用 | 示例 |
|------|------|------|
| `work install <ref>` | 安装资源套装或外部 CLI | `work install dev-kit` |
| | | `work install openspec`（执行 OpenSpec 官方安装命令） |
| | | `work install git:team/ai-rules@v1.2` |
| | | `work install ./local-bundle` |
| `work list` | 列出已安装项（套装 + CLI） | `work list` |
| | | `work list --kind cli` |
| | | `work list --ide cursor`（仅 bundle） |
| `work uninstall <name>` | 卸载 | `work uninstall dev-kit` |
| | | `work uninstall openspec` |
| `work update [name]` | 更新 | `work update` |
| | | `work update openspec` |

### 3.2 全局参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--scope` | `user` 或 `project`（仅 `bundle` 类型） | `user` |
| `--ide` | 逗号分隔：`qoder,cursor,claude`（仅 `bundle`） | 全部已检测到的 IDE |
| `--kind` | 过滤类型：`bundle` / `cli`（用于 list） | 全部 |
| `--dry-run` | 预览将执行的操作（写入路径或安装命令） | `false` |
| `--json` | JSON 输出（脚本/CI 用） | `false` |

### 3.3 `<ref>` 解析规则

| 格式 | 来源 | 行为 |
|------|------|------|
| `dev-kit` | Registry | 拉取包 → 根据 manifest `type` 分发 |
| `openspec` | Registry | 拉取 cli 包 → 执行 `install` 命令 |
| `git:host/org/repo@v1.0` | Git | shallow clone 到缓存目录 |
| `./path/to/pkg` | 本地 | 目录须含 `bundle.yaml` 或 `installer.yaml` |

**本地目录识别优先级：** 存在 `installer.yaml` → `cli`；存在 `bundle.yaml` → `bundle`；否则报错。

---

## 4. Manifest 格式

所有可安装包使用统一元数据文件。资源套装用 `bundle.yaml`；外部 CLI 用 `installer.yaml`（字段结构兼容，见 §4.3）。

### 4.1 Bundle Manifest（`type: bundle`，可省略 type）

每个资源套装根目录包含 `bundle.yaml`：

```yaml
type: bundle   # 可省略，默认 bundle
name: dev-kit
version: 1.2.0
description: 公司通用 AI 技能包

env:
  - name: MY_API_KEY
    description: 内部 API 密钥
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
        - API_KEY: ${MY_API_KEY}

targets: [qoder, cursor, claude]   # 可选；省略则三家都装
```

### 4.2 Bundle 字段说明

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 否 | `bundle`（默认） |
| `name` | 是 | 套装唯一标识，用于 uninstall / update |
| `version` | 是 | 语义化版本 |
| `description` | 否 | 人类可读描述 |
| `env` | 否 | 安装前需存在的环境变量列表 |
| `resources.skills` | 否 | Skill 资源列表 |
| `resources.rules` | 否 | Rule 资源列表 |
| `resources.mcp` | 否 | MCP 服务配置列表 |
| `targets` | 否 | 限制目标 IDE |

### 4.3 CLI Installer Manifest（`installer.yaml`）

外部 CLI 安装包根目录包含 `installer.yaml`：

```yaml
type: cli
name: openspec
version: 1.0.0
description: OpenSpec 官方 CLI

env:
  - name: NPM_TOKEN
    description: 若走私有 npm 源时需要
    required: false

# 安装命令（二选一：全局 run 或按平台 platforms）
install:
  run: npm install -g @fission-ai/openspec@latest

verify:
  command: [openspec, --version]

uninstall:
  run: npm uninstall -g @fission-ai/openspec

update:
  run: npm install -g @fission-ai/openspec@latest
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 是 | 固定 `cli` |
| `name` | 是 | CLI 唯一标识，对应 `work install <name>` |
| `version` | 是 | 安装包版本（非被安装 CLI 的运行时版本） |
| `install` | 是 | 安装定义，见下表 |
| `verify` | 否 | 安装后验证命令；失败则警告（不阻断） |
| `uninstall` | 否 | 卸载命令；缺失时 `uninstall` 提示仅支持手动卸载 |
| `update` | 否 | 更新命令；缺失时 `update` 回退为重新执行 `install` |
| `env` | 否 | 执行前检查的环境变量（规则同 bundle） |

**`install` / `uninstall` / `update` 结构：**

| 字段 | 说明 |
|------|------|
| `run` | 单行 shell 命令字符串，`work` 在默认 shell 中执行 |
| `platforms.{darwin,linux,windows}.run` | 按 `GOOS` 选择；优先于全局 `run` |

**执行规则：**

- `work install openspec` → 解析 manifest → 检查 `env` → `--dry-run` 时仅打印将执行的命令 → 执行 `install`
- 命令在**用户环境**中运行（继承 PATH、环境变量）
- 仅允许来自 Registry / Git / 本地路径的 manifest，**不接受** `work install 'npm i -g foo'` 这种任意 shell（防注入）
- 安装成功后写入 `installed.json`，`kind: "cli"`

**`--scope` 对 cli 类型：** 忽略 `project`，始终为用户级全局 CLI；若用户传 `--scope project` 则警告并继续。

### 4.4 资源项字段（bundle）

**Skill**

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | 稳定标识，对应安装目录名 |
| `source` | 是 | 相对 bundle 根目录的路径（含 `SKILL.md` 的目录） |

**Rule**

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | 稳定标识 |
| `source` | 是 | 规则文件路径 |
| `apply` | 是 | `always` / `manual` / `files` |
| `globs` | 条件 | `apply=files` 时必填 |

**MCP**

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | MCP server 标识，用于 merge 与 uninstall |
| `source` | 是 | MCP JSON 配置文件 |
| `env` | 否 | 环境变量映射，值支持 `${VAR}` 占位 |

---

## 5. IDE 适配器

### 5.1 接口

```go
type Adapter interface {
    Name() string
    Detect() bool
    InstallSkill(ctx context.Context, skill SkillResource, scope Scope) error
    InstallRule(ctx context.Context, rule RuleResource, scope Scope) error
    InstallMCP(ctx context.Context, mcp MCPResource, scope Scope) error
    Uninstall(ctx context.Context, state BundleInstallState) error
}
```

### 5.2 路径映射（用户级示例）

路径经 `platform.IDEPaths(ide, scope)` 解析，以下为 macOS/Linux 参考；Windows 使用 `%USERPROFILE%` 对应目录。

| 资源 | Qoder | Cursor | Claude Code |
|------|-------|--------|-------------|
| Skill (user) | `~/.qoder/skills/{id}/` | `~/.cursor/skills/{id}/` | `~/.claude/skills/{id}/` |
| Skill (project) | `.qoder/skills/{id}/` | `.cursor/skills/{id}/` | `.claude/skills/{id}/` |
| Rule (user) | `~/.qoder/rules/` | `~/.cursor/rules/` | `~/.claude/` 或 rules 约定路径 |
| Rule (project) | `.qoder/rules/` | `.cursor/rules/` | `CLAUDE.md` / `.claude/` |
| MCP (user) | Qoder MCP 配置 | `~/.cursor/mcp.json` | Claude MCP 配置 |
| MCP (project) | 项目 MCP 配置 | `.cursor/mcp.json` | 项目 MCP 配置 |

### 5.3 合并策略

| 资源类型 | 策略 |
|----------|------|
| Skills | 按 `id` 复制整个目录；uninstall 删除该目录 |
| Rules | 按 `id` 写入独立文件；uninstall 删除对应文件 |
| MCP | **merge**：按 server `id` 合并进现有 JSON，不整文件覆盖；uninstall 移除对应 server 条目 |

### 5.4 Rule `apply` 映射

| manifest `apply` | Qoder | Cursor | Claude Code |
|------------------|-------|--------|-------------|
| `always` | Always Apply | alwaysApply: true | 写入始终生效规则区 |
| `manual` | Apply Manually | 手动规则 | 手动引用规则 |
| `files` | Specific Files + globs | globs 映射 | globs 映射 |

具体 front matter / 元数据格式在实现阶段按各 IDE 官方文档对齐，Adapter 内封装转换逻辑。

### 5.5 IDE 未检测到

若 `Detect()` 返回 false：

- 默认跳过该 IDE，继续安装其他 IDE
- 输出 warning（`--json` 时写入 `warnings` 数组）
- `--ide` 显式指定但未检测到时返回错误

---

## 6. 核心流程

### 6.1 install

1. 解析 `<ref>`，从 Registry / Git / 本地获取包目录
2. 识别 manifest 类型（`installer.yaml` → cli；`bundle.yaml` → bundle）
3. 检查 `env` 中 `required: true` 的变量；缺失则报错并输出平台相关设置示例
4. 若 `--dry-run`：
   - **bundle**：打印计划写入路径
   - **cli**：打印将执行的 `install.run` 命令
5. **bundle 分支**：解析 `--scope` / `--ide` → Adapter 安装 → 写入状态
6. **cli 分支**：选择当前 `GOOS` 对应 `install` 命令 → `os/exec` 执行 → 可选 `verify` → 写入状态（`kind: cli`）
7. 输出中文友好结果

### 6.2 list

读取状态文件，输出：名称、**kind**（bundle/cli）、版本、scope（cli 显示 `user`）、来源 ref、安装时间；bundle 额外显示目标 IDE 列表。

支持 `--kind bundle|cli` 过滤。

### 6.3 uninstall

1. 从状态文件查找 `name`（bundle 还需匹配 `scope`）
2. **bundle**：各 IDE `Adapter.Uninstall` 回滚
3. **cli**：执行 manifest `uninstall.run`；若未定义则提示手动卸载并仅删除状态记录
4. 删除状态记录

### 6.4 update

| 来源 | 更新检测 |
|------|----------|
| Registry | `GET /bundles/{name}/latest`，比较 version |
| Git | 比较 tag/branch 或 commit |
| 本地 | 比较 manifest `version` 或目录 checksum |

| kind | 更新策略 |
|------|----------|
| bundle | 同 scope 先 uninstall 再 install |
| cli | 有 `update.run` 则执行；否则重新执行 `install` |

---

## 7. 状态与配置

### 7.1 状态文件

| scope | 路径 |
|-------|------|
| user | `~/.work/installed.json` |
| project | `<project-root>/.work/installed.json` |

状态记录示例：

```json
{
  "bundles": [
    {
      "name": "dev-kit",
      "kind": "bundle",
      "version": "1.2.0",
      "scope": "user",
      "ref": "registry:dev-kit",
      "installed_at": "2026-06-11T10:00:00Z",
      "ides": ["qoder", "cursor", "claude"],
      "resources": {
        "skills": ["code-review"],
        "rules": ["go-style"],
        "mcp": ["internal-mysql"]
      }
    },
    {
      "name": "openspec",
      "kind": "cli",
      "version": "1.0.0",
      "scope": "user",
      "ref": "registry:openspec",
      "installed_at": "2026-06-11T11:00:00Z",
      "install_command": "npm install -g @fission-ai/openspec@latest"
    }
  ]
}
```

### 7.2 用户配置

`~/.work/config.yaml`：

```yaml
registry:
  url: https://registry.internal.example.com

cache:
  dir: ~/.work/cache
```

---

## 8. 套装来源

### 8.1 Registry（官方套装）

MVP API（内网、无认证）：

```
GET /bundles/{name}/latest
GET /bundles/{name}/{version}
```

响应：

```json
{
  "name": "dev-kit",
  "type": "bundle",
  "version": "1.2.0",
  "download_url": "https://.../dev-kit-1.2.0.zip",
  "checksum": "sha256:abc..."
}
```

`type` 为 `bundle` 或 `cli`；包内分别含 `bundle.yaml` 或 `installer.yaml`。  
示例：`openspec` 的 `type: cli`，下载解压后执行其中 `install.run`。

### 8.2 Git（团队套装）

- 格式：`git:host/org/repo@ref`（ref 为 tag、branch 或 commit）
- shallow clone 到 `~/.work/cache/git/`
- bundle 须位于仓库根目录或 manifest 中声明的路径（MVP 假定根目录）

### 8.3 本地

- 目录须包含 `bundle.yaml`
- 用于开发调试与内网共享盘场景

---

## 9. 跨平台支持

### 9.1 目标平台

| GOOS | GOARCH | 优先级 |
|------|--------|--------|
| darwin | arm64, amd64 | P0 |
| linux | amd64, arm64 | P0 |
| windows | amd64 | P0 |
| windows | arm64 | P1（不做） |

### 9.2 platform 包职责

```
internal/platform/
├── paths.go       # UserHomeDir, WorkConfigDir, ProjectRoot
├── ide_paths.go   # 各 IDE × scope 路径表
└── env_hint.go    # 按 GOOS 生成环境变量设置提示
```

规则：

- 使用 `os.UserHomeDir()` 与 `filepath.Join`
- 禁止硬编码 `/home` 或 `C:\Users`
- JSON 文件统一 UTF-8、LF 换行
- 目录创建使用 `os.MkdirAll(dir, 0755)`

### 9.3 构建与分发

CI 矩阵构建（goreleaser 或 gox）：

| 产物 | 文件名示例 |
|------|-----------|
| macOS arm64 | `work-darwin-arm64` |
| macOS amd64 | `work-darwin-amd64` |
| Linux amd64 | `work-linux-amd64` |
| Linux arm64 | `work-linux-arm64` |
| Windows amd64 | `work-windows-amd64.exe` |

安装方式（MVP）：

- macOS / Linux：安装脚本或手动下载二进制加入 PATH
- Windows：PowerShell 安装脚本或手动配置 PATH

### 9.4 WSL 说明

MVP 不自动处理 WSL 与 Windows 双环境。文档要求用户在**实际运行 IDE 的环境**中执行 `work` 命令。

---

## 10. Monorepo 目录结构

```
work-cli/
├── cmd/work/                 # main.go
├── internal/
│   ├── cli/                  # Cobra 命令
│   ├── bundle/               # bundle.yaml 解析与校验
│   ├── installer/            # installer.yaml 解析与 CLI 命令执行
│   ├── engine/               # 统一 install/uninstall/update 编排
│   │   ├── bundle.go         # bundle 安装分支
│   │   └── cli.go            # cli 委托安装分支
│   ├── source/
│   │   ├── registry.go
│   │   ├── git.go
│   │   └── local.go
│   ├── state/                # installed.json
│   ├── platform/             # 跨平台路径与环境提示
│   └── adapter/
│       ├── adapter.go
│       ├── qoder/
│       ├── cursor/
│       └── claude/
├── examples/
│   ├── dev-kit/              # 示例 bundle
│   └── openspec/             # 示例 cli installer（mock 命令）
├── docs/
│   └── superpowers/specs/
├── .github/workflows/
│   └── release.yml
├── go.mod
└── README.md
```

### 10.1 技术依赖

| 依赖 | 用途 |
|------|------|
| Go 1.26+ | 语言运行时；`go.mod` 声明 `go 1.26`，CI 使用最新稳定补丁版 |
| `github.com/spf13/cobra` | CLI 框架 |
| `gopkg.in/yaml.v3` | manifest 解析 |
| 标准库 | 文件操作、HTTP、Git（或 `os/exec` 调用 git） |

---

## 11. 错误处理与输出

### 11.1 错误码约定

| 退出码 | 场景 |
|--------|------|
| 0 | 成功 |
| 1 | 一般错误（校验失败、IO 错误等） |
| 2 | 用法错误（参数不合法） |
| 3 | 环境不满足（缺 env、指定 IDE 未安装） |

### 11.2 人类可读输出

面向公司全员，默认输出避免术语堆砌，采用「问题 + 下一步」结构。

- 成功（bundle）：`✓ 已安装 dev-kit v1.2.0 → qoder, cursor（范围：用户级）`
- 成功（cli）：`✓ 已安装 openspec v1.0.0（已执行：npm install -g @fission-ai/openspec@latest）`
- 警告：`⚠ 未检测到 Cursor，已跳过；请先安装 Cursor 或访问 <文档链接>`
- 错误：说明原因 + 可复制执行的下一步（如环境变量设置示例、联系 IT 的提示）

### 11.3 JSON 输出（`--json`）

```json
{
  "success": true,
  "bundle": "dev-kit",
  "version": "1.2.0",
  "scope": "user",
  "installed_ides": ["qoder", "cursor"],
  "skipped_ides": ["claude"],
  "warnings": ["Claude Code not detected, skipped"],
  "files_written": ["~/.cursor/skills/code-review/SKILL.md"]
}
```

---

## 12. 测试策略

| 层级 | 内容 |
|------|------|
| 单元测试 | manifest 解析、路径解析（mock HOME/USERPROFILE）、MCP merge |
| 集成测试 | `examples/dev-kit` bundle 流程；`examples/openspec-mock` cli 流程（CI）；`openspec` 真实命令用于 `--dry-run` 校验 |
| CI 矩阵 | ubuntu-latest、macos-latest、windows-latest |
| 手工清单 | 三系统各安装 example bundle，验证三 IDE 目录 |

---

## 13. 后续扩展（P1+）

| 功能 | 说明 |
|------|------|
| `work pack` | 将目录打包为可分发 zip + manifest |
| `work publish` | 上传至 Registry |
| `work doctor` | 检查 IDE、路径、MCP 连通性 |
| `work init` | 生成空 bundle 模板 |
| 认证 | SSO / API Key，对接 Registry 与 Git |
| Vault 集成 | 自动拉取 MCP 密钥 |
| 更多 IDE | VS Code、Windsurf 等 |
| Windows arm64 | 补充构建目标 |

---

## 14. 认证扩展预留

MVP 不实现，但在 `internal/source` 与 `internal/cli` 预留：

```go
type Authenticator interface {
    HTTPHeaders() map[string]string  // Registry 请求
    GitCredentials() (*url.Userinfo, error)
}
```

默认实现返回空（无认证）。
