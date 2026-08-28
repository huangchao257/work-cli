# 自更新

> `work` 自身的版本检查与静默更新。跨模块共性见 [总览](../overview.md)。

## 1. 目标

执行 `work install`、`work list` 等命令时，按间隔自动检查 GitHub Releases；若有新版本自动下载并静默更新，然后重新执行原命令，对用户透明。

## 2. 架构

```
work <任意命令>
        │
        ▼
cli/autoupdate.go::runAutoUpdate (PersistentPreRunE)
  - ShouldAutoUpdate 判断是否跳过
  - LoadConfig 读 self_update（按 check_interval 节流）
  - github.go 查询 Releases
  - version.go::CompareVersions 比较 semver
  - 有新版本 → updater.go 下载/替换二进制
  - NotifyAutoUpdate 提示
        │
        ▼
reexec.go（+ reexec_unix.go / reexec_windows.go）
  用原 argv 重新执行 → 命令可能透明运行两次
```

**注意**：因为更新后会用原 argv 重执行，一条命令可能透明运行两次（更新前一次、更新后一次）。修改命令流程时需考虑此特性。

## 3. 触发与跳过

`runAutoUpdate` 作为 cobra `PersistentPreRunE` 在每条命令前运行。`ShouldAutoUpdate` 在以下情况跳过：

- `--no-auto-update` 显式跳过本次
- `--dry-run` 或 `--json`（避免污染预览/脚本输出）
- 命令名为 `upgrade` / `version` / `help` / `completion` / `work`，或以 `help` 开头

超时：检查上下文 `context.WithTimeout` 2 分钟；检查失败仅 stderr 警告，不阻断命令。

## 4. 配置

`~/.work/config.yaml`：

```yaml
self_update:
  enabled: true          # 是否自动更新，默认 true
  check_interval: 2h     # 检查间隔，默认 2h
  channel: stable        # stable 或 beta，默认 stable
```

环境变量：`WORK_AUTO_UPDATE=false` 关闭自动更新；`WORK_SELF_UPDATE_CHANNEL=beta` 切换通道（覆盖配置文件）。

- **stable**：仅正式 Release（排除 prerelease tag）。
- **beta**：优先取最新的 prerelease 版本，无 prerelease 时回退 stable。
- 通道值非法时报错（`ValidateChannel`），不静默忽略。

`internal/selfupdate/state.go` 持久化上次检查时间，按 `check_interval` 节流，避免每次命令都查询 GitHub。`Updater` 缓存最近一次拉取的 release 信息，同一命令内 Check + Upgrade 不重复请求 GitHub API。

## 5. 命令

| 命令 | 作用 |
|------|------|
| `work upgrade --check` | 仅检查是否有新版本 |
| `work upgrade` | 手动更新到最新版 |
| `work upgrade --dry-run` | 预览将下载的版本 |
| `work upgrade --version v0.2.0` | 更新到指定版本 |
| `work version` | 显示版本（默认检查更新，有新版本则提示 `work upgrade`） |

## 6. 版本比较

`version.go::CompareVersions` 委托给共享包 `internal/semver.Compare`（按 semver 比较当前版本与 GitHub Releases 最新版本；支持 `v` 前缀，`dev` 视为最低版本），决定是否更新。`internal/semver` 是跨模块的版本比较统一入口。

## 7. 失败处理

- GitHub 不可达 / 下载失败：stderr 警告，不阻断原命令。
- Windows 下若 `work` 二进制被占用（终端占用），替换可能失败；提示关闭占用终端后重试。

## 8. 非目标

- 不做强制更新（用户可通过 `--no-auto-update` 或 `WORK_AUTO_UPDATE=false` 关闭）。
- 不内置认证（公共 GitHub Releases，无认证）。
