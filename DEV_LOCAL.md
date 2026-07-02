# Sub2API 本地开发环境

## 环境准备

已完成配置：
- ✅ PostgreSQL 18 (Docker容器，端口: 5434)
- ✅ Redis 8 (Docker容器，端口: 6381)
- ✅ Go 依赖已下载
- ✅ 前端依赖已安装 (pnpm)

## 快速启动

### 1. 启动数据库 (已启动)

```bash
cd deploy
docker compose -f docker-compose.db-only.yml up -d
```

查看状态：
```bash
docker compose -f docker-compose.db-only.yml ps
```

停止数据库：
```bash
docker compose -f docker-compose.db-only.yml down
```

### 2. 启动后端 (Go)

**方式一：使用启动脚本**
```bash
./start-backend.sh
```

**方式二：手动启动**
```bash
cd backend
export CONFIG_FILE=config.dev.yaml
export AUTO_SETUP=true
go run ./cmd/server
```

后端地址: http://localhost:8080

默认管理员账号：
- 邮箱: admin@sub2api.local
- 密码: admin123

### 3. 启动前端 (Vue 3)

**新开一个终端，运行：**

```bash
./start-frontend.sh
```

或手动启动：
```bash
cd frontend
pnpm run dev
```

前端地址: http://localhost:5173

## 数据库连接信息

- Host: 127.0.0.1
- Port: 5434 (映射到容器的5432)
- User: sub2api
- Password: sub2api_dev_password
- Database: sub2api

## Redis 连接信息

- Host: 127.0.0.1
- Port: 6381 (映射到容器的6379)
- Password: (无)
- DB: 0

## 开发提示

### 后端开发

1. 配置文件：`backend/config.dev.yaml`
2. 主程序：`backend/cmd/server/main.go`
3. 数据库Schema：`backend/ent/schema/`
4. API处理器：`backend/internal/handler/`

修改Schema后重新生成：
```bash
cd backend
go generate ./ent
```

### 前端开发

1. API配置：`frontend/src/api/`
2. 路由：`frontend/src/router/`
3. 组件：`frontend/src/components/`
4. 视图：`frontend/src/views/`

### 运行测试

后端测试：
```bash
cd backend
go test -tags=unit ./...           # 单元测试
go test -tags=integration ./...    # 集成测试
```

前端测试：
```bash
cd frontend
pnpm run test              # 运行所有测试
pnpm run lint:check        # 代码检查
pnpm run typecheck         # 类型检查
```

## 常见问题

### 数据库连接失败
检查容器是否健康：
```bash
docker ps | grep sub2api
```

查看容器日志：
```bash
docker logs sub2api-postgres-dev
docker logs sub2api-redis-dev
```

### 端口冲突
如果5434或6381端口被占用，可以修改 `deploy/docker-compose.db-only.yml` 中的端口映射。

### 重置数据库
```bash
cd deploy
docker compose -f docker-compose.db-only.yml down -v
rm -rf postgres_data redis_data
docker compose -f docker-compose.db-only.yml up -d
```

## 项目架构

```
sub2api/
├── backend/           # Go后端 (Gin + Ent ORM)
│   ├── cmd/          # 程序入口
│   ├── internal/     # 业务逻辑
│   ├── ent/          # 数据库ORM
│   └── config.dev.yaml
├── frontend/         # Vue 3前端
│   ├── src/
│   └── package.json
├── deploy/           # 部署配置
│   ├── docker-compose.db-only.yml
│   └── docker-compose.local.yml
├── start-backend.sh
└── start-frontend.sh
```
