# work-cli

公司统一 CLI 入口。当前提供**资源管理模块**：

- 安装 AI IDE 资源套装（Skills / MCP / Rules），支持 Qoder、Cursor、Claude Code
- 委托安装外部 CLI（如 OpenSpec）

## 安装 work（员工）

### 一键安装

**macOS / Linux：**

```bash
curl -fsSL https://github.com/huangchao257/work-cli/releases/latest/download/install.sh | bash
```

**Windows（PowerShell）：**

```powershell
irm https://github.com/huangchao257/work-cli/releases/latest/download/install.ps1 | iex
```

安装后执行 `work version` 验证。详细说明见 [员工安装指南](docs/install-guide.md)。

### 手动下载

从 [Releases](https://github.com/huangchao257/work-cli/releases) 下载对应平台压缩包，解压后将 `work` 加入 PATH。

### 本地构建（开发）

```bash
make build
# 或
go build -o bin/work ./cmd/work
```

### 发布新版本（IT）

```bash
git tag v0.1.0 && git push origin v0.1.0   # 自动触发 GitHub Release
# 本地打包: make package  → 产物在 dist/
```

## 快速开始

```bash
# 安装示例资源套装（写入 ~/.cursor）
work install ./examples/dev-kit

# 预览 OpenSpec 安装命令（不实际执行）
work install ./examples/openspec --dry-run

# 实际安装 OpenSpec（需要 npm）
work install ./examples/openspec

# 或通过 Registry 名称（需配置 ~/.work/config.yaml）
work install openspec
```

## 常用命令

| 命令 | 说明 |
|------|------|
| `work install <ref>` | 安装 bundle 或外部 CLI |
| `work list` | 列出已安装项 |
| `work list --kind cli` | 仅列出 CLI |
| `work uninstall <name>` | 卸载 |
| `work update [name]` | 更新 |

### 全局参数

| 参数 | 默认 | 说明 |
|------|------|------|
| `--scope` | `user` | `user` 或 `project`（仅 bundle） |
| `--ide` | 全部已检测 | `qoder,cursor,claude` |
| `--dry-run` | false | 预览操作 |
| `--json` | false | JSON 输出 |

### 安装引用 `<ref>`

| 格式 | 示例 |
|------|------|
| Registry 名称 | `dev-kit`、`openspec` |
| Git | `git:github.com/org/repo@v1.0` |
| 本地目录 | `./examples/dev-kit` |

## Registry 配置

`~/.work/config.yaml`：

```yaml
registry:
  url: https://registry.internal.example.com

cache:
  dir: ~/.work/cache
```

## OpenSpec

```bash
work install openspec
# 执行: npm install -g @fission-ai/openspec@latest
```

## 故障排查

| 问题 | 处理 |
|------|------|
| 未检测到 IDE | 先安装 Cursor/Qoder/Claude Code，或用 `--ide cursor` 指定 |
| 缺少环境变量 | 按提示执行 `export VAR=值` |
| Registry 失败 | 检查 `~/.work/config.yaml` 中 `registry.url` |
| npm 安装失败 | 确认已安装 Node.js 和 npm |

## 文档

- 设计规格：`docs/superpowers/specs/2026-06-11-work-cli-design.md`
- 实现计划：`docs/superpowers/plans/2026-06-11-resource-module.md`
