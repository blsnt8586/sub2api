#!/usr/bin/env bash
# 一键构建并更新线上 sub2api 二进制
# 用法：bash deploy/update.sh
# 前提：本机已配置 SSH 免密 -> root@186.241.72.237

set -euo pipefail

REMOTE_HOST="root@186.241.72.237"
REMOTE_DIR="/opt/sub2api"
BINARY="sub2api_linux_amd64"

info()    { echo -e "\033[0;34m[INFO]\033[0m  $*"; }
success() { echo -e "\033[0;32m[OK]\033[0m    $*"; }
error()   { echo -e "\033[0;31m[ERROR]\033[0m $*" >&2; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/.."

# ── 1. 构建 ──────────────────────────────────────────────────────────────────
info "开始构建..."
bash build.sh
[[ -f "$BINARY" ]] || error "构建失败：未找到 $BINARY"
success "构建完成：$(ls -lh "$BINARY" | awk '{print $5}')"

# ── 2. 上传 ──────────────────────────────────────────────────────────────────
info "上传到 ${REMOTE_HOST}:${REMOTE_DIR}/sub2api.new ..."
scp "$BINARY" "${REMOTE_HOST}:${REMOTE_DIR}/sub2api.new"
success "上传完成"

# ── 3. 服务器替换 & 重启 ──────────────────────────────────────────────────────
info "替换二进制并重启服务..."
# shellcheck disable=SC2029
ssh "$REMOTE_HOST" "
  set -euo pipefail
  cd ${REMOTE_DIR}
  systemctl stop sub2api
  cp sub2api sub2api.bak.\$(date +%Y%m%d-%H%M%S)
  mv sub2api.new sub2api
  chmod +x sub2api
  systemctl start sub2api
  sleep 2
  systemctl is-active --quiet sub2api && echo '[服务器] 服务已启动 ✓' || { echo '[服务器] 启动失败'; exit 1; }
"

success "线上已更新 🚀"
