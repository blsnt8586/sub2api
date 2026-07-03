#!/bin/bash
set -e

echo "=========================================="
echo "Sub2API 构建脚本"
echo "=========================================="

# 检查必需工具
if ! command -v pnpm &> /dev/null; then
    echo "错误: 未找到 pnpm，请先安装"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go，请先安装"
    exit 1
fi

# 获取项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo ""
echo "[1/5] 清理旧的构建产物..."
rm -rf backend/internal/web/dist/
rm -f sub2api sub2api_linux_amd64

echo ""
echo "[2/5] 安装前端依赖..."
cd frontend
pnpm install --frozen-lockfile

echo ""
echo "[3/5] 构建前端..."
pnpm run build

# 检查前端构建产物
if [ ! -d "../backend/internal/web/dist" ]; then
    echo "错误: 前端构建失败，未找到 dist/ 目录"
    exit 1
fi

DIST_FILE_COUNT=$(find ../backend/internal/web/dist -type f | wc -l)
echo "前端构建完成: $DIST_FILE_COUNT 个文件"

cd ..

echo ""
echo "[4/5] 构建 Linux amd64 后端二进制 (embed 模式)..."
cd backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags embed -ldflags="-s -w" -o ../sub2api_linux_amd64 ./cmd/server && cd ..

# 检查二进制
if [ ! -f "sub2api_linux_amd64" ]; then
    echo "错误: 二进制构建失败"
    exit 1
fi

echo ""
echo "[5/5] 构建完成!"
echo "=========================================="
ls -lh sub2api_linux_amd64
echo "=========================================="
echo ""
echo "部署到服务器:"
echo "  scp sub2api_linux_amd64 root@186.241.72.237:/opt/sub2api/sub2api.new"
echo ""
echo "在服务器上执行:"
echo "  systemctl stop sub2api"
echo "  cd /opt/sub2api"
echo "  cp sub2api sub2api.bak.\$(date +%Y%m%d-%H%M%S)"
echo "  mv sub2api.new sub2api"
echo "  chmod +x sub2api"
echo "  systemctl start sub2api"
echo ""
