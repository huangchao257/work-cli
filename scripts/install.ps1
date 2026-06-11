# work CLI 一键安装脚本（Windows PowerShell）
#
# 用法:
#   irm https://github.com/huangchao257/work-cli/releases/latest/download/install.ps1 | iex
#
# 环境变量:
#   WORK_INSTALL_REPO   默认 huangchao257/work-cli
#   WORK_VERSION        v0.1.0 或 latest
#   WORK_INSTALL_DIR    默认 %USERPROFILE%\.local\bin

$ErrorActionPreference = "Stop"

$InstallDir = if ($env:WORK_INSTALL_DIR) { $env:WORK_INSTALL_DIR } else { Join-Path $env:USERPROFILE ".local\bin" }
$Repo = if ($env:WORK_INSTALL_REPO) { $env:WORK_INSTALL_REPO } else { "huangchao257/work-cli" }
$Version = if ($env:WORK_VERSION) { $env:WORK_VERSION } else { "latest" }

function Write-Step($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }

$Arch = "amd64"

function Get-DownloadUrl {
    if ($Version -eq "latest") {
        $api = "https://api.github.com/repos/$Repo/releases/latest"
        $release = Invoke-RestMethod -Uri $api -UseBasicParsing
        $asset = $release.assets | Where-Object { $_.name -match "work_.*_windows_${Arch}\.zip" } | Select-Object -First 1
        if (-not $asset) { throw "未找到 windows/$Arch 的 Release 产物" }
        return $asset.browser_download_url
    }
    $ver = $Version.TrimStart("v")
    return "https://github.com/$Repo/releases/download/$Version/work_${ver}_windows_${Arch}.zip"
}

Write-Step "work CLI 安装程序"
$Url = Get-DownloadUrl
Write-Step "下载 $Url"

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$TempDir = Join-Path $env:TEMP ("work-install-" + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null

$ZipPath = Join-Path $TempDir "work.zip"
Invoke-WebRequest -Uri $Url -OutFile $ZipPath -UseBasicParsing
Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

$Bin = Get-ChildItem -Path $TempDir -Recurse -Filter "work.exe" | Select-Object -First 1
if (-not $Bin) { throw "压缩包中未找到 work.exe" }

$Dest = Join-Path $InstallDir "work.exe"
Copy-Item -Path $Bin.FullName -Destination $Dest -Force
Remove-Item -Recurse -Force $TempDir

Write-Step "已安装到 $Dest"
& $Dest version

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Step "将 $InstallDir 加入用户 PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$InstallDir;$UserPath", "User")
    $env:Path = "$InstallDir;$env:Path"
}

Write-Step "安装完成。请重新打开终端后运行: work --help"
