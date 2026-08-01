# ✅ 按模型定价功能部署完成

## 📦 已完成的工作

### 1. 后端代码实现
- ✅ 新增 `ModelPricingConfig` 结构体 (`model_pricing_config.go`)
- ✅ 修改 `CalculateJimengVideoCost` 支持模型参数
- ✅ 修改 `getImageUnitPrice` 优先查找模型定价
- ✅ 更新调用点传递 `ModelPricing`
- ✅ 编译通过，无语法错误

### 2. 数据库迁移
- ✅ 创建迁移文件 `191_add_groups_model_pricing.sql`
- ✅ 在 `groups` 表添加 `model_pricing` JSONB 字段
- ✅ 迁移已成功执行

### 3. 模型定价配置
- ✅ 已为 `aiv2api-test` 分组配置定价
- ✅ 支持 5 个视频模型（seedance/kling 系列）
- ✅ 支持 5 个图片模型（leonardo/flux 系列）

## 🎯 当前配置详情

### 视频模型定价（按次计费）
| 模型 | 单价 | 说明 |
|------|------|------|
| seedance-fast | $0.06 | 快速生成 |
| seedance-v1 | $0.08 | 标准质量 |
| seedance-v2 | $0.12 | 高清质量 |
| kling-v1 | $0.15 | Kling 基础版 |
| kling-v1.5 | $0.18 | Kling 增强版 |

### 图片模型定价（按尺寸）
| 模型 | 1K | 2K | 4K |
|------|-----|-----|-----|
| leonardo-phoenix | $0.01 | $0.02 | $0.05 |
| leonardo-anime | $0.01 | $0.02 | $0.04 |
| flux-dev | $0.01 | $0.02 | $0.04 |
| flux-pro | $0.02 | $0.04 | $0.08 |
| leonardo-kino | $0.03 | $0.06 | $0.12 |

## 🚀 下一步

### 重启服务使配置生效
```bash
cd /workspace/code/008_sub2api/sub2api
./start-backend.sh
```

### 测试验证

**测试视频生成计费：**
```bash
# 使用 seedance-v2 模型（应该按 $0.12 计费）
curl -X POST http://localhost:8080/v1/videos \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-v2",
    "prompt": "测试视频",
    "seconds": "5"
  }'

# 查看计费记录
docker compose -f deploy/docker-compose.db-only.yml exec postgres \
  psql -U sub2api -d sub2api -c \
  "SELECT model, cost, created_at FROM usage_records ORDER BY created_at DESC LIMIT 5;"
```

**测试图片生成计费：**
```bash
# 使用 leonardo-kino 模型 2K（应该按 $0.06 计费）
curl -X POST http://localhost:8080/v1/images/generations \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "leonardo-kino",
    "prompt": "测试图片",
    "size": "2048x2048"
  }'
```

## 📋 修改定价

如果需要调整价格，直接执行：

```bash
docker compose -f deploy/docker-compose.db-only.yml exec -T postgres psql -U sub2api -d sub2api << 'EOF'
UPDATE groups
SET model_pricing = jsonb_set(
    model_pricing,
    '{video,seedance-v2,per_count}',
    '0.15'::jsonb
)
WHERE platform = 'jimeng';
EOF
```

或者完全替换配置：
```bash
docker compose -f deploy/docker-compose.db-only.yml exec -T postgres psql -U sub2api -d sub2api << 'EOF'
UPDATE groups
SET model_pricing = '{
  "video": {
    "seedance-v1": {"per_count": 0.10},
    "seedance-v2": {"per_count": 0.15}
  }
}'::jsonb
WHERE platform = 'jimeng';
EOF
```

## 🔍 监控与排查

### 查看计费记录
```bash
docker compose -f deploy/docker-compose.db-only.yml exec postgres \
  psql -U sub2api -d sub2api -c "
SELECT 
    model,
    cost,
    metadata->>'video_count' as video_count,
    metadata->>'image_count' as image_count,
    created_at
FROM usage_records
WHERE platform = 'jimeng'
ORDER BY created_at DESC
LIMIT 10;
"
```

### 统计各模型收入
```bash
docker compose -f deploy/docker-compose.db-only.yml exec postgres \
  psql -U sub2api -d sub2api -c "
SELECT 
    model,
    COUNT(*) as usage_count,
    SUM(cost) as total_revenue
FROM usage_records
WHERE platform = 'jimeng'
    AND created_at >= NOW() - INTERVAL '7 days'
GROUP BY model
ORDER BY total_revenue DESC;
"
```

### 查看当前配置
```bash
docker compose -f deploy/docker-compose.db-only.yml exec postgres \
  psql -U sub2api -d sub2api -c "
SELECT 
    id, 
    name,
    jsonb_pretty(model_pricing) as config
FROM groups 
WHERE platform = 'jimeng';
"
```

## 📚 相关文档

- **详细文档**: `MODEL_PRICING.md` - 完整功能说明和原理
- **快速入门**: `AVI2API_MODEL_PRICING_QUICKSTART.md` - AIV2API 专用配置指南
- **迁移文件**: `backend/migrations/191_add_groups_model_pricing.sql`
- **代码实现**:
  - `backend/internal/service/model_pricing_config.go`
  - `backend/internal/service/billing_service.go`
  - `backend/internal/service/gateway_usage_billing.go`

## ⚠️ 重要提示

1. **模型名必须完全匹配**：请求中的 `model` 字段必须与配置中的键完全一致
2. **即时生效**：修改配置后立即生效，无需重启（但建议重启以确保所有缓存清除）
3. **向后兼容**：未配置的模型会自动回退到分组全局定价
4. **三级优先级**：模型专属 > 分组全局 > 系统默认

## 🎉 总结

现在你的 Sub2API 平台已经支持：
- ✅ 为不同视频模型设置不同价格（按次或按秒）
- ✅ 为不同图片模型按尺寸设置不同价格
- ✅ 灵活的三级定价优先级
- ✅ 完全向后兼容原有计费方式

只需重启服务，即可开始按模型精确计费！🚀
