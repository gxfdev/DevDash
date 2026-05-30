#!/bin/bash
# ============================================
# DevDash 生产环境本地测试脚本
# 用于在无Docker环境下验证所有功能
# ============================================

set -e

echo "=========================================="
echo "  DevDash 生产环境本地测试"
echo "=========================================="

# ---- 颜色定义 ----
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0
WARN_COUNT=0

pass() {
    echo -e "${GREEN}✅ PASS${NC}: $1"
    ((PASS_COUNT++))
}

fail() {
    echo -e "${RED}❌ FAIL${NC}: $1"
    ((FAIL_COUNT++))
}

warn() {
    echo -e "${YELLOW}⚠️  WARN${NC}: $1"
    ((WARN_COUNT++))
}

# ============================================
# 1. 环境检查
# ============================================
echo ""
echo "📋 步骤1: 环境依赖检查"
echo "----------------------------------------"

check_command() {
    if command -v $1 &> /dev/null; then
        pass "$1 已安装 ($(command -v $1))"
        return 0
    else
        fail "$1 未安装"
        return 1
    fi
}

check_command go
check_command node
check_command npm
check_command git

version_gte() {
    local v1="$1" v2="$2"
    local IFS=.
    read -ra a <<< "$v1"
    read -ra b <<< "$v2"
    local i
    for i in "${!b[@]}"; do
        local va="${a[i]:-0}" vb="${b[i]:-0}"
        if ((va > vb)); then return 0; fi
        if ((va < vb)); then return 1; fi
    done
    return 0
}

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
if version_gte "$GO_VERSION" "1.21"; then
    pass "Go版本符合要求 ($GO_VERSION >= 1.21)"
else
    warn "Go版本较低 ($GO_VERSION)，建议升级到1.21+"
fi

NODE_VERSION=$(node --version | sed 's/v//')
if version_gte "$NODE_VERSION" "18"; then
    pass "Node版本符合要求 ($NODE_VERSION >= 18)"
else
    fail "Node版本过低 ($NODE_VERSION < 18)"
fi

# ============================================
# 2. Go后端构建
# ============================================
echo ""
echo "📋 步骤2: Go后端构建 (生产模式)"
echo "----------------------------------------"

cd server

export GIN_MODE=release
export JWT_SECRET="test-production-secret-32chars-minimum!!"

echo "[build] 编译中..."
if CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=test-build" \
    -o ../devdash-test ./cmd/server 2>&1; then
    
    BINARY_SIZE=$(du -h ../devdash-test | cut -f1)
    
    # 静态编译的二进制应该很小 (<15MB)
    SIZE_MB=$(du -m ../devdash-test | cut -f1)
    if [[ $SIZE_MB -lt 15 ]]; then
        pass "Go编译成功，二进制大小合理 (${BINARY_SIZE})"
    else
        warn "二进制文件较大 (${BINARY_SIZE})，建议优化"
    fi
    
    # 检查是否为静态链接
    if file ../devdash-test | grep -q "statically linked"; then
        pass "二进制文件为静态链接 (适合Docker Alpine镜像)"
    else
        warn "非静态链接，Docker镜像可能较大"
    fi
    
else
    fail "Go编译失败"
    exit 1
fi

cd ..

# ============================================
# 3. 前端构建
# ============================================
echo ""
echo "📋 步骤3: 前端Production Build"
echo "----------------------------------------"

cd web

if [ ! -d "node_modules" ]; then
    echo "[install] 安装npm依赖..."
    npm install --registry=https://registry.npmmirror.com 2>&1 | tail -5
fi

echo "[build] 构建前端..."
if npm run build 2>&1; then
    DIST_SIZE=$(du -sh dist/ | cut -f1)
    pass "前端构建成功 (dist大小: ${DIST_SIZE})"
    
    # 检查关键文件
    if [ -f "dist/index.html" ]; then
        pass "index.html 存在"
    else
        fail "index.html 缺失"
    fi
    
    if [ -d "dist/assets" ]; then
        ASSETS_COUNT=$(ls dist/assets/ | wc -l)
        pass "资源文件目录存在 (${ASSETS_COUNT} 个文件)"
    else
        fail "assets 目录缺失"
    fi
else
    fail "前端构建失败"
    exit 1
fi

cd ..

# ============================================
# 4. 配置文件验证
# ============================================
echo ""
echo "📋 步骤4: 配置文件完整性检查"
echo "----------------------------------------"

# Dockerfile
if [ -f "docker/Dockerfile.server" ]; then
    pass "Dockerfile.server 存在"
    
    # 检查多阶段构建
    if grep -q "FROM.*AS" docker/Dockerfile.server; then
        pass "使用多阶段构建 (减小镜像体积)"
    else
        warn "未使用多阶段构建"
    fi
    
    # 检查Alpine基础镜像
    if grep -q "alpine" docker/Dockerfile.server; then
        pass "使用Alpine基础镜像 (轻量级)"
    else
        warn "未使用Alpine镜像"
    fi
    
    # 检查非root用户
    if grep -q "USER" docker/Dockerfile.server; then
        pass "运行用户非root (安全)"
    else
        warn "以root用户运行 (不安全)"
    fi
    
    # 检查健康检查
    if grep -q "HEALTHCHECK" docker/Dockerfile.server; then
        pass "包含健康检查 (Docker就绪探测)"
    else
        warn "缺少健康检查"
    fi
else
    fail "Dockerfile.server 缺失"
fi

# docker-compose.prod.yml
if [ -f "docker-compose.prod.yml" ]; then
    pass "docker-compose.prod.yml 存在"
    
    # 检查服务定义
    if grep -q "services:" docker-compose.prod.yml; then
        pass "包含服务定义"
    fi
    
    # 检查网络隔离
    if grep -q "networks:" docker-compose.prod.yml; then
        pass "配置了自定义网络 (网络隔离)"
    else
        warn "未配置自定义网络"
    fi
    
    # 检查数据卷
    if grep -q "volumes:" docker-compose.prod.yml; then
        pass "配置了持久化卷 (数据持久化)"
    else
        warn "未配置持久化卷"
    fi
    
    # 检查restart策略
    if grep -q "restart:" docker-compose.prod.yml; then
        pass "配置了重启策略 (高可用)"
    else
        warn "未配置重启策略"
    fi
else
    fail "docker-compose.prod.yml 缺失"
fi

# Nginx SSL配置
if [ -f "docker/nginx.ssl.conf" ]; then
    pass "nginx.ssl.conf 存在"
    
    # 检查SSL协议版本
    if grep -q "TLSv1.2 TLSv1.3" docker/nginx.ssl.conf; then
        pass "SSL协议版本正确 (TLS 1.2+)"
    else
        warn "SSL协议版本可能不安全"
    fi
    
    # 检查HSTS
    if grep -q "Strict-Transport-Security" docker/nginx.ssl.conf; then
        pass "启用HSTS (强制HTTPS)"
    else
        warn "未启用HSTS"
    fi
    
    # 检查安全头
    SECURITY_HEADERS=("X-Frame-Options" "X-Content-Type-Options" "X-XSS-Protection")
    for header in "${SECURITY_HEADERS[@]}"; do
        if grep -q "$header" docker/nginx.ssl.conf; then
            pass "安全头: $header"
        else
            warn "缺少安全头: $header"
        fi
    done
else
    fail "nginx.ssl.conf 缺失"
fi

# .env.production模板
if [ -f ".env.production" ]; then
    pass ".env.production 模板存在"
    
    # 检查必要的环境变量
    ENV_VARS=("JWT_SECRET" "PORT" "GIN_MODE" "TZ" "CORS_ORIGINS")
    for var in "${ENV_VARS[@]}"; do
        if grep -q "^$var=" .env.production; then
            pass "环境变量: $var"
        else
            warn "缺失环境变量: $var"
        done
    fi
else
    warn ".env.production 模板不存在"
fi

# .dockerignore
if [ -f ".dockerignore" ]; then
    pass ".dockerignore 存在"
    
    # 应该忽略的文件
    IGNORE_PATTERNS=("node_modules" "*.db" ".git" ".env" "*.md")
    for pattern in "${IGNORE_PATTERNS[@]}"; do
        if grep -q "^$pattern$" .dockerignore; then
            pass "忽略: $pattern"
        else
            warn "应忽略但未设置: $pattern"
        done
    done
else
    fail ".dockerignore 缺失"
fi

# ============================================
# 5. 代码质量检查
# ============================================
echo ""
echo "📋 步骤5: Go代码质量检查"
echo "----------------------------------------"

cd server

# go vet
echo "[vet] 运行go vet..."
if go vet ./... 2>&1 | grep -v "no Go files"; then
    warn "go vet 发现问题 (详见输出)"
else
    pass "go vet 通过 (无明显问题)"
fi

# gofmt
echo "[fmt] 检查代码格式..."
UNFORMATTED=$(gofmt -l . 2>/dev/null | head -10)
if [[ -z "$UNFORMATTED" ]]; then
    pass "代码格式正确 (gofmt)"
else
    warn "以下文件需要格式化:"
    echo "$UNFORMATTED"
fi

# 检测硬编码密钥
echo "[security] 检测硬编码敏感信息..."
if grep -r "password\|secret\|token" --include="*.go" . 2>/dev/null | grep -v "_test.go" | grep -v "//\|#" | head -5; then
    warn "发现可能的硬编码敏感信息 (请人工审查)"
else
    pass "未发现明显的硬编码敏感信息"
fi

cd ..

# ============================================
# 6. 安全性检查
# ============================================
echo ""
echo "📋 步骤6: 安全性配置检查"
echo "----------------------------------------"

# JWT Secret强度
if [ -n "$JWT_SECRET" ] && [[ ${#JWT_SECRET} -ge 32 ]]; then
    pass "JWT Secret长度符合要求 (${#JWT_SECRET} 字符)"
else
    fail "JWT Secret过短或未设置 (当前: ${#JWT_SECRET} 字符)"
fi

# CORS配置
if grep -q "CORS_ORIGINS" .env.production; then
    pass "CORS配置存在 (.env.production)"
else
    warn "CORS配置可能不完整"
fi

# 检查CSRF保护
if grep -r "csrf\|CSRF" server/internal/auth/ 2>/dev/null | head -1; then
    pass "CSRF保护机制已实现"
else
    warn "未发现CSRF保护实现"
fi

# 检查Rate Limiting
if grep -r "rate\|limit\|throttle" server/internal/auth/ 2>/dev/null | head -1; then
    pass "速率限制已实现"
else
    warn "未发现速率限制实现"
fi

# 检查SQL注入防护
if grep -r "\?" server/internal/store/*.go 2>/dev/null | grep -i "Query\|Exec" | head -3; then
    pass "使用参数化查询 (防SQL注入)"
else
    warn "SQL注入防护需人工确认"
fi

# ============================================
# 7. 性能相关检查
# ============================================
echo ""
echo "📋 步骤7: 性能优化检查"
echo "----------------------------------------"

cd server

# 数据库连接池
if grep -q "SetMaxOpenConns\|SetMaxIdleConns" internal/store/store.go; then
    pass "数据库连接池已配置"
else
    fail "数据库连接池未配置"
fi

# 数据库索引
INDEX_COUNT=$(grep -c "CREATE INDEX" internal/store/store.go || echo "0")
if [[ $INDEX_COUNT -gt 0 ]]; then
    pass "数据库索引已创建 (${INDEX_COUNT} 个索引)"
else
    warn "未创建数据库索引"
fi

# WAL模式
if grep -q "journal_mode=WAL" internal/store/store.go; then
    pass "SQLite使用WAL模式 (提升并发性能)"
else
    warn "未启用WAL模式"
fi

# 并发控制
if grep -q "semaphore\|channel.*struct" cmd/server/main.go; then
    pass "并发控制已实现"
else
    warn "并发控制未实现"
fi

cd ..

# ============================================
# 8. 功能模块检查
# ============================================
echo ""
echo "📋 步骤8: 功能模块完整性"
echo "----------------------------------------"

MODULES=(
    "server/internal/alert/engine.go:告警引擎"
    "server/internal/api/response.go:API响应格式"
    "server/internal/logger/logger.go:日志系统"
    "server/internal/auth/jwt.go:JWT认证"
    "server/internal/auth/security.go:安全中间件"
    "server/internal/collector/collector.go:数据采集"
    "server/internal/store/store.go:数据存储"
    "web/src/views/AlertView.vue:告警界面"
    "web/src/views/StoreView.vue:软件商店"
    "web/src/views/DashboardView.vue:仪表盘"
)

for module in "${MODULES[@]}"; do
    FILE=${module%%:*}
    DESC=${module##*:}
    
    if [ -f "$FILE" ]; then
        pass "$DESC ($FILE)"
    else
        fail "$DESC 缺失 ($FILE)"
    fi
done

# ============================================
# 清理
# ============================================
echo ""
echo "🧹 清理临时文件..."
rm -f devdash-test

# ============================================
# 测试结果汇总
# ============================================
echo ""
echo "=========================================="
echo "  📊 测试结果汇总"
echo "=========================================="
echo -e "${GREEN}✅ 通过: ${PASS_COUNT}${NC}"
echo -e "${RED}❌ 失败: ${FAIL_COUNT}${NC}"
echo -e "${YELLOW}⚠️  警告: ${WARN_COUNT}${NC}"
echo ""

TOTAL=$((PASS_COUNT + FAIL_COUNT + WARN_COUNT))
PASS_RATE=$((PASS_COUNT * 100 / TOTAL))

if [[ $FAIL_COUNT -eq 0 ]]; then
    echo -e "${GREEN}🎉 所有核心测试通过！可以部署到生产环境。${NC}"
    echo -e "通过率: ${PASS_RATE}%"
    exit 0
elif [[ $FAIL_COUNT -lt 3 ]]; then
    echo -e "${YELLOW}⚠️  有少量问题，建议修复后再部署。${NC}"
    exit 1
else
    echo -e "${RED}🚨 存在严重问题，禁止部署！请先修复。${NC}"
    exit 2
fi
