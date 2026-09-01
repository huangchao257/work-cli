#!/usr/bin/env bash
# work CLI 一键安装脚本（macOS / Linux）
#
# 用法:
#   curl -fsSL https://github.com/huangchao257/work-cli/releases/latest/download/install.sh | bash
#   WORK_VERSION=v0.1.0 curl -fsSL .../install.sh | bash
#
# 环境变量:
#   WORK_INSTALL_REPO   GitHub 仓库，默认 huangchao257/work-cli
#   WORK_VERSION        版本号 v0.1.0 或 latest（默认）
#   WORK_INSTALL_DIR    安装目录，默认 ~/.local/bin

set -euo pipefail

INSTALL_DIR="${WORK_INSTALL_DIR:-$HOME/.local/bin}"
REPO="${WORK_INSTALL_REPO:-huangchao257/work-cli}"
VERSION="${WORK_VERSION:-latest}"

log() { printf '==> %s\n' "$*"; }
err() { printf '错误: %s\n' "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "未找到命令: $1"
}

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) err "不支持的 CPU 架构: $arch" ;;
  esac
  case "$os" in
    linux|darwin) ;;
    *) err "不支持的操作系统: $os" ;;
  esac
  printf '%s %s' "$os" "$arch"
}

resolve_download_url() {
  local os="$1"
  local arch="$2"
  need_cmd curl

  if [ "$VERSION" = "latest" ]; then
    local api="https://api.github.com/repos/${REPO}/releases/latest"
    local url
    url="$(curl -fsSL "$api" | grep -oE "https://[^\"]+work_[^\"]+_${os}_${arch}\\.tar\\.gz" | head -n 1)"
    [ -n "$url" ] || err "未找到 ${os}/${arch} 的 Release 产物，请先发布版本"
    printf '%s' "$url"
    return
  fi

  local ver="${VERSION#v}"
  printf 'https://github.com/%s/releases/download/%s/work_%s_%s_%s.tar.gz' \
    "$REPO" "$VERSION" "$ver" "$os" "$arch"
}

install_binary() {
  local url="$1"
  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  log "下载 $url"
  need_cmd tar
  curl -fsSL "$url" -o "$tmp/work.tar.gz"

  # 校验和验证：下载同 Release 的 checksums.txt，校验失败拒绝安装。
  # 归档经公网分发，无校验则 MITM/CDN 污染会直接落盘执行。
  local checksum_url="${url%/*}/checksums.txt"
  if curl -fsSL "$checksum_url" -o "$tmp/checksums.txt" 2>/dev/null; then
    local want have
    want="$(grep -F "$(basename "$url")" "$tmp/checksums.txt" | awk '{print $1}')"
    [ -n "$want" ] || err "checksums.txt 中未找到 $(basename "$url")，拒绝安装"
    local hash_cmd
    if command -v sha256sum >/dev/null 2>&1; then
      hash_cmd="sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
      hash_cmd="shasum -a 256"
    else
      err "未找到 sha256sum 或 shasum"
    fi
    have="$($hash_cmd "$tmp/work.tar.gz" | awk '{print $1}')"
    [ "$have" = "$want" ] || err "校验和不匹配（期望 $want，实际 $have），拒绝安装"
    log "校验和验证通过"
  else
    err "无法下载 checksums.txt，拒绝安装"
  fi

  tar -xzf "$tmp/work.tar.gz" -C "$tmp"
  local bin
  bin="$(find "$tmp" -type f -name work | head -n 1)"
  [ -n "$bin" ] || err "压缩包中未找到 work 二进制"

  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$bin" "$INSTALL_DIR/work"
  log "已安装到 $INSTALL_DIR/work"

  local examples_src
  examples_src="$(find "$tmp" -type d -name examples | head -n 1)"
  if [ -n "$examples_src" ] && [ -d "$examples_src/codegraph-stack" ]; then
    local pkg_dir="${HOME}/.work/examples"
    mkdir -p "$(dirname "$pkg_dir")"
    # 原子升级：先完整复制到临时目录，成功后再切换。
    # 直接 rm -rf 后 cp 失败会留下空目录，内置套装全部不可用且无法自愈。
    local staging="${pkg_dir}.staging.$$"
    rm -rf "$staging"
    if cp -a "$examples_src" "$staging"; then
      rm -rf "$pkg_dir"
      mv "$staging" "$pkg_dir"
      log "内置套装已安装到 $pkg_dir"
    else
      rm -rf "$staging"
      log "警告: 复制内置套装失败，保留原有 $pkg_dir"
    fi
  fi
}

check_path() {
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) return 0 ;;
  esac
  log "请将以下行加入 ~/.bashrc 或 ~/.zshrc："
  printf '\n  export PATH="%s:$PATH"\n\n' "$INSTALL_DIR"
}

main() {
  log "work CLI 安装程序"
  read -r os arch <<< "$(detect_platform)"
  url="$(resolve_download_url "$os" "$arch")"
  install_binary "$url"
  if "$INSTALL_DIR/work" version >/dev/null 2>&1; then
    log "安装成功: $($INSTALL_DIR/work version)"
  else
    log "安装完成"
  fi
  check_path
  log "运行 work --help 查看帮助"
}

main "$@"
