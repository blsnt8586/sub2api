# AIV2API 模型定价快速配置指南

## 📋 支持的模型列表

### 视频模型（Video）
根据 AIV2API 常见模型：
- `seedance-v1` - Seedance 第一代（基础版本）
- `seedance-v2` - Seedance 第二代（高清版本）
- `seedance-fast` - Seedance 快速版（低延迟）
- `kling-v1` - Kling 第一代
- `kling-v1.5` - Kling 增强版

### 图片模型（Image）
- `leonardo-phoenix` - Leonardo Phoenix 风格
- `leonardo-kino` - Leonardo Kino 电影风格（高质量）
- `leonardo-anime` - Leonardo 动漫风格
- `flux-pro` - Flux Pro（专业版）
- `flux-dev` - Flux Dev（开发版）

## 🚀 快速部署

### 步骤 1：运行数据库迁移
```bash
cd /workspace/code/008_sub2api/sub2api/backend
go run ./cmd/migrate up
```

### 步骤 2：应用模型定价配置

**选项 A：推荐配置（按次计费，简单明了）**
```bash
psql -U sub2api_dev -d sub2api_dev << 'EOF'
UPDATE groups
SET model_pricing = '{
  "video": {
    "seedance-v1": {"per_count": 0.08},
    "seedance-v2": {"per_count": 0.12},
    "seedance-fast": {"per_count": 0.06},
    "kling-v1": {"per_count": 0.15},
    "kling-v1.5": {"per_count": 0.18}
  },
  "image": {
    "leonardo-phoenix": {"1k": 0.01, "2k": 0.02, "4k": 0.05},
    "leonardo-kino": {"1k": 0.03, "2k": 0.06, "4k": 0.12},
    "leonardo-anime": {"1k": 0.01, "2k": 0.02, "4k": 0.04},
    "flux-pro": {"1k": 0.02, "2k": 0.04, "4k": 0.08},
    "flux-dev": {"1k": 0.01, "2k": 0.02, "4k": 0.04}
  }
}'::jsonb
WHERE platform = 'jimeng';
EOF
```

**选项 B：精细配置（按秒计费）**
```bash
psql -U sub2api_dev -d sub2api_dev << 'EOF'
UPDATE groups
SET model_pricing = '{
  "video": {
    "seedance-v1": {"per_second": 0.015},
    "seedance-v2": {"per_second": 0.02},
    "seedance-fast": {"per_second": 0.01},
    "kling-v1": {"per_second": 0.025},
    "kling-v1.5": {"per_second": 0.03}
  },
  "image": {
    "leonardo-phoenix": {"1k": 0.01, "2k": 0.02, "4k": 0.05},
    "leonardo-kino": {"1k": 0.03, "2k": 0.06, "4k": 0.12},
    "flux-pro": {"1k": 0.02, "2k": 0.04, "4k": 0.08}
  }
}'::jsonb
WHERE platform = 'jimeng';
EOF
```

### 步骤 3：重启服务
```bash
cd /workspace/code/008_sub2api/sub2api
./start-backend.sh
```

### 步骤 4：验证配置
```bash
psql -U sub2api_dev -d sub2api_dev -c "
SELECT 
    id, 
    name, 
    jsonb_pretty(model_pricing) as pricing_config
FROM groups 
WHERE platform = 'jimeng' AND model_pricing IS NOT NULL;
"
```

## 💰 定价参考

### 视频模型定价（按次）
| 模型 | 推荐价格 | 说明 |
|------|---------|------|
| seedance-fast | $0.06 | 快速生成，适合预览 |
| seedance-v1 | $0.08 | 标准质量 |
| seedance-v2 | $0.12 | 高清质量 |
| kling-v1 | $0.15 | Kling 基础版 |
| kling-v1.5 | $0.18 | Kling 增强版 |

### 视频模型定价（按秒）
| 模型 | 推荐价格 | 说明 |
|------|---------|------|
| seedance-fast | $0.01/秒 | 5秒视频 = $0.05 |
| seedance-v1 | $0.015/秒 | 5秒视频 = $0.075 |
| seedance-v2 | $0.02/秒 | 5秒视频 = $0.10 |
| kling-v1 | $0.025/秒 | 5秒视频 = $0.125 |
| kling-v1.5 | $0.03/秒 | 5秒视频 = $0.15 |

### 图片模型定价
| 模型 | 1K | 2K | 4K | 说明 |
|------|-----|-----|-----|------|
| leonardo-phoenix | $0.01 | $0.02 | $0.05 | 基础风格 |
| leonardo-anime | $0.01 | $0.02 | $0.04 | 动漫风格 |
| flux-dev | $0.01 | $0.02 | $0.04 | 开发版 |
| flux-pro | $0.02 | $0.04 | $0.08 | 专业版 |
| leonardo-kino | $0.03 | $0.06 | $0.12 | 电影级高质量 |

## 🎯 计费示例

### 视频按次计费
```
用户请求: {"model":"seedance-v2", "prompt":"...", "seconds":"5"}
计费: $0.12 (固定价格，不管时长)
```

### 视频按秒计费
```
用户请求: {"model":"seedance-v2", "prompt":"...", "seconds":"5"}
计费: $0.02 × 5 = $0.10

用户请求: {"model":"seedance-v2", "prompt":"...", "seconds":"10"}
计费: $0.02 × 10 = $0.20
```

### 图片计费
```
用户请求: {"model":"leonardo-kino", "size":"2048x2048"}
实际生成: 2048x2048 (2K档位)
计费: $0.06

用户请求: {"model":"flux-dev", "size":"1024x1024"}
实际生成: 1024x1024 (1K档位)
计费: $0.01
```

## 🔧 自定义模型

如果 AIV2API 新增了其他模型，直接在 JSON 中添加：

```sql
UPDATE groups
SET model_pricing = jsonb_set(
    model_pricing,
    '{video,new-model-name}',
    '{"per_count": 0.10}'::jsonb
)
WHERE platform = 'jimeng';
```

或者直接修改整个配置：
```sql
UPDATE groups
SET model_pricing = model_pricing || '{
  "video": {
    "new-video-model": {"per_count": 0.10}
  },
  "image": {
    "new-image-model": {"1k": 0.01, "2k": 0.02}
  }
}'::jsonb
WHERE platform = 'jimeng';
```

## 📊 查看当前计费记录

```sql
-- 查看最近的视频生成计费
SELECT 
    model,
    cost,
    metadata->>'video_count' as video_count,
    metadata->>'video_seconds' as video_seconds,
    created_at
FROM usage_records
WHERE model LIKE 'seedance%' OR model LIKE 'kling%'
ORDER BY created_at DESC
LIMIT 10;

-- 查看最近的图片生成计费
SELECT 
    model,
    cost,
    metadata->>'image_count' as image_count,
    metadata->>'image_size' as image_size,
    created_at
FROM usage_records
WHERE model LIKE 'leonardo%' OR model LIKE 'flux%'
ORDER BY created_at DESC
LIMIT 10;

-- 统计各模型使用量和收入
SELECT 
    model,
    COUNT(*) as usage_count,
    SUM(cost) as total_revenue
FROM usage_records
WHERE platform = 'jimeng'
    AND created_at >= NOW() - INTERVAL '7 days'
GROUP BY model
ORDER BY total_revenue DESC;
```

## ⚠️ 重要提示

1. **模型名必须完全匹配**
   - 配置中的键（如 `"seedance-v1"`）必须与请求中的 `model` 字段完全一致
   - 区分大小写

2. **未配置的模型会回退**
   - 如果模型不在 `model_pricing` 中，会使用分组的全局定价
   - 全局定价字段：`video_price_per_count`、`image_price_1k` 等

3. **即时生效**
   - 修改 `model_pricing` 后立即生效，无需重启服务

4. **查看实际模型名**
   ```bash
   # 从日志中查看实际使用的模型名
   grep "model" /var/log/sub2api/backend.log | tail -20
   
   # 从数据库查看
   SELECT DISTINCT model FROM usage_records WHERE platform = 'jimeng' ORDER BY model;
   ```

## 🧪 测试验证

```bash
# 1. 生成视频（seedance-v2）
curl -X POST http://localhost:8080/v1/videos \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-v2",
    "prompt": "测试视频生成",
    "seconds": "5"
  }'

# 2. 生成图片（leonardo-kino）
curl -X POST http://localhost:8080/v1/images/generations \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "leonardo-kino",
    "prompt": "测试图片生成",
    "size": "2048x2048"
  }'

# 3. 查看计费记录
psql -U sub2api_dev -d sub2api_dev -c "
SELECT model, cost, created_at 
FROM usage_records 
ORDER BY created_at DESC 
LIMIT 5;
"
```

期望结果：
- `seedance-v2` 视频：$0.12（按次）或 $0.10（按秒 × 5）
- `leonardo-kino` 2K 图片：$0.06
