#!/bin/bash

# 安全审计平台 - 用户注册脚本
# 用法: ./register_user.sh <username> <email> <password>
# 示例: ./register_user.sh admin admin@example.com password123

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 默认配置
API_URL="${API_URL:-http://localhost:8080}"
TIMEOUT=10

# 检测是否使用 docker 模式
# 方式1: 环境变量 USE_DOCKER=true
# 方式2: 第一个参数为 --docker
# 方式3: 环境变量 API_URL 已设置为 http://backend:8080
if [ "$USE_DOCKER" = "true" ] || [ "$1" = "--docker" ] || [ "$API_URL" = "http://backend:8080" ]; then
    # 跳过 --docker 参数
    if [ "$1" = "--docker" ]; then
        shift
    fi
    # 使用 docker exec 在 backend 容器内执行 curl
    DOCKER_EXEC=true
    API_URL="http://localhost:8080"
fi

# 打印帮助
show_help() {
    echo "用法: $0 <用户名> <邮箱> <密码>"
    echo ""
    echo "参数:"
    echo "  用户名    - 用户登录名称"
    echo "  邮箱      - 用户邮箱地址"
    echo "  密码      - 用户登录密码"
    echo ""
    echo "示例:"
    echo "  $0 admin admin@example.com mypassword123"
    echo "  $0 testuser test@test.com 123456"
    echo ""
    echo "环境变量:"
    echo "  API_URL   - API 地址 (默认: http://localhost:8080)"
}

# 检查参数
if [ $# -lt 3 ]; then
    echo -e "${RED}错误: 缺少必要参数${NC}"
    show_help
    exit 1
fi

USERNAME="$1"
EMAIL="$2"
PASSWORD="$3"

# 验证输入
if [ -z "$USERNAME" ] || [ -z "$EMAIL" ] || [ -z "$PASSWORD" ]; then
    echo -e "${RED}错误: 参数不能为空${NC}"
    exit 1
fi

# 验证密码强度（后端要求：至少8位，包含大写、小写、数字、特殊字符）
if [ ${#PASSWORD} -lt 8 ]; then
    echo -e "${RED}错误: 密码长度至少8个字符${NC}"
    exit 1
fi

# 检查是否包含大写字母
if ! echo "$PASSWORD" | grep -q '[A-Z]'; then
    echo -e "${RED}错误: 密码必须包含大写字母(A-Z)${NC}"
    exit 1
fi

# 检查是否包含小写字母
if ! echo "$PASSWORD" | grep -q '[a-z]'; then
    echo -e "${RED}错误: 密码必须包含小写字母(a-z)${NC}"
    exit 1
fi

# 检查是否包含数字
if ! echo "$PASSWORD" | grep -q '[0-9]'; then
    echo -e "${RED}错误: 密码必须包含数字(0-9)${NC}"
    exit 1
fi

# 检查是否包含特殊字符
if ! echo "$PASSWORD" | grep -q '[!@#$%^&*]'; then
    echo -e "${RED}错误: 密码必须包含特殊字符(!@#$%^&*)${NC}"
    exit 1
fi

echo -e "${GREEN}正在注册用户...${NC}"
echo "  用户名: $USERNAME"
echo "  邮箱: $EMAIL"
echo "  API: $API_URL"

# 发送注册请求（完整字段）
if [ "$DOCKER_EXEC" = "true" ]; then
    RESPONSE=$(docker exec platform-backend curl -s -w "\n%{http_code}" -X POST "$API_URL/api/auth/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"username\": \"$USERNAME\",
            \"email\": \"$EMAIL\",
            \"password\": \"$PASSWORD\",
            \"phone\": \"13800138000\",
            \"company\": \"TestCompany\",
            \"industry\": \"IT\",
            \"userCount\": \"1-10\",
            \"description\": \"Test user account\"
        }" \
        --connect-timeout "$TIMEOUT" \
        --max-time "$TIMEOUT" 2>&1)
else
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/auth/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"username\": \"$USERNAME\",
            \"email\": \"$EMAIL\",
            \"password\": \"$PASSWORD\",
            \"phone\": \"13800138000\",
            \"company\": \"TestCompany\",
            \"industry\": \"IT\",
            \"userCount\": \"1-10\",
            \"description\": \"Test user account\"
        }" \
        --connect-timeout "$TIMEOUT" \
        --max-time "$TIMEOUT" 2>&1)
fi

# 分离 HTTP 状态码和响应体
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

# 解析响应
if [ "$HTTP_CODE" = "200" ]; then
    # 尝试提取 token 和用户信息
    TOKEN=$(echo "$BODY" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    if [ -n "$TOKEN" ]; then
        echo -e "${GREEN}✓ 用户注册成功!${NC}"
        echo ""
        echo "用户信息:"
        echo "$BODY" | grep -o '"username":"[^"]*"' | head -1 | sed 's/"/"/g'
        echo "$BODY" | grep -o '"email":"[^"]*"' | head -1 | sed 's/"/"/g'
        echo ""
        echo -e "${GREEN}Token: ${NC}$TOKEN"
    else
        echo -e "${GREEN}✓ 用户注册成功!${NC}"
        echo "响应: $BODY"
    fi
    exit 0
elif [ "$HTTP_CODE" = "400" ]; then
    # 解析错误消息判断具体原因
    if echo "$BODY" | grep -q "Password"; then
        echo -e "${RED}✗ 注册失败: 密码验证失败${NC}"
        echo "响应: $BODY"
    elif echo "$BODY" | grep -q "用户名已存在\|邮箱已存在"; then
        echo -e "${RED}✗ 注册失败: 用户名或邮箱已存在${NC}"
    else
        echo -e "${RED}✗ 注册失败: 请求参数错误${NC}"
        echo "响应: $BODY"
    fi
    exit 1
elif [ "$HTTP_CODE" = "422" ]; then
    echo -e "${RED}✗ 注册失败: 输入验证错误${NC}"
    echo "响应: $BODY"
    exit 1
elif [ "$HTTP_CODE" = "000" ]; then
    echo -e "${RED}✗ 连接失败: 无法连接到 API 服务器${NC}"
    echo "请确保后端服务正在运行: $API_URL"
    exit 1
else
    echo -e "${RED}✗ 注册失败 (HTTP $HTTP_CODE)${NC}"
    echo "响应: $BODY"
    exit 1
fi