# Sub2API 本地开发环境搭建完成 ✅

## 当前状态

✅ **数据库服务** (Docker)
- PostgreSQL 18: `127.0.0.1:5434` (运行中)
- Redis 8: `127.0.0.1:6381` (运行中)

✅ **后端服务** (Go)
- 编译测试通过
- 配置文件: `backend/config.dev.yaml`
- 依赖已下载

✅ **前端服务** (Vue 3 + Vite)
- 依赖已安装 (pnpm)
- 准备就绪

---

## 🚀 启动开发环境

### 第一步：检查数据库（已启动）

```bash
cd deploy
docker compose -f docker-compose.db-only.yml ps
```

应该看到两个容器状态为 `healthy`。

### 第二步：启动后端

**在终端1中运行：**

```bash
./start-backend.sh
```

或手动运行：
```bash
cd backend
export CONFIG_FILE=config.dev.yaml
export AUTO_SETUP=true
go run ./cmd/server
```

后端启动后会自动：
1. 连接数据库（127.0.0.1:5434）
2. 运行数据库迁移
3. 创建管理员账号
4. 监听 http://localhost:8080

### 第三步：启动前端

**在终端2中运行：**

```bash
./start-frontend.sh
```

或手动运行：
```bash
cd frontend
pnpm run dev
```

前端开发服务器会启动在: http://localhost:5173

---

## 📋 默认配置

### 数据库连接
```yaml
PostgreSQL:
  host: 127.0.0.1
  port: 5434
  user: sub2api
  password: sub2api_dev_password
  database: sub2api

Redis:
  host: 127.0.0.1
  port: 6381
  password: (无)
```

### 管理员账号
```
邮箱: admin@sub2api.local
密码: admin123
```

首次启动后端时会自动创建。

---

## 🔧 开发工具命令

### 后端开发

```bash
cd backend

# 运行服务
go run ./cmd/server

# 编译
go build -o server ./cmd/server

# 单元测试
go test -tags=unit ./...

# 集成测试
go test -tags=integration ./...

# 代码检查
golangci-lint run ./...

# 重新生成 Ent ORM 代码
go generate ./ent
```

### 前端开发

```bash
cd frontend

# 开发服务器
pnpm run dev

# 构建生产版本
pnpm run build

# 运行测试
pnpm run test

# 类型检查
pnpm run typecheck

# 代码检查
pnpm run lint:check

# 自动修复
pnpm run lint:fix
```

### 数据库管理

```bash
cd deploy

# 启动数据库
docker compose -f docker-compose.db-only.yml up -d

# 查看状态
docker compose -f docker-compose.db-only.yml ps

# 查看日志
docker compose -f docker-compose.db-only.yml logs -f

# 停止数据库
docker compose -f docker-compose.db-only.yml down

# 完全重置（删除所有数据）
docker compose -f docker-compose.db-only.yml down -v
rm -rf postgres_data redis_data
docker compose -f docker-compose.db-only.yml up -d
```

---

## 📁 项目结构

```
sub2api/
├── backend/                    # Go 后端
│   ├── cmd/
│   │   └── server/            # 主程序入口
│   ├── internal/              # 业务逻辑
│   │   ├── handler/           # HTTP 处理器
│   │   ├── service/           # 业务服务
│   │   └── repository/        # 数据访问层
│   ├── ent/                   # Ent ORM
│   │   └── schema/            # 数据库模型定义
│   ├── migrations/            # SQL 迁移脚本
│   └── config.dev.yaml        # 本地开发配置
│
├── frontend/                  # Vue 3 前端
│   ├── src/
│   │   ├── api/              # API 客户端
│   │   ├── components/       # Vue 组件
│   │   ├── views/            # 页面视图
│   │   ├── router/           # 路由配置
│   │   └── stores/           # Pinia 状态管理
│   └── package.json
│
├── deploy/                    # 部署配置
│   ├── docker-compose.db-only.yml    # 仅数据库（开发用）
│   └── docker-compose.local.yml      # 完整部署
│
├── start-backend.sh           # 后端启动脚本
├── start-frontend.sh          # 前端启动脚本
└── README_DEV.md             # 本文档
```

---

## 🐛 常见问题

### 1. 后端连接数据库失败

**检查容器状态：**
```bash
docker ps | grep sub2api
```

**查看容器日志：**
```bash
docker logs sub2api-postgres-dev
```

**重启容器：**
```bash
cd deploy
docker compose -f docker-compose.db-only.yml restart
```

### 2. 端口被占用

如果 5434 或 6381 被占用，编辑 `deploy/docker-compose.db-only.yml`：

```yaml
ports:
  - "5435:5432"  # 改成其他端口
```

然后同步修改 `backend/config.dev.yaml` 中的端口。

### 3. 前端无法连接后端

检查后端是否在运行：
```bash
curl http://localhost:8080/health
```

检查前端 API 配置：
```bash
cat frontend/src/api/config.ts
```

### 4. Go 版本不匹配

项目要求 Go 1.26.4，当前系统是 1.22.2。如果遇到编译问题，需要升级 Go：

```bash
# 下载并安装新版本
wget https://go.dev/dl/go1.26.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.26.4.linux-amd64.tar.gz
go version
```

### 5. 重置所有数据

```bash
# 停止所有服务
cd deploy
docker compose -f docker-compose.db-only.yml down -v

# 删除数据目录
rm -rf postgres_data redis_data data

# 重新启动
docker compose -f docker-compose.db-only.yml up -d

# 重新启动后端（会重建数据库）
cd ../backend
export CONFIG_FILE=config.dev.yaml
export AUTO_SETUP=true
go run ./cmd/server
```

---

## 📚 相关文档

- [开发指南](DEV_GUIDE.md) - 详细的开发文档
- [部署文档](deploy/DOCKER.md) - Docker 部署说明
- [上游仓库](https://github.com/Wei-Shaw/sub2api) - 原始项目

---

## 🎉 开始开发

一切就绪！现在你可以：

1. **访问前端**: http://localhost:5173
2. **访问后端API**: http://localhost:8080
3. **健康检查**: http://localhost:8080/health
4. **使用管理员账号登录**: admin@sub2api.local / admin123

祝开发愉快！🚀
