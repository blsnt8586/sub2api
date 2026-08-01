# ✅ 按模型定价功能 - 执行清单

## 已完成任务

### 1. 后端代码实现 ✅
- [x] 创建 `model_pricing_config.go` - 模型定价配置结构
- [x] 修改 `billing_service.go` - 视频/图片计费支持模型参数
- [x] 修改 `gateway_usage_billing.go` - 传递模型定价配置
- [x] 修改 `openai_gateway_usage.go` - 传递模型名称
- [x] 修改 `media_price_config.go` - 构建配置时包含 ModelPricing
- [x] 修改 `group.go` - Group 结构添加 ModelPricing 字段
- [x] 编译验证通过

### 2. 数据库迁移 ✅
- [x] 创建迁移文件 `191_add_groups_model_pricing.sql`
- [x] 执行迁移添加 `model_pricing` JSONB 字段
- [x] 验证字段已创建

### 3. 模型定价配置 ✅
- [x] 为 jimeng 平台分组配置 5 个视频模型定价
- [x] 为 jimeng 平台分组配置 5 个图片模型定价
- [x] 验证配置已正确存储

### 4. 服务部署 ✅
- [x] 重启后端服务
- [x] 服务健康检查通过 (http://localhost:8080/health)
- [x] 配置已生效

### 5. 文档 ✅
- [x] `MODEL_PRICING.md` - 完整功能说明文档
- [x] `AVI2API_MODEL_PRICING_QUICKSTART.md` - AIV2API 快速配置指南
- [x] `DEPLOYMENT_SUMMARY.md` - 部署总结文档

## 当前配置摘要

### 视频模型（按次计费）
```json
{
  "seedance-fast": {"per_count": 0.06},
  "seedance-v1": {"per_count": 0.08},
  "seedance-v2": {"per_count": 0.12},
  "kling-v1": {"per_count": 0.15},
  "kling-v1.5": {"per_count": 0.18}
}
```

### 图片模型（按尺寸计费）
```json
{
  "leonardo-phoenix": {"1k": 0.01, "2k": 0.02, "4k": 0.05},
  "leonardo-anime": {"1k": 0.01, "2k": 0.02, "4k": 0.04},
  "flux-dev": {"1k": 0.01, "2k": 0.02, "4k": 0.04},
  "flux-pro": {"1k": 0.02, "2k": 0.04, "4k": 0.08},
  "leonardo-kino": {"1k": 0.03, "2k": 0.06, "4k": 0.12}
}
```

## 核心特性

✅ **三级计费优先级**
```
模型专属定价 > 分组全局定价 > 系统默认定价
```

✅ **灵活的计费模式**
- 视频：支持按次或按秒计费
- 图片：支持按尺寸分档计费

✅ **向后兼容**
- 未配置的模型自动回退到全局定价
- 现有分组无需修改继续工作

✅ **即时生效**
- 修改配置后立即生效
- 无需重启服务

## 快速操作指南

### 查看当前配置
```bash
docker compose -f deploy/docker-compose.db-only.yml exec postgres \
  psql -U sub2api -d sub2api -c "
SELECT id, name, jsonb_pretty(model_pricing) 
FROM groups WHERE platform = 'jimeng';
"
```

### 修改单个价格
```bash
docker compose -f deploy/docker-compose.db-only.yml exec -T postgres \
  psql -U sub2api -d sub2api << 'EOF'
UPDATE groups
SET model_pricing = jsonb_set(
    model_pricing,
    '{video,seedance-v2,per_count}',
    '0.15'::jsonb
)
WHERE platform = 'jimeng';
EOF
```

### 添加新模型
```bash
docker compose -f deploy/docker-compose.db-only.yml exec -T postgres \
  psql -U sub2api -d sub2api << 'EOF'
UPDATE groups
SET model_pricing = model_pricing || '{
  "video": {
    "new-model": {"per_count": 0.10}
  }
}'::jsonb
WHERE platform = 'jimeng';
EOF
```

### 查看计费记录
```bash
docker compose -f deploy/docker-compose.db-only.yml exec postgres \
  psql -U sub2api -d sub2api -c "
SELECT model, cost, created_at 
FROM usage_records 
WHERE platform = 'jimeng'
ORDER BY created_at DESC 
LIMIT 10;
"
```

## 测试验证命令

```bash
# 1. 测试视频生成（seedance-v2 应按 $0.12 计费）
curl -X POST http://localhost:8080/v1/videos \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-v2",
    "prompt": "测试视频",
    "seconds": "5"
  }'

# 2. 测试图片生成（leonardo-kino 2K 应按 $0.06 计费）
curl -X POST http://localhost:8080/v1/images/generations \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "leonardo-kino",
    "prompt": "测试图片",
    "size": "2048x2048"
  }'

# 3. 验证计费
docker compose -f deploy/docker-compose.db-only.yml exec postgres \
  psql -U sub2api -d sub2api -c "
SELECT model, cost, created_at 
FROM usage_records 
ORDER BY created_at DESC LIMIT 5;
"
```

## 相关文件

### 代码
- `backend/internal/service/model_pricing_config.go`
- `backend/internal/service/billing_service.go`
- `backend/internal/service/gateway_usage_billing.go`
- `backend/internal/service/openai_gateway_usage.go`
- `backend/internal/service/media_price_config.go`
- `backend/internal/service/group.go`

### 数据库
- `backend/migrations/191_add_groups_model_pricing.sql`

### 文档
- `MODEL_PRICING.md`
- `AVI2API_MODEL_PRICING_QUICKSTART.md`
- `DEPLOYMENT_SUMMARY.md`
- `EXECUTION_CHECKLIST.md` (本文件)

## 注意事项

⚠️ **模型名必须完全匹配**
- 配置中的键必须与请求中的 `model` 字段完全一致
- 区分大小写

⚠️ **查看实际模型名**
```bash
# 从数据库查看实际使用的模型名
docker compose -f deploy/docker-compose.db-only.yml exec postgres \
  psql -U sub2api -d sub2api -c "
SELECT DISTINCT model 
FROM usage_records 
WHERE platform = 'jimeng' 
ORDER BY model;
"
```

⚠️ **配置验证**
修改配置后建议先查询确认：
```bash
docker compose -f deploy/docker-compose.db-only.yml exec postgres \
  psql -U sub2api -d sub2api -c "
SELECT jsonb_pretty(model_pricing) 
FROM groups 
WHERE platform = 'jimeng';
"
```

## 状态总结

🎉 **功能已完整实现并成功部署**
- ✅ 后端代码编译通过
- ✅ 数据库迁移完成
- ✅ 模型定价已配置
- ✅ 服务运行正常
- ✅ 健康检查通过

**可以开始使用按模型精确计费！**
