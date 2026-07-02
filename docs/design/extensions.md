# 扩展能力设计

> 在现有四大模块（资源管理 / Hooks / CodeGraph / 自更新）基础上新增的独立命令。跨模块共性见 [总览](./overview.md)。
> 设计原则：每项能力只用 Go 标准库、只新增文件、命令自注册（`init()` 调 `rootCmd.AddCommand`），互不冲突，可并行实现。

## 1. `work doctor` — 诊断

### 1.1 目标

一键体检本机 `work` 运行环境，定位 README「故障排查」表中的常见问题（IDE 未检测、缺环境变量、config 损坏、状态文件不可读、MCP 配置无效、codegraph 缺失）。

### 1.2 检查项

| 检查 | 实现 | 失败提示 |
|------|------|----------|
| IDE 探测 | `adapter.All()` 各 `Detect()` | 列出已检测/未检测 IDE；`--ide` 显式指定但未检测则提示安装 |
| `work` 在 PATH | `exec.LookPath("work")` 或 `os.Executable()` | 提示加入 PATH |
| `~/.work/config.yaml` 合法 | yaml 解析；不要求存在 | 损坏则提示修复或删除 |
| `installed.json` 可读 | `state.Open(scope)` | 损坏则提示 |
| MCP 配置合法 | 各已检测 IDE 的 MCP 文件 JSON 解析 | 非法则提示文件路径 |
| codegraph 可用 | `exec.LookPath("codegraph")` + `jq` | 缺失则提示安装 codegraph-stack |
| 自更新配置 | 读 `self_update` / `WORK_AUTO_UPDATE` | 显示当前状态 |

### 1.3 命令

```
work doctor [--scope user|project] [--ide ...] [--json]
```

### 1.4 输出

- human：逐项 `✓` / `✗` 清单，末尾汇总「N 项通过 / M 项失败」。
- `--json`：`{ "checks": [{name, ok, detail}], "summary": {...} }`。
- 退出码：全部通过 `0`，任一失败 `1`（便于 CI）。

### 1.5 实现

- 新包 `internal/doctor/`：`Check` 函数返回 `[]CheckResult`。
- 新命令 `internal/cli/doctor.go`：自注册，复用全局 `scope`/`ide`/`asJSON`，用 `output.PrintJSON` 输出 JSON。
- 不修改 `root.go`/`help.go`/`go.mod`。

---

## 2. `work init` — 脚手架

### 2.1 目标

为套装作者生成符合 manifest 规范的骨架目录，降低手写 `bundle.yaml`/`installer.yaml`/`hooks.yaml` 出错率。

### 2.2 命令

```
work init <type> <name> [--dir path] [--dry-run]
# type: bundle | cli | hooks
```

### 2.3 生成内容

| type | 生成 |
|------|------|
| `bundle` | `bundle.yaml` + `skills/<name>/SKILL.md` + `rules/sample.md` + `mcp/sample.json`（注释说明可选） |
| `cli` | `installer.yaml`（含 install/verify/uninstall/update 桩）+ `README.md` |
| `hooks` | `hooks.yaml`（audit preset）+ `scripts/telemetry.sh`（可执行权限） |

manifest 中 `name`/`version`（`0.1.0`）填入，`description` 留占位；字段带中文注释。

### 2.4 输出

- human：列出将创建/已创建文件路径。
- `--dry-run`：仅预览。
- `--json`：`{ "files": [...] }`。
- 退出码：`0` 成功，`2` 用法错误（type 非法、目录已存在且非空），`1` IO 错误。

### 2.5 实现

- 新包 `internal/scaffold/`：按 type 生成文件树，`Run` 返回写入路径列表。
- 新命令 `internal/cli/init.go`：自注册。注意命令名 `init` 不与 Go init 冲突。

---

## 3. `work config` — 配置管理

### 3.1 目标

提供 `~/.work/config.yaml` 的读写入口，免去手编 YAML；为 IT 推送脚本提供可脚本化（`--json`）的配置查询。

### 3.2 命令

```
work config path                          # 打印配置文件路径
work config list [--json]                 # 列出全部键值
work config get <key>                     # 取值，如 registry.url
work config set <key> <value>             # 设值
work config unset <key>                   # 删除键
```

键为点分路径：`registry.url`、`cache.dir`、`self_update.enabled`、`self_update.check_interval`、`telemetry.enabled`、`telemetry.url`、`telemetry.events` 等。

### 3.3 实现

- 新包 `internal/config/`：基于 `yaml.v3` 的 `yaml.Node` 导航/设值/删除，**保留注释与顺序**；`Load`/`Save`/`Get`/`Set`/`Unset`/`List`。文件不存在时 `get` 返回空、`set` 创建骨架。
- **不重构**既有 `selfupdate`/`source` 各自的 config 读取（避免改动其文件）；本包作为新的权威读写入口，后续可渐进合并。
- 新命令 `internal/cli/config.go`：自注册。
- `set` 对布尔/数值尝试类型转换；`events` 等列表以逗号分隔或 JSON 数组字面量。
- 退出码：`0` 成功，`2` 用法错误（键非法），`1` IO 错误。

### 3.4 输出

- `get`：直接打印值（便于脚本）。
- `list`：`key: value` 逐行；`--json` 为对象。
- `path`：打印绝对路径。

---

## 4. `work pack` — 打包

### 4.1 目标

将本地套装目录打包为可分发归档（对标 Registry `download_url` 指向的产物），便于上传内部 Registry 或共享。

### 4.2 命令

```
work pack <dir> [--format zip|tar.gz] [-o output] [--dry-run]
```

### 4.3 行为

1. 校验 `<dir>` 含 `bundle.yaml`/`installer.yaml`/`hooks.yaml` 之一，否则报错（退出 2）。
2. 读取 manifest 的 `name`/`version`，默认输出文件名 `<name>-<version>.<ext>`，写到 `<dir>/../` 或 `--output`。
3. 打包目录内容（含 manifest 与资源），格式 `tar.gz`（默认）或 `zip`。
4. 生成 `<output>.sha256` 校验和文件。
5. `--dry-run`：打印将生成的归档路径与文件清单，不写盘。

### 4.4 实现

- 新包 `internal/pack/`：`archive/zip`、`archive/tar`、`compress/gzip`、`crypto/sha256`、`filepath.Walk`。
- 新命令 `internal/cli/pack.go`：自注册。
- 退出码：`0` 成功，`2` 用法错误（目录无 manifest、格式非法），`1` IO 错误。

### 4.5 输出

- human：`✓ 已打包 <name> v<version> → <path>` + 校验和路径。
- `--json`：`{ "archive": "...", "checksum": "...", "files": N }`。

---

## 5. `work publish` — 上传 Registry

### 5.1 目标

将 `work pack` 产出的归档上传至内部 Registry，补齐「打包 → 发布」链路（对标后续扩展中的 `publish`）。

### 5.2 命令

```
work publish <archive> [--checksum path] [--dry-run] [--json]
# archive: work pack 产出的 .tar.gz / .zip
```

### 5.3 行为

1. 校验 `<archive>` 存在；`--checksum` 默认 `<archive>.sha256`，校验和文件须存在且与归档一致。
2. 读 `source.LoadUserConfig()` 取 `registry.url`；未配置 → 退出 2 并提示。
3. 从归档文件名或同目录 manifest 推断 `name`/`version`/`type`（`pkgmanifest.DetectKind` 对解压临时目录探测；或直接从归档内 manifest 读）。
4. `--dry-run`：打印将上传的 URL、归档、checksum、推断的 name/version/type，不发送。
5. `multipart/form-data` POST 至 `{registry.url}/bundles`，字段：`name`、`version`、`type`、`archive`(file)、`checksum`(file)。
6. 处理响应：`201`/`200` 成功；`4xx` 退出 1 并打印服务端错误；网络错误退出 1。

### 5.4 输出

- human：`✓ 已发布 <name> v<version> → <url>`。
- `--json`：`{ "url": "...", "name": "...", "version": "...", "type": "..." }`。
- 退出码：0 成功 / 2 用法错误（无 registry.url、归档不存在）/ 1 上传失败。

### 5.5 实现

- 新包 `internal/publish/`：`net/http` + `mime/multipart`；`Run(opts) (Result, error)`。
- 新命令 `internal/cli/publish.go`：自注册，复用 `asJSON`/`dryRun`。
- 复用 `source.LoadUserConfig()`（只读），不改 `source` 包。

---

## 6. `work search` — 可用资源发现

### 6.1 目标

列出**可安装**的资源（与 `work list` 已安装项互补）：内置 catalog + 可选 Registry 远程清单，帮助员工发现可用套装。

### 6.2 命令

```
work search [query] [--remote] [--json]
# query: 模糊匹配名称/描述（子串，不区分大小写）
# --remote: 同时查询 Registry 远程清单
```

### 6.3 行为

1. **本地**：遍历 `catalog.Names()`，对每个内置包读 manifest（`bundle.yaml`/`installer.yaml`/`hooks.yaml`）取 name/type/version/description；`query` 子串过滤。
2. **远程**（`--remote`）：读 `registry.url`，`GET {url}/bundles` 返回 `[{name,type,version,description}]`；未配置 registry.url 则仅本地并 warning；失败则 warning 不阻断。
3. 合并去重（本地优先标注 `source: builtin`，远程标注 `source: registry`）。

### 6.4 输出

- human：`- <name> v<version> [type] (source) — <description>`。
- `--json`：`{ "items": [{name,version,type,description,source}] }`。
- 退出码：0（即使远程失败也 0，仅 warning）。

### 6.5 实现

- 新包 `internal/search/`：`catalog` + manifest 解析 + 可选 HTTP；`Run(opts) (Result, error)`。
- 新命令 `internal/cli/search.go`：自注册。
- 复用 `catalog.Resolve`/`Names`、`source.LoadUserConfig()`（只读）。

### 6.6 Registry 契约（新增）

```
GET {registry.url}/bundles   → [{ "name","type","version","description" }]
```

---

## 7. Hooks 阶段二：本地审计引擎

### 7.1 目标

推进 Hooks 模块阶段二（执行审计）的**本地旁路分析**基础：对本地 `queue.jsonl` 中的事件按策略规则匹配，输出违规清单。MVP 不阻断 IDE，纯离线分析。

### 7.2 策略文件

`audit-policy.yaml`（项目根或 `~/.work/audit-policy.yaml`，`--policy` 可覆盖）：

```yaml
rules:
  - id: deny-rm-rf
    event: shell            # 抽象事件名；省略则匹配所有
    match: 'rm\s+-rf'       # 正则，对 payload 文本匹配
    severity: high
  - id: sensitive-write
    event: file_edit
    path_regex: '(/etc/|\.env|credentials)'
    severity: medium
  - id: deny-curl-exfil
    event: shell
    match: 'curl.*|\bnc\b'
    severity: high
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | 规则标识 |
| `event` | 否 | 限定抽象事件；省略匹配全部 |
| `match` | 否 | 正则，对事件 payload 序列化文本匹配 |
| `path_regex` | 否 | 正则，对 payload 中路径字段匹配（file_edit 等） |
| `severity` | 否 | `low`/`medium`/`high`（默认 medium） |

`match` 与 `path_regex` 至少一个；同时存在则任一命中即违规。

### 7.3 命令

```
work hooks audit [--policy path] [--file queue.jsonl] [--since duration] [--json]
```

- `--policy`：策略文件，默认按「项目根 → `~/.work/audit-policy.yaml`」顺序查找。
- `--file`：事件源，默认 `~/.work/telemetry/queue.jsonl`。
- `--since`：仅审计近 N 内的事件（如 `24h`）。

### 7.4 行为

1. 加载策略；无策略文件 → warning 并退出 0（无规则）。
2. 逐行读 `queue.jsonl` 的 `event`（`EventRecord`），按 `--since` 过滤。
3. 对每事件遍历规则：`event` 过滤 → `match`/`path_regex` 正则匹配。
4. 收集 `Violation { RuleID, EventID, Severity, Detail, Timestamp }`。

### 7.5 输出

- human：逐条违规 `✗ [high] <rule-id> @ <event-id> — <detail>`，末尾汇总「N 条违规（high x, medium y）」。
- `--json`：`{ "violations": [...], "summary": {...} }`。
- 退出码：0 无违规 / 1 有违规 / 2 用法错误（策略文件损坏）。

### 7.6 实现

- 新包 `internal/audit/`：`Policy`/`Rule`/`Violation` 类型、`LoadPolicy(path)`、`Evaluate(events []EventRecord, p Policy) []Violation`。正则用 `regexp`。
- 复用 `hooks.EventRecord` 类型（只读引用 `internal/hooks`，不改它）；若 `EventRecord` 未导出字段不便，可在 audit 包定义兼容结构体按需解码。
- 新命令 `internal/cli/hooks_audit.go`：**同级新文件**，`func init() { hooksCmd.AddCommand(auditCmd) }`，不改 `hooks.go`。
- 附 `internal/audit/audit_test.go`：覆盖 event 过滤、match/path_regex 命中、severity 默认、无策略、`--since` 过滤。

### 7.7 与阶段规划关系

本节是设计文档 `modules/hooks.md` 第 12 节「阶段二」的落地起点：本地策略预览，不要求 hook 阻断 IDE。后续可扩展 `telemetry.audit_rules` 引用策略、服务端集中审计、阶段三阻断型 hook。

---

## 8. 实现约束（共同）

- 仅用 Go 标准库 + 既有依赖（`cobra`/`yaml.v3`）；**不得**新增依赖、不得改 `go.mod`/`go.sum`。
- **只新增文件**：新包目录 + 新 `internal/cli/<cmd>.go`；**不得**修改 `root.go`、`help.go`、`go.mod`、既有包文件。
- 命令自注册：`func init() { rootCmd.AddCommand(xxxCmd) }`。
- 复用全局 flag：`scope`/`ide`/`kind`/`dryRun`/`asJSON`；中文 `Short`/`Long`/`Example` 直接写在 `cobra.Command` 上。
- 输出：human 用 `fmt.Fprintf`；JSON 用 `output.PrintJSON(w, v)`；非 1 退出码用 `exitErr(code, err)`。
- 每包附 `_test.go`，覆盖核心逻辑；自验证 `go build ./...` 与 `go test ./<pkg>/...` 通过。
