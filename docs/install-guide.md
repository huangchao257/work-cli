# work CLI 员工安装指南

面向公司全体员工的一键安装说明。

## 方式一：一键脚本（推荐）

### macOS / Linux

```bash
curl -fsSL https://github.com/huangchao257/work-cli/releases/latest/download/install.sh -o /tmp/install-work.sh
bash /tmp/install-work.sh
```

安装后把 `~/.local/bin` 加入 PATH（脚本会提示），然后验证：

```bash
work version
work --help
```

### Windows（PowerShell）

```powershell
irm https://github.com/huangchao257/work-cli/releases/latest/download/install.ps1 -OutFile install-work.ps1
.\install-work.ps1
```

重新打开终端后执行 `work version`。

---

## 方式二：手动下载二进制

1. 打开 [Releases 页面](https://github.com/huangchao257/work-cli/releases)
2. 按系统下载对应文件：

| 系统 | 文件 |
|------|------|
| macOS Apple 芯片 | `work_*_darwin_arm64.tar.gz` |
| macOS Intel | `work_*_darwin_amd64.tar.gz` |
| Linux x64 | `work_*_linux_amd64.tar.gz` |
| Linux ARM | `work_*_linux_arm64.tar.gz` |
| Windows | `work_*_windows_amd64.zip` |

3. 解压后将 `work`（Windows 为 `work.exe`）放到 PATH 目录

4. 校验（可选）：对比 `checksums.txt` 中的 SHA256

---

## 方式三：内网镜像（IT 部署后）

若公司使用内网制品库，IT 可设置环境变量后分发统一脚本：

```bash
export WORK_INSTALL_BASE="https://artifacts.internal.example.com/work-cli/releases"
export WORK_VERSION="v0.1.0"
curl -fsSL https://artifacts.internal.example.com/work-cli/install.sh | bash
```

---

## 发布新版本（IT / 平台组）

### 1. 打标签触发 GitHub Release

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions 会自动用 GoReleaser 构建并发布多平台二进制。

### 2. 本地打包（不上传）

```bash
make package
ls dist/
```

产物在 `dist/` 目录，含 `checksums.txt`。

### 3. 将安装脚本附到 Release

发布时把 `scripts/install.sh` 和 `scripts/install.ps1` 作为 Release 附件上传，或托管在公司 CDN。

---

## 安装后第一步

```bash
# 配置公司 Registry（IT 提供地址）
mkdir -p ~/.work
cat > ~/.work/config.yaml <<'EOF'
registry:
  url: https://registry.internal.example.com
EOF

# 安装 OpenSpec
work install openspec

# 安装公司 AI 技能包
work install dev-kit
```

---

## 常见问题

| 问题 | 解决办法 |
|------|----------|
| `work: command not found` | 确认 `~/.local/bin` 在 PATH 中，重开终端 |
| 下载失败 | 检查网络/VPN；改用手动下载 |
| macOS 提示无法验证开发者 | 系统设置 → 隐私与安全性 → 仍要打开；或 IT 签名二进制 |
| Windows 脚本被拦截 | 以管理员运行：`Set-ExecutionPolicy RemoteSigned -Scope CurrentUser` |
