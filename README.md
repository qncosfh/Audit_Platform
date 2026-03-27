# 🔒 AI 代码审计平台

[English](./README_en.md) | 简体中文1

一个基于 AI 的自动化代码审计平台，支持多种编程语言的漏洞检测、跨文件调用链分析和实时审计报告生成。

![Platform Preview](./frontend/hhh.jpeg)

## ✨ 特性

### 🤖 AI 驱动的代码分析
- 支持多种编程语言：Java、Python、Go、PHP、JavaScript/TypeScript、C#、Ruby、Swift、Kotlin、Rust 等
- 智能漏洞识别与分类（SQL注入、命令注入、XSS、路径遍历、反序列化等）
- 漏洞复查验证机制，降低误报率

### 🔗 深度代码分析
- 跨文件调用链分析，构建完整的函数调用图
- 利用链追踪，发现潜在的深层漏洞
- 支持沙箱环境隔离执行

### 📊 可视化仪表盘
- 宇宙星空视图展示任务状态
- 漏洞态势统计（Critical/High/Medium/Low）
- 系统资源监控（CPU、内存、磁盘、网络）

### 🔄 实时审计流程
- WebSocket 实时进度推送
- 多并发 Worker 并行分析
- 自动生成 Markdown 格式审计报告

### 🌐 国际化支持
- 中文、英文双语支持
- 响应式界面设计

## 🏗️ 技术栈

### 后端
- **语言**: Go 1.21+
- **框架**: Gin Web Framework
- **数据库**: PostgreSQL 15
- **AI 集成**: OpenAI API 兼容接口
- **通信**: WebSocket 实时推送
- **安全**: JWT 认证、沙箱隔离

### 前端
- **框架**: Vue 3 + TypeScript
- **UI 库**: Element Plus
- **状态管理**: Pinia
- **路由**: Vue Router 4
- **国际化**: Vue I18n
- **图表**: ECharts
- **图形可视化**: Cytoscape.js

## 🚀 快速开始

### 环境要求
- Docker & Docker Compose
- Git

### 1. 克隆项目

```bash
git clone https://github.com/yourusername/platform.git
cd platform
```

### 2. 配置环境变量

复制并编辑配置文件：

```bash
cp .env.example .env
```

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

```bash
# 构建前端
cd frontend
npm install
npm run build
cd ..

# 启动所有服务
docker-compose up -d
```

或者直接使用 Docker Compose 启动：

```bash
docker-compose up -d --build
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

## 🐳 Docker 部署

### 前置要求
- Docker 20.10+
- Docker Compose 2.0+

### 构建镜像

```bash
# 构建后端镜像
docker build -t platform-backend ./backend

# 前端需要先构建
cd frontend
npm install && npm run build
cd ..
docker build -t platform-frontend ./frontend
```

### 使用 Docker Compose

```yaml
# docker-compose.yml 示例
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
    volumes:
      - postgres-data:/var/lib/postgresql/data

  backend:
    build: ./backend
    depends_on:
      - postgres
    environment:
      - DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable

  frontend:
    build: ./frontend
    ports:
      - "80:80"

volumes:
  postgres-data:
```

## 📁 项目结构

```
platform/
├── backend/                    # 后端服务
│   ├── cmd/                    # 程序入口
│   ├── config/                 # 配置管理
│   ├── handler/                # HTTP 处理器
│   ├── middleware/             # 中间件
│   ├── model/                  # 数据模型
│   ├── router/                # 路由定义
│   ├── mcp/                    # MCP 审计服务
│   │   ├── audit_service.go   # 审计核心服务
│   │   ├── code_analyzer.go   # 代码分析器
│   │   └── callgraph_analyzer.go # 调用链分析
│   ├── sandbox/                # 沙箱目录
│   ├── util/                   # 工具函数
│   └── websocket/              # WebSocket 处理
│
├── frontend/                   # 前端应用
│   ├── src/
│   │   ├── api/               # API 调用
│   │   ├── components/         # 公共组件
│   │   ├── locales/           # 国际化文件
│   │   ├── pages/             # 页面组件
│   │   ├── router/            # 路由配置
│   │   ├── stores/             # Pinia 状态
│   │   └── types/             # TypeScript 类型
│   └── dist/                  # 构建输出
│
├── init-scripts/              # 数据库初始化脚本
├── docker-compose.yml         # Docker Compose 配置
└── .env                       # 环境变量配置
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

## 📄 许可证

本项目采用 ISC 许可证。详见 [LICENSE](LICENSE) 文件。

## 🙏 致谢

- [Gin](https://github.com/gin-gonic/gin) - Go Web 框架
- [Vue.js](https://vuejs.org/) - 渐进式 JavaScript 框架
- [Element Plus](https://element-plus.org/) - Vue 3 UI 组件库
- [ECharts](https://echarts.apache.org/) - 数据可视化图表库
- [Cytoscape.js](https://js.cytoscape.org/) - 图可视化库

---

<p align="center">
  <strong>Made with ❤️ by AI Code Audit Team</strong>
</p>
