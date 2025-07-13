# Stripe对账系统

## 概述

Stripe对账系统是一个完整的自动化对账解决方案，用于确保本地数据库与Stripe平台的数据一致性。系统支持定时对账、差异检测、自动修复和审计日志等功能。

## 功能特性

### ✅ 定期数据对比
- 支持Cron表达式配置的定时对账
- 默认每天凌晨2点执行对账任务
- 可配置对账时间范围（日、周、月）

### ✅ 差异检测和报告
- 自动检测订阅、发票、客户数据的差异
- 支持多种差异类型：缺失、不匹配、多余
- 差异严重程度分级：低、中、高、严重
- 生成详细的对账报告

### ✅ 数据修复机制
- 自动修复功能（可配置严重程度阈值）
- 手动修复接口
- 差异忽略功能
- 修复历史记录

### ✅ 对账审计功能
- 完整的操作审计日志
- 修复操作追踪
- 系统状态监控
- 通知机制

## 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   定时调度器     │    │   对账服务      │    │   API接口       │
│   Scheduler     │───▶│ Reconciliation  │───▶│   Handler       │
└─────────────────┘    │   Service       │    └─────────────────┘
                       └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐
                       │   数据库        │
                       │   Supabase      │
                       └─────────────────┘
```

## 数据库表结构

### reconciliation_reports (对账报告表)
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| report_date | TIMESTAMP | 报告日期 |
| start_date | TIMESTAMP | 对账开始时间 |
| end_date | TIMESTAMP | 对账结束时间 |
| status | VARCHAR(20) | 状态：pending/running/completed/failed |
| total_records | INTEGER | 总记录数 |
| matched_records | INTEGER | 匹配记录数 |
| mismatched_records | INTEGER | 不匹配记录数 |
| missing_records | INTEGER | 缺失记录数 |

### reconciliation_diffs (对账差异表)
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| report_id | UUID | 报告ID |
| record_type | VARCHAR(20) | 记录类型：subscription/invoice/customer |
| record_id | VARCHAR(255) | 记录ID |
| user_id | VARCHAR(255) | 用户ID |
| diff_type | VARCHAR(20) | 差异类型：missing/mismatch/extra |
| field_name | VARCHAR(100) | 字段名 |
| local_value | TEXT | 本地值 |
| stripe_value | TEXT | Stripe值 |
| severity | VARCHAR(20) | 严重程度：low/medium/high/critical |
| status | VARCHAR(20) | 状态：pending/auto_fixed/manual_fixed/ignored |

### reconciliation_audit_logs (审计日志表)
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| report_id | UUID | 报告ID |
| action | VARCHAR(50) | 操作类型 |
| record_type | VARCHAR(20) | 记录类型 |
| record_id | VARCHAR(255) | 记录ID |
| user_id | VARCHAR(255) | 用户ID |
| details | TEXT | 详细信息 |
| performed_by | VARCHAR(100) | 执行者 |

## API接口

### 对账报告管理
- `GET /admin/reconciliation/reports` - 获取对账报告列表
- `GET /admin/reconciliation/reports/{reportID}` - 获取单个对账报告
- `GET /admin/reconciliation/reports/{reportID}/diffs` - 获取报告差异列表
- `GET /admin/reconciliation/reports/{reportID}/audit-logs` - 获取报告审计日志

### 对账任务管理
- `POST /admin/reconciliation/run` - 执行对账任务
- `POST /admin/reconciliation/run-manual` - 手动执行对账任务
- `GET /admin/reconciliation/status` - 获取对账状态
- `POST /admin/reconciliation/schedule` - 更新调度配置

### 差异管理
- `PATCH /admin/reconciliation/diffs/{diffID}/fix` - 修复差异
- `PATCH /admin/reconciliation/diffs/{diffID}/ignore` - 忽略差异
- `GET /admin/reconciliation/diffs` - 获取所有差异

### 配置管理
- `GET /admin/reconciliation/config` - 获取对账配置
- `PUT /admin/reconciliation/config` - 更新对账配置

## 配置说明

### 对账配置 (reconciliation_config)
```json
{
  "id": "default",
  "enabled": true,
  "schedule": "0 2 * * *",
  "auto_fix_enabled": true,
  "auto_fix_severity": "medium",
  "notification_enabled": true,
  "notification_email": "admin@example.com",
  "retention_days": 30
}
```

### 配置参数说明
- `enabled`: 是否启用对账功能
- `schedule`: Cron表达式，定义对账执行时间
- `auto_fix_enabled`: 是否启用自动修复
- `auto_fix_severity`: 自动修复的严重程度阈值
- `notification_enabled`: 是否启用通知
- `notification_email`: 通知邮箱地址
- `retention_days`: 数据保留天数

## 部署指南

### 1. 环境准备
```bash
# 安装依赖
go mod tidy

# 设置环境变量
export SUPABASE_URL="your_supabase_url"
export SUPABASE_SECRET_KEY="your_supabase_secret_key"
export STRIPE_SECRET_KEY="your_stripe_secret_key"
```

### 2. 数据库迁移
```bash
# 执行数据库迁移
psql -h your_db_host -U your_db_user -d your_db_name -f migrations/001_create_reconciliation_tables.sql
```

### 3. 启动服务

#### 方式1：使用Cobra命令
```bash
# 启动API服务
go run main.go api

# 启动队列服务
go run main.go queue

# 启动对账服务
go run main.go reconciliation
```

#### 方式2：使用启动脚本
```bash
chmod +x scripts/start_reconciliation.sh
./scripts/start_reconciliation.sh
```

#### 方式3：构建后运行
```bash
# 构建所有服务
go build -o bin/json_trans_api main.go

# 运行对账服务
./bin/json_trans_api reconciliation
```

### 4. 验证部署
```bash
# 检查服务状态
curl -X GET "http://localhost:8080/admin/reconciliation/status" \
  -H "Authorization: Bearer your_admin_token"
```

## 使用示例

### 1. 手动执行对账
```bash
curl -X POST "http://localhost:8080/admin/reconciliation/run-manual" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your_admin_token" \
  -d '{
    "start_date": "2024-01-01",
    "end_date": "2024-01-31"
  }'
```

### 2. 查看对账报告
```bash
curl -X GET "http://localhost:8080/admin/reconciliation/reports" \
  -H "Authorization: Bearer your_admin_token"
```

### 3. 修复差异
```bash
curl -X PATCH "http://localhost:8080/admin/reconciliation/diffs/diff_id/fix" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your_admin_token" \
  -d '{
    "notes": "手动修复订阅状态"
  }'
```

### 4. 更新配置
```bash
curl -X PUT "http://localhost:8080/admin/reconciliation/config" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your_admin_token" \
  -d '{
    "enabled": true,
    "schedule": "0 3 * * *",
    "auto_fix_enabled": true,
    "auto_fix_severity": "high",
    "notification_enabled": true,
    "notification_email": "admin@company.com",
    "retention_days": 60
  }'
```

## 监控和告警

### 1. 日志监控
系统会记录详细的操作日志，包括：
- 对账任务执行状态
- 差异检测结果
- 修复操作记录
- 错误信息

### 2. 性能指标
- 对账执行时间
- 差异数量统计
- 自动修复成功率
- API响应时间

### 3. 告警机制
- 对账失败告警
- 差异数量超阈值告警
- 系统异常告警

## 故障排除

### 常见问题

1. **对账任务执行失败**
   - 检查数据库连接
   - 验证Stripe API密钥
   - 查看错误日志

2. **差异数量异常**
   - 检查对账时间范围
   - 验证数据源完整性
   - 分析差异类型分布

3. **自动修复失败**
   - 检查修复权限
   - 验证数据格式
   - 查看修复日志

### 调试命令
```bash
# 查看服务日志
tail -f logs/reconciliation.log

# 检查数据库连接
curl -H "apikey: $SUPABASE_SECRET_KEY" \
  "$SUPABASE_URL/rest/v1/reconciliation_config?select=id&limit=1"

# 测试Stripe连接
curl -H "Authorization: Bearer $STRIPE_SECRET_KEY" \
  "https://api.stripe.com/v1/customers?limit=1"
```

## 安全考虑

1. **访问控制**
   - 所有API接口需要管理员权限
   - 使用JWT token认证
   - 实施RLS策略

2. **数据保护**
   - 敏感数据加密存储
   - 审计日志完整性
   - 数据访问权限控制

3. **系统安全**
   - 定期更新依赖
   - 监控异常访问
   - 备份重要数据

## 扩展功能

### 未来计划
- [ ] 支持更多数据源对账
- [ ] 可视化对账报告
- [ ] 机器学习差异预测
- [ ] 实时对账功能
- [ ] 多租户支持

### 自定义扩展
系统采用模块化设计，可以轻松扩展：
- 添加新的对账数据类型
- 自定义差异检测规则
- 实现特定的修复逻辑
- 集成第三方通知服务 