#!/bin/bash
# Sub2API 前端开发服务器启动脚本

set -e

echo "======================================"
echo "Sub2API 前端开发服务器"
echo "======================================"
echo ""
echo "前端地址: http://localhost:5173"
echo "后端API: http://localhost:8080"
echo ""
echo "======================================"

cd frontend
pnpm run dev
