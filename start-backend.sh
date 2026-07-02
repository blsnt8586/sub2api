#!/bin/bash
# Sub2API 本地开发环境启动脚本

set -e

echo "======================================"
echo "Sub2API 本地开发环境启动"
echo "======================================"

# 检查数据库容器
echo ""
echo "1. 检查数据库容器状态..."
cd deploy
if ! docker compose -f docker-compose.db-only.yml ps | grep -q "healthy"; then
    echo "   启动数据库容器..."
    docker compose -f docker-compose.db-only.yml up -d
    echo "   等待数据库就绪..."
    sleep 5
fi
echo "   ✓ PostgreSQL (5434) 和 Redis (6381) 运行中"

cd ..

# 启动后端
echo ""
echo "2. 启动后端服务..."
echo "   配置文件: backend/config.dev.yaml"
echo "   地址: http://localhost:8080"
echo ""

cd backend
export CONFIG_FILE=config.dev.yaml
export AUTO_SETUP=true  # 自动初始化数据库

echo "   执行命令: go run ./cmd/server"
echo ""
echo "======================================"
echo "后端日志输出:"
echo "======================================"

go run ./cmd/server
