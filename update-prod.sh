#!/usr/bin/env bash
# =============================================================================
# Sub2API 生产环境一键更新脚本
# 用法：./update-prod.sh
# 前提：本机已配置 SSH 免密登录 root@186.241.72.237
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="sub2api_linux_amd64"
REMOTE_HOST="root@186.241.72.237"
REMOTE_DIR="/opt/sub2api"
SERVICE_NAME="sub2api"

# 颜色输出
info()    { echo -e "\033[0;34m[INFO]\033[0m  $*"; }
success() { echo -e "\033[0;32m[OK]\033[0m    $*"; }
warn()    { echo -e "\033[1;33m[WARN]\033[0m  $*"; }
error()   { echo -e "\033[0;31m[ERROR]\033[0m $*" >&2; }
step()    { echo -e "\n\033[1;37m==> $*\033[0m"; }

cd "$SCRIPT_DIR"

# ── Step 1: 本地构建 ──────────────────────────────────────────────────────────
step "1/3  本地构建"
bash build.sh

if [ ! -f "$BINARY" ]; then
    error "构建失败：未找到 $BINARY"
    exit 1
fi
success "构建完成：$(ls -lh "$BINARY" | awk '{print $5, $9}')"

# ── Step 2: 上传到服务器 ───────────────────────────────────────────────────────
step "2/3  上传二进制"
info "目标：${REMOTE_HOST}:${REMOTE_DIR}/${SERVICE_NAME}.new"
scp "$BINARY" "${REMOTE_HOST}:${REMOTE_DIR}/${SERVICE_NAME}.new"
success "上传完成"

# ── Step 3: 服务器上原子替换 + 重启 ──────────────────────────────────────────
step "3/3  服务器替换 & 重启"
ssh "$REMOTE_HOST" bash -s << EOF
set -euo pipefail
cd ${REMOTE_DIR}

BACKUP="${SERVICE_NAME}.bak.\$(date +%Y%m%d-%H%M%S)"
echo "[服务器] 停止 ${SERVICE_NAME}..."
systemctl stop ${SERVICE_NAME}

echo "[服务器] 备份旧版本 -> \$BACKUP"
cp ${SERVICE_NAME} "\$BACKUP"

echo "[服务器] 替换二进制..."
mv ${SERVICE_NAME}.new ${SERVICE_NAME}
chmod +x ${SERVICE_NAME}

echo "[服务器] 启动 ${SERVICE_NAME}..."
systemctl start ${SERVICE_NAME}

echo "[服务器] 等待服务就绪..."
sleep 3
if systemctl is-active --quiet ${SERVICE_NAME}; then
    echo "[服务器] 服务运行中 ✓"
    systemctl status ${SERVICE_NAME} --no-pager -n 5
else
    echo "[服务器] 服务启动失败，回滚..."
    systemctl stop ${SERVICE_NAME} 2>/dev/null || true
    mv "\$BACKUP" ${SERVICE_NAME}
    systemctl start ${SERVICE_NAME}
    echo "[服务器] 已回滚到旧版本"
    exit 1
fi
EOF

success "线上更新完成 🚀"
echo ""
info "查看实时日志：ssh ${REMOTE_HOST} 'journalctl -u ${SERVICE_NAME} -f'"
