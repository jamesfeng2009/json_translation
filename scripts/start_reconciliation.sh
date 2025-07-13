#!/bin/bash

# 对账服务启动脚本

echo "启动Stripe对账服务..."

# 检查环境变量
if [ -z "$SUPABASE_URL" ] || [ -z "$SUPABASE_SECRET_KEY" ]; then
    echo "错误: 缺少必要的环境变量"
    echo "请设置以下环境变量:"
    echo "  - SUPABASE_URL"
    echo "  - SUPABASE_SECRET_KEY"
    echo "  - STRIPE_SECRET_KEY"
    exit 1
fi

# 检查数据库连接
echo "检查数据库连接..."
curl -s -o /dev/null -w "%{http_code}" \
  -H "apikey: $SUPABASE_SECRET_KEY" \
  -H "Authorization: Bearer $SUPABASE_SECRET_KEY" \
  "$SUPABASE_URL/rest/v1/reconciliation_config?select=id&limit=1"

if [ $? -ne 0 ]; then
    echo "错误: 无法连接到数据库"
    exit 1
fi

echo "数据库连接正常"

# 启动对账调度器
echo "启动对账调度器..."
go run main.go reconciliation &

# 保存进程ID
RECONCILIATION_PID=$!
echo $RECONCILIATION_PID > /tmp/reconciliation.pid

echo "对账服务已启动，进程ID: $RECONCILIATION_PID"

# 等待进程结束
wait $RECONCILIATION_PID

echo "对账服务已停止" 