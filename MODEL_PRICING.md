# 按模型定价功能说明

## 概述

支持为不同的视频/图片生成模型设置不同的价格，特别适用于即梦（jimeng）平台接入多个模型的场景。

## 计费优先级

```
模型专属定价 > 分组全局定价 > 系统默认定价
```

### 视频计费优先级（详细）
1. `model_pricing.video[模型名].per_second` — 模型专属按秒计费
2. `model_pricing.video[模型名].per_count` — 模型专属按次计费
3. `video_price_per_second` — 分组全局按秒计费
4. `video_price_per_count` — 分组全局按次计费
5. 系统默认值：$0.05/次

### 图片计费优先级（详细）
1. `model_pricing.image[模型名][尺寸]` — 模型专属定价
2. `image_price_1k/2k/4k` — 分组全局定价
3. 系统默认值：$0.02/张（1K/2K）、$0.05/张（4K）

## 数据库字段

### groups 表新增字段
```sql
model_pricing JSONB DEFAULT NULL
```

### JSON 结构
```json
{
  "video": {
    "模型名": {
      "per_count": 0.08,      // USD/次（可选）
      "per_second": 0.02      // USD/秒（可选，优先级高于 per_count）
    }
  },
  "image": {
    "模型名": {
      "1k": 0.01,  // USD/张
      "2k": 0.02,  // USD/张
      "4k": 0.05   // USD/张
    }
  }
}
```

## 配置示例

### 示例 1：视频按次计费（简单场景）

```json
{
  "video": {
    "seedance-v1": {
      "per_count": 0.08
    },
    "seedance-v2": {
      "per_count": 0.12
    },
    "kling-v1": {
      "per_count": 0.15
    }
  }
}
```

**效果**：
- `seedance-v1` 模型：每生成 1 个视频扣 $0.08
- `seedance-v2` 模型：每生成 1 个视频扣 $0.12
- `kling-v1` 模型：每生成 1 个视频扣 $0.15
- 其他未配置模型：回退到分组全局定价或系统默认值

### 示例 2：视频按秒计费（精细控制）

```json
{
  "video": {
    "seedance-v1": {
      "per_second": 0.02
    },
    "seedance-v2-hd": {
      "per_second": 0.05
    }
  }
}
```

**效果**：
- `seedance-v1` 生成 5 秒视频：$0.02 × 5 = $0.10
- `seedance-v1` 生成 10 秒视频：$0.02 × 10 = $0.20
- `seedance-v2-hd` 生成 5 秒视频：$0.05 × 5 = $0.25

### 示例 3：图片按模型+尺寸定价

```json
{
  "image": {
    "leonardo-phoenix": {
      "1k": 0.01,
      "2k": 0.02,
      "4k": 0.05
    },
    "leonardo-kino": {
      "1k": 0.03,
      "2k": 0.06,
      "4k": 0.12
    },
    "flux-pro": {
      "1k": 0.02,
      "2k": 0.04,
      "4k": 0.08
    }
  }
}
```

**效果**：
- `leonardo-phoenix` 生成 1024x1024：$0.01/张
- `leonardo-phoenix` 生成 2048x2048：$0.02/张
- `leonardo-kino` 生成 2048x2048：$0.06/张（更贵）
- `flux-pro` 生成 4096x4096：$0.08/张

### 示例 4：混合配置（视频+图片）

```json
{
  "video": {
    "seedance-v1": {
      "per_count": 0.08
    },
    "seedance-v2": {
      "per_second": 0.02
    }
  },
  "image": {
    "leonardo-phoenix": {
      "1k": 0.01,
      "2k": 0.02,
      "4k": 0.05
    }
  }
}
```

## 配置方式

### 方式 1：管理端界面（推荐）

1. 登录管理端 `/admin/groups`
2. 编辑即梦平台分组
3. 找到"模型定价配置"区块
4. 填写 JSON 配置
5. 保存

### 方式 2：直接修改数据库

```sql
UPDATE groups 
SET model_pricing = '{
  "video": {
    "seedance-v1": {"per_count": 0.08},
    "seedance-v2": {"per_second": 0.02}
  },
  "image": {
    "leonardo-phoenix": {"1k": 0.01, "2k": 0.02, "4k": 0.05}
  }
}'::jsonb
WHERE id = 1 AND platform = 'jimeng';
```

### 方式 3：API（如有）

```bash
curl -X PATCH /api/v1/admin/groups/{id} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_pricing": {
      "video": {
        "seedance-v1": {"per_count": 0.08}
      }
    }
  }'
```

## 兼容性说明

### 向后兼容
- `model_pricing` 字段为 `NULL` 时，使用原有的全局定价字段：
  - 视频：`video_price_per_count` / `video_price_per_second`
  - 图片：`image_price_1k` / `image_price_2k` / `image_price_4k`
- 已有分组无需修改，继续按原逻辑计费

### 混合使用
可以同时配置模型专属定价和全局定价：
```json
// groups 表字段
video_price_per_count = 0.05  // 全局兜底价格
model_pricing = {
  "video": {
    "seedance-v2": {"per_count": 0.12}  // seedance-v2 专属价格
  }
}
```
**效果**：
- `seedance-v2`：$0.12/次（使用模型专属定价）
- 其他模型：$0.05/次（使用全局定价）

## 常见场景

### 场景 1：上游按积分扣费，转成 USD
假设：
- `seedance-v1` 生成 1 个视频扣 1600 积分
- `seedance-v2` 生成 1 个视频扣 2400 积分
- 1000 积分 = $0.05

配置：
```json
{
  "video": {
    "seedance-v1": {
      "per_count": 0.08  // 1600 × 0.05 / 1000
    },
    "seedance-v2": {
      "per_count": 0.12  // 2400 × 0.05 / 1000
    }
  }
}
```

### 场景 2：高清视频按秒收费更贵
配置：
```json
{
  "video": {
    "seedance-standard": {
      "per_second": 0.01
    },
    "seedance-hd": {
      "per_second": 0.03
    },
    "seedance-4k": {
      "per_second": 0.08
    }
  }
}
```

### 场景 3：不同风格图片模型定价差异
配置：
```json
{
  "image": {
    "leonardo-phoenix": {
      "2k": 0.02
    },
    "leonardo-kino": {
      "2k": 0.06
    },
    "flux-pro": {
      "2k": 0.04
    }
  }
}
```

## 迁移步骤

### 1. 运行数据库迁移
```bash
cd backend
go run ./cmd/migrate up
```

### 2. 重启服务
```bash
./start-backend.sh
```

### 3. 配置模型定价
通过管理端或 SQL 更新 `groups.model_pricing` 字段

### 4. 验证计费
查看 `usage_records` 表的 `cost` 字段，确认按模型正确计费

## 测试验证

### 验证脚本
```bash
# 1. 生成视频（seedance-v1）
curl -X POST http://localhost:8080/v1/videos \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{"model":"seedance-v1","prompt":"test","seconds":"5"}'

# 2. 查看计费记录
psql -U sub2api_dev -d sub2api_dev -c "
  SELECT model, cost, created_at 
  FROM usage_records 
  ORDER BY created_at DESC 
  LIMIT 5;
"
```

## 注意事项

1. **模型名必须完全匹配**：配置中的模型名必须与请求中的 `model` 字段完全一致（区分大小写）
2. **JSON 格式校验**：错误的 JSON 格式会导致整个配置失效，回退到全局定价
3. **价格单位**：所有价格单位均为 USD（美元）
4. **即时生效**：修改配置后立即生效，无需重启服务
5. **日志查看**：计费日志会记录使用的定价来源（模型专属/分组全局/系统默认）

## 故障排查

### 问题：配置后未生效
1. 检查 `model_pricing` 字段的 JSON 格式是否正确
2. 确认请求中的 `model` 字段与配置中的键完全一致
3. 查看日志：`grep "billing" /var/log/sub2api/backend.log`

### 问题：部分模型按旧价格计费
- 正常现象，未在 `model_pricing` 中配置的模型会回退到全局定价

### 问题：价格显示为 0
- 检查是否同时缺少模型专属定价、全局定价和系统默认值
- 确认 `rate_multiplier` 是否设置为 0

## 相关文件

- 数据库迁移：`backend/migrations/191_add_groups_model_pricing.sql`
- 计费逻辑：`backend/internal/service/billing_service.go`
- 模型定价配置：`backend/internal/service/model_pricing_config.go`
- 前端配置界面：`frontend/src/views/admin/GroupsView.vue`（待补充 UI）
