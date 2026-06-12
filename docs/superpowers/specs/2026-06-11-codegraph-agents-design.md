# CodeGraph 知识图谱 + AGENTS.md 套装设计

> 状态：已评审（2026-06-11）  
> 范围：封装 [CodeGraph](https://github.com/colbymchenry/codegraph) CLI，基于索引为项目各目录生成 `AGENTS.md`

## 1. 目标

员工通过 `work install` 一键获得：

1. **CodeGraph CLI** — 本地 SQLite 代码知识图谱（符号、调用关系、文件结构）
2. **IDE MCP 配置** — Cursor / Claude Code / Qoder 自动连接 `codegraph serve --mcp`
3. **AGENTS.md 生成技能** — 脚本扫描图谱，向「有意义」的目录写入 AI 操作指引

以 **bundle + cli 套装** 交付；对外暴露 **`work graph`** 简易命令（对标 `codegraph init -i`）。

## 2. 已确认决策

| 维度 | 决策 |
|------|------|
| 图谱实现 | 封装上游 `codegraph` CLI（npm 全局安装） |
| 集成方式 | `examples/codegraph/installer.yaml` + `examples/codegraph-kit/bundle.yaml` |
| AGENTS.md 范围 | 仅 CodeGraph 已索引的**代码目录**（`nodeCount > 0` 且语言为 go/ts/js/py 等，排除 yaml/md/docs/examples） |
| 复杂度过滤 | 简单代码链路不生成：单文件且符号少于 12 个（`AGENTS_MIN_SYMBOLS`）；`cmd/*` 入口始终保留 |
| AGENTS.md 内容 | AI 操作指引（改什么功能去哪个目录/文件） |
| 写入方式 | 直接写入各目录，支持 `--dry-run` 预览 |
| 自动更新 | `setup-auto-sync.sh` 写 Cursor hook，防抖 2s 后 `sync` + 重生 AGENTS.md |
| 依赖 | `codegraph`、`jq`（脚本解析 JSON） |

## 3. 架构

```
work install codegraph        → 安装 codegraph CLI
work install codegraph-kit    → 安装 Skill + MCP

项目根目录执行 generate-agents.sh
        │
        ▼
codegraph init -i / sync                 → 确保 .codegraph/ 索引就绪
        │
        ▼
codegraph files --json                   → 按目录聚合文件与符号数
codegraph query --kind function|struct   → 提取关键符号
        │
        ▼
各目录 AGENTS.md                          → 任务指引表 + 关键符号 + 相关目录
```

## 4. 目录筛选规则

生成前从 `codegraph files --json` 读取索引，**只为代码目录**写入或更新 `AGENTS.md`：

1. **代码目录** — 目录内至少有一个文件满足：`nodeCount > 0` 且 `language` 属于代码语言（go、typescript、javascript、python、rust 等）；排除 docs、examples、测试目录与构建产物。
2. **复杂度达标** — 满足以下任一条件：
   - 目录内代码符号总数 ≥ `AGENTS_MIN_SYMBOLS`（默认 12）
   - 目录内代码文件数 ≥ `AGENTS_MIN_CODE_FILES`（默认 2）
   - 路径为 `cmd` / `cmd/*`（程序入口始终生成）
3. **清理** — 同步时删除不再符合条件的旧 `AGENTS.md`（仅带自动生成标记的文件）。

示例：`internal/pkg/copyutil`（单文件、8 符号）不生成；`internal/catalog`（2 文件、17 符号）生成。

## 5. AGENTS.md 模板

每个有意义目录生成：

- **目录用途** — 基于路径启发式（如 `internal/cli` → 命令层）
- **AI 操作指引** — 任务 → 目标文件/目录 对照表
- **关键符号** — 该目录下导出函数/类型（来自图谱）
- **相关目录** — 父目录、子包、常见协作目录

页眉标注自动生成与更新命令。

## 6. 自动同步机制

```
保存源码 (Cursor afterFileEdit)
        │
        ▼
on-file-edit.sh（透传 stdin，后台调度）
        │
        ▼
sync-agents-schedule.sh（防抖 2s，对齐 CodeGraph）
        │
        ▼
codegraph sync  →  generate-agents.sh --skip-sync
        │
        ▼
各目录 AGENTS.md 更新
```

备选：`watch-agents.sh` 用 inotify/fswatch 监听文件系统，不依赖 IDE hook。

## 7. 非目标

- 自研多语言 AST 解析器
- 将 AGENTS.md 默认加入 `.gitignore`（团队自行决定是否提交）

## 8. 使用流程

```bash
# 一键（推荐，对标 codegraph install + init -i）
work install codegraph-stack

# 或分步
work install codegraph
work install codegraph-kit --scope project   # 自动 post_install → graph init

# 简易命令
work graph init    # 初始化 + 无感自动同步
work graph sync    # 手动同步
work graph status  # 查看状态
```
