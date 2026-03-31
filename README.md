# 🔒 AI 代码审计平台



一个基于 AI 的自动化代码审计平台，支持多种编程语言的漏洞检测、跨文件调用链分析和实时审计报告生成。

![Platform Preview](./frontend/hhh.jpeg)


## 🚀 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/yourusername/platform.git
cd platform
```

### 2. 配置环境变量

编辑 `.env` 文件，配置必要的参数：

```env
# 数据库配置
POSTGRES_USER=your_user
POSTGRES_PASSWORD=your_password
POSTGRES_DB=platform

# JWT 配置
JWT_SECRET=your_jwt_secret_key
JWT_EXPIRE_HOUR=24

# AI 模型配置（可选，通过前端界面配置）
OPENAI_API_KEY=your_api_key
OPENAI_BASE_URL=https://api.openai.com/v1

# 邮件配置（可选）
SMTP_SERVER=smtp.example.com
SMTP_PORT=465
```

### 3. 构建并启动

直接使用 Docker Compose 启动：

```bash
docker compose up -d 
```

### 4. 访问平台

打开浏览器访问：`http://localhost:4416`

默认端口：
- 前端：http://localhost:4416
- 后端：http://localhost:8080

## 📖 使用指南

### 1. 用户注册/登录

首次使用时注册账号并登录系统。

### 2. 配置 AI 模型

在「模型配置」页面添加您的 AI 模型：

| 参数 | 说明 |
|------|------|
| API Key | OpenAI 或兼容 API 的密钥 |
| Base URL | API 地址（留空使用 OpenAI 默认） |
| 模型名称 | 如 `gpt-4`、`gpt-3.5-turbo` |
| Temperature | 生成温度参数 (0-1) |

### 3. 上传代码源

在「代码源管理」页面：

1. 点击「上传代码」按钮
2. 选择要审计的代码（支持 ZIP 格式）
3. 系统自动解压并存储代码

### 4. 创建审计任务

在「创建任务」页面：

1. 选择代码源
2. 选择 AI 模型
3. （可选）自定义审计提示词
4. 点击「开始审计」

### 5. 查看审计报告

审计完成后，在「报告列表」查看：

- 漏洞统计概览
- 漏洞详情（位置、严重程度、修复建议）
- 生成的 Markdown 报告

## 🏗️ 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         前端 (Vue 3 + TypeScript)                │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐   │
│  │ 仪表盘  │  │ 代码源   │  │ 任务管理 │  │   模型配置      │   │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                                │ HTTP/WebSocket
┌─────────────────────────────────────────────────────────────────┐
│                         后端 (Go + Gin)                          │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐   │
│  │ 认证模块 │  │ 任务调度 │  │ MCP服务  │  │  WebSocket     │   │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              AI 审计引擎 (OpenAI API 兼容)               │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌───────────────┐  │    │
│  │  │ 单文件扫描   │  │ 调用链分析   │  │ 漏洞复查验证  │  │    │
│  │  └─────────────┘  └─────────────┘  └───────────────┘  │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│   PostgreSQL  │     │     CFR       │     │  沙盒目录     │
│   (数据库)     │     │  (Java反编译) │     │  (代码存储)   │
└───────────────┘     └───────────────┘     └───────────────┘
```

## 📁 项目结构

```
platform/
├── backend/                          # 后端服务
│   ├── cmd/
│   │   └── main.go                  # 程序入口
│   ├── config/
│   │   └── config.go                # 配置管理
│   ├── handler/                     # HTTP 处理器
│   │   ├── auth.go                  # 认证相关
│   │   ├── task.go                  # 任务管理
│   │   ├── analysis.go              # 分析结果
│   │   ├── code_source.go           # 代码源管理
│   │   ├── model.go                 # 模型配置
│   │   └── report.go                # 报告管理
│   ├── middleware/                   # 中间件
│   │   ├── auth.go                  # JWT 认证
│   │   └── ratelimit.go             # 限流
│   ├── model/                       # 数据模型
│   │   ├── user.go                  # 用户模型
│   │   ├── task.go                  # 任务模型
│   │   ├── code_source.go           # 代码源模型
│   │   └── model_config.go          # 模型配置模型
│   ├── mcp/                         # MCP 核心服务
│   │   ├── audit_service.go         # 审计服务
│   │   ├── code_analyzer.go         # 代码分析器
│   │   └── callgraph_analyzer.go    # 调用图分析器
│   ├── router/
│   │   └── router.go                # 路由配置
│   ├── util/                        # 工具函数
│   │   ├── database.go              # 数据库连接
│   │   ├── cache.go                 # 缓存
│   │   ├── common.go                # 通用工具
│   │   └── email.go                 # 邮件发送
│   ├── websocket/
│   │   └── progress.go              # WebSocket 进度推送
│   └── sandbox/
│       └── cfr.jar                  # Java 反编译器
├── frontend/                        # 前端应用
│   ├── src/
│   │   ├── api/                     # API 调用
│   │   ├── components/              # 公共组件
│   │   ├── pages/                   # 页面组件
│   │   ├── router/                  # 路由配置
│   │   ├── stores/                  # Pinia 状态
│   │   ├── types/                   # TypeScript 类型
│   │   ├── locales/                 # 国际化
│   │   └── App.vue                  # 根组件
│   ├── package.json
│   └── vite.config.ts
├── init-scripts/                    # 数据库初始化脚本
│   └── 01-init.sql
├── docker-compose.yml               # Docker 编排
├── .env                            # 环境配置
└── README.md
```

## 🔧 API 接口

### 认证接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/auth/register | 用户注册 |
| POST | /api/auth/login | 用户登录 |
| POST | /api/auth/logout | 用户登出 |

### 代码源接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/code-sources | 获取代码源列表 |
| POST | /api/code-sources | 上传代码源 |
| GET | /api/code-sources/:id | 获取代码源详情 |
| DELETE | /api/code-sources/:id | 删除代码源 |

### 任务接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/tasks | 获取任务列表 |
| POST | /api/tasks | 创建审计任务 |
| GET | /api/tasks/:id | 获取任务详情 |
| DELETE | /api/tasks/:id | 删除任务 |
| GET | /api/tasks/:id/progress | 获取任务进度 (WebSocket) |

### 报告接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/reports | 获取报告列表 |
| GET | /api/reports/:id | 获取报告详情 |

### 模型配置接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/models | 获取模型列表 |
| POST | /api/models | 添加模型配置 |
| PUT | /api/models/:id | 更新模型配置 |
| DELETE | /api/models/:id | 删除模型配置 |

## 🔐 安全说明

1. **数据隔离**：审计任务在沙箱环境中执行，防止恶意代码影响系统
2. **路径安全**：所有文件操作经过严格路径校验，防止路径遍历攻击
3. **认证授权**：使用 JWT 进行用户认证和接口访问控制
4. **敏感信息**：API 密钥等敏感配置通过环境变量管理

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 📄 授权
欢迎开发者基于本项目进行二次开发、学习与交流，推动更多创新应用落地。但本项目不允许用于商业用途，请在非商业范围内使用。

