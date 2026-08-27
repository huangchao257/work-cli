# work api — 系统接口 CLI 化模块设计

> 状态：已实现（首期）。参考钉钉 `dws`（渐进 schema、风险元数据、快捷命令）与飞书 `lark-cli`（三层命令、元数据驱动、输出契约）的公开设计。

## 1. 目标与非目标

**目标**

- 把内部/第三方系统的 OpenAPI 3.x 规范变成统一 CLI 命令，供人和 Agent 使用。
- 三层命令面：`+shortcut`（意图级）→ 动态类型化命令（元数据生成）→ `call METHOD PATH`（通用兜底）。
- 扩展模式：OpenAPI 数据驱动（导入即可用）+ 编译期 Go 插件（鉴权/传输/组合快捷方式）。
- 统一风险门禁（fail-closed）、凭据环境变量化（永不明文落盘/上屏）、稳定 JSON 信封与退出码。

**非目标（首期明确排除）**

- OpenAPI 2.0/Swagger 支持、跨文件/跨 URL 外部 `$ref` 解析、完整 JSON Schema 校验。
- OAuth/SSO/token 刷新、keychain 凭据存储（`Authenticator` 是未来接缝）。
- 进程外插件协议（`.so`/RPC/子进程）。
- 自动分页、重试、multipart 上传、jq 过滤、MCP 发现、Observer/Restrict/Wrap/On 扩展点。

## 2. 架构

```text
work api <cmd>
   │
   ▼
internal/cli/api*.go ──── Cobra 自注册（init() → rootCmd.AddCommand）
   │                       ├── api.go            管理命令（list/info/import/refresh/remove）
   │                       ├── api_schema.go     渐进 schema 查询
   │                       ├── api_call.go       L3 通用调用 + 确认 TTY + 信封渲染
   │                       ├── api_dynamic.go    L1/L2 动态命令装配
   │                       └── api_builtin_*.go  内置系统 blank-import 装配点
   ▼
internal/api/             框架：System 接口 + Registry + 调用编排 + 风险门禁
   ├── system.go          System/Authenticator/TransportProvider/Shortcuts 接口
   ├── registry.go        （在 system.go 内）编译期注册表，冻结语义
   ├── config.go          system.yaml 读写 + configSystem（导入系统包装）
   ├── import.go          Import/Refresh/Remove（原子写 + HTTPS 强制）
   ├── call.go            Call：解析 operation → 构造请求 → 鉴权 → 门禁 → 发送
   ├── risk.go            read/write/dangerous 三级 + ConfirmationRequiredError
   ├── shortcut.go        快捷方式合并与执行（handler 型组合编排）
   └── demo/              内嵌示例系统（mock 传输，离线全链路）
   ▼
internal/openapi/         纯解析：Load(JSON/YAML) / $ref / Index → Catalog
```

关键原则：

1. **单一执行路径**：导入系统与编译期插件最终都实现 `System` 接口，`Call` 不区分来源。
2. **目录与规范分离**：启动期只读 `catalog.json`（紧凑、稳定），完整规范在 schema/调用时惰性加载。损坏的单个系统跳过其动态命令，不阻塞 CLI 启动。
3. **门禁在框架内**：风险判定是 `internal/api/risk.go` 的单一函数，不是插件策略。

## 3. 三层命令面与路由

```text
L1  work api <system> +<shortcut> [flags]        意图级，预设参数可被显式 flag 覆盖
L2  work api <system> <cli-path...> [flags]      类型化命令（kebab-case flags、enum 补全）
L3  work api call <system> <op|METHOD PATH>      兜底；--params/--set/--data/--header

管理/自省：
work api list / info / import / refresh / remove / schema [--compact|--all]
```

- **cli-path 生成**：operation 显式 `x-work-cli-path`（字符串或数组）优先；否则 `首个 tag + kebab(operationId)`；无 operationId 无扩展时不可动态化。
- **冲突降级**：同 cli-path 的多个 operation 全部失去动态命令（不覆盖、不猜），仅通过 schema/call 可用，并记录 warning。
- **flag 冲突降级**：同名参数出现在不同位置（query/header）时两者都禁用类型化 flag，降级到 `--set location.name=value`；保留字（json/dry-run/yes/params/set/data/header）冲突同样降级。
- **参数优先级**：shortcut 预设 < `--params`/`--set` < 显式类型化 flag。

## 4. OpenAPI 子集与扩展

- 支持 3.0/3.1，JSON/YAML 双格式（首字节 sniff）；拒绝 2.0/Swagger 与空 paths。
- 内部 `#/components/...` `$ref` 惰性解析：深度上限 32、visited 集合防环；命中上限/循环返回截断占位而非失败。
- 外部 `$ref`（`file.yaml#/…`、`http://…`）不解析：参数仍可调用（name/in/required 在 operation 层内联），受影响 schema 摘要显示原始 ref 串并附 warning。
- 校验边界：只做 required 参数存在性与请求体 JSON 语法校验，不做 JSON Schema 验证。
- 规范大小上限 8 MiB；响应体读取上限 4 MiB。

**`x-work-*` 扩展**：

| 扩展 | 位置 | 说明 |
|---|---|---|
| `x-work-cli-path` | operation | 显式命令路径（字符串空格分隔或数组） |
| `x-work-risk` | operation | `read` / `write` / `dangerous`；缺省按 method 推断（GET/HEAD/OPTIONS→read，POST/PUT/PATCH/DELETE→write，其他→dangerous） |

## 5. 系统：内置插件与导入

**编译期插件**（`System` + 可选接口）：

```go
type System interface {
    Manifest() Manifest
    Catalog(ctx) (*openapi.Catalog, error)   // 惰性
    Document(ctx) (*openapi.Document, error) // schema 用
    BaseURL() string
}
// 可选：Authenticator / TransportProvider / Shortcuts
```

- 在包 `init()` 中 `api.DefaultRegistry.Register(New())`，由 `internal/cli/api_builtin_<system>.go` blank-import 触发装配。
- `Authenticator`：自定义鉴权（签名、token 刷新），默认实现 none/bearer/apikey。
- `TransportProvider`：自定义 `http.RoundTripper`（demo mock、未来审计/重试）。
- `Shortcuts`：handler 型快捷方式可编排多个 operation（demo `+seed` = createPet + listPets）。

**导入系统**（纯数据）：

```text
~/.work/api/systems/<name>/
├── system.yaml    # base_url / auth（环境变量名）/ shortcuts / source_url；0600
├── openapi.yaml   # 规范快照；0644
└── catalog.json   # 启动期命令目录；0644
```

- `work api import <name> <file|https-url> [--auth ... --credential-env ... --base-url ...]`：校验 → 原子写入（临时目录 + rename）。
- `work api refresh [system]`：仅刷新记录了 HTTPS `source_url` 的系统。
- `work api remove <system> [--yes]`：只删导入系统；内置系统不可删。

## 6. 配置与凭据

- `system.yaml` 的 auth 段只保存环境变量名（`credential_env`），不保存 token；命令行不接受明文凭据。
- `${ENV}` 缺失时报环境错误（退出码 3），提示用 `platform.EnvSetHint` 输出设置命令。
- 所有输出（list/info/dry-run/错误详情）对凭据脱敏：`[已设置]/[未设置]` 或 `***`。
- 全局默认超时 `api.timeout`（`~/.work/config.yaml`，缺省 30s）。

## 7. 调用管线与风险门禁

```text
解析引用(operationId | METHOD PATH | cli-path | shortcut)
→ 参数合并 → path 替换/url-escape → query/header/body 构造
→ 鉴权(Authenticator) → 风险门禁 → transport 发送 → 信封
```

- `--dry-run` 在确认与网络之前短路：输出脱敏后的完整 invocation，绝不发送。
- 风险门禁：read 直接执行；write/dangerous 需要 `--yes` 或 TTY 交互确认；非交互未确认 → 结构化 `confirmation_required` 错误（fail-closed）。确认协议为「原命令追加 `--yes` 重试」，不修改业务参数。
- shortcut 有效风险不低于其目标 operation（声明 read 但目标是 write → 按 write）；handler 型未声明风险按 dangerous。
- 自定义 `--header` 禁止覆盖 Host/Content-Length/Authorization/Cookie/Content-Type 等传输级字段。
- 调用期网络错误/超时/缺凭据 → 退出码 3（结构化 `environment` 错误）；参数与规范问题 → 2；HTTP 非 2xx 与确认未满足 → 1（信封照常输出响应体）。import/refresh 的规范下载失败目前为一般错误 1。

## 8. 输出契约

**JSON（`--json`）**，成功上 stdout：

```json
{
  "ok": true, "system": "demo", "operation": "listPets",
  "method": "GET", "path": "/pets", "status": 200,
  "duration_ms": 3, "data": {...}, "warnings": []
}
```

两类失败信封，注意区分：

- **调用前失败**（参数/规范/凭据/网络问题）→ stderr：`{"ok":false,"error":{"type":"cli|environment|api","subtype":"missing_credential","message":"...","hint":"...","retryable":false}}`。
- **HTTP 非 2xx**（调用已发出、收到响应）→ stdout 完整 result 信封（`ok:false` + `error` 字段，含响应体），退出码 1。

判断成功用 `ok == true` 或退出码。

**human**：`✓ GET /pets（200，3ms）` + 数据摘要；HTTP 错误时状态行用 `✗`，详细错误行在 stderr；调用前失败 `✗ 消息（下一步提示）` 上 stderr。

api 子树全部命令设置 `SilenceErrors/SilenceUsage`，RunE 失败路径、Args 校验与 flag 解析错误（FlagErrorFunc + Args 包装兜底）均经统一 renderer 输出一次；错误信封按 `usage→2 / environment→3 / 其余→1` 映射退出码。

**已知限制**：未声明参数的裸名键与前缀键并存时前缀键优先（裸名跳过）；`--params` 值只接受标量（数组/对象请用 `--set` 或类型化 flag 单值传递，多元素数组参数首期不支持）；L2 类型化 flag 均为 string 类型（int/bool 直转是二期项）。

L2 动态叶子命令带 `cobra.NoArgs`：多余位置参数（多为拼错的子命令）报 usage 错误并退出 2，不静默忽略。

## 9. 安全基线

- 远程导入与刷新仅 HTTPS；下载与响应体设大小上限；HTTP 客户端拒绝跨 host 重定向与协议降级（防凭据外带与 307/308 请求体重发）。
- 原子写（临时文件 + rename，Windows 回退）；system.yaml 0600、目录 0700。
- 凭据永不上屏、永不进命令行参数。dry-run 脱敏按鉴权层注入点遮蔽（自定义 Authenticator 未上报注入点时保守全遮蔽 header/query 值）。
- `--data @file` 只接受当前目录相对路径（拒绝绝对路径与 `..` 上跳），值需为合法 JSON。
- 动态命令名与保留字（list/info/import/refresh/remove/schema/call/help）冲突时拒绝或降级。

## 10. demo 示例系统

`internal/api/demo/`：内嵌 OpenAPI（pets CRUD + status），mock transport 离线返回确定性响应（含 404/500 路径），提供 `+seed`（组合 createPet+listPets，演示 handler 型 shortcut）与 `+top`（listPets 预设参数，演示配置型 shortcut）。`deletePet` 标注 `x-work-risk: dangerous`。

验证命令：

```bash
work api list
work api schema demo --compact
work api demo +top --json
work api demo pets list-pets --limit 2 --json
work api call demo GET /pets --params '{"limit":2}' --json
work api call demo createPet --data '{"name":"rex"}' --dry-run --json
work api call demo createPet --data '{"name":"rex"}' --yes --json
```

## 11. 二期路线（均为只加文件的扩展）

- **动态命令树增强**：更多 flag 类型（int/bool 直转）、`--all` schema 投影定制。
- **OAuth/设备流**：实现 `Authenticator`（`--no-wait` + `--device-code` 两段式），凭据进 keychain。
- **观测/审批**：`TransportProvider` 包装层加审计与限流，不改框架。
- **multipart 上传 / 分页自动聚合 / jq 过滤**。
- **更多内置系统**：每个系统一个 `internal/api/<sys>/` + `internal/cli/api_builtin_<sys>.go`。
