#!/bin/bash

# AIThink API 测试脚本

BASE_URL="http://localhost:8080"

echo "=== AIThink API 测试 ==="
echo ""

# 1. 健康检查
echo "1. 健康检查..."
curl -s "$BASE_URL/health" | jq .
echo ""

# 2. 登录测试（需要手动输入验证码）
echo "2. 登录测试..."
echo "注意: 智谱清言登录需要验证码，此测试可能失败"
curl -s -X POST "$BASE_URL/api/v1/login" \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "zhipu",
    "username": "13800138000",
    "password": "123456"
  }' | jq .
echo ""

# 3. 如果有session_id，可以测试提问
if [ -n "$SESSION_ID" ]; then
    echo "3. 提问测试..."
    curl -s -X POST "$BASE_URL/api/v1/ask" \
      -H "Content-Type: application/json" \
      -d "{
        \"platform\": \"zhipu\",
        \"session_id\": \"$SESSION_ID\",
        \"question\": \"你好\"
      }" | jq .
    echo ""
fi

echo "=== 测试完成 ==="
