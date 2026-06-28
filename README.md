# UserCenter — 统一用户中心与 AI 网关

企业级身份认证、权限管理、AI 网关的统一平台。支持 OIDC/OAuth2 认证、SCIM 2.0 用户/组同步、OpenAI 兼容 AI 网关鉴权与用量计量。

## 架构概览

```
┌─────────────────────────────────────┐
│  UserCenter                         │
│  ┌─────────────────────────────┐    │
│  │ Auth (JWT / OIDC / MFA)     │    │
│  │ RBAC (Casbin)               │    │
│  │ User / Role / Tenant        │    │
│  │ SCIM 2.0 User & Group sync  │    │
│  └─────────────────────────────┘    │
│  ┌─────────────────────────────┐    │
│  │ AI Gateway (OpenAI compat)  │    │
│  │  - Chat/Embed/Images/Audio  │    │
│  │  - Semantic cache + quotas  │    │
│  │  - Rate limiting per user   │    │
│  └─────────────────────────────┘    │
│  ┌─────────────────────────────┐    │
│  │ Admin Panel (React+Vite SPA)│    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
```

## 快速开始（开发模式）

依赖：Go 1.25+、本地 MySQL 8.0+（默认端口 13306）。

```bash
# 启动纯 HTTP 开发服务（自动建表 + 播种管理员）
NO_PROXY=localhost,127.0.0.1 go run ./cmd/devserver/

# 打开管理后台
open http://localhost:48080/web/admin
# 默认管理员：admin / Admin@123456
```

环境变量：`UC_MYSQL_DSN`（MySQL 连接串）、`UC_PORT`（HTTP 端口）、`UC_TOKEN_KEY`（token 签名密钥）。

## 功能特性

### 身份认证
- 用户名密码 / 微信扫码 / 小程序 / 工号登录
- OIDC/OAuth2 Provider（authorization_code / client_credentials / refresh_token + PKCE）
- TOTP MFA 两阶段登录（enroll → confirm → 挑战-响应）
- 登录失败锁定 + 暴力破解告警

### 权限管理
- 基于 Casbin 的 RBAC 权限模型（菜单 + API 两级）
- 多租户数据隔离

### AI 网关
OpenAI 兼容端点：`/v1/chat/completions`（流式/非流式）、`/v1/embeddings`、`/v1/images/generations`、`/v1/audio/transcriptions`、`/v1/audio/speech`、`/v1/moderations`、`/v1/models`

内置：模型路由 + API Key 主从池 + 429 自动冷却、语义缓存、用量配额、每用户限流、对话会话管理、Prompt 模板注入、请求日志。

### SCIM 2.0 同步
`/scim/v2/Users`（用户 CRUD + filter）和 `/scim/v2/Groups`（角色 CRUD + 成员管理），Bearer Token 鉴权。

### 管理面板
内嵌 React+Vite SPA（`/web/admin`）：用户/角色管理、AI Provider/Key/Route 配置、用量面板、请求日志、MFA 管理、Webhook 订阅等。

## 生产部署

依赖：Nacos 配置中心、MySQL 8.0+、可选 Redis。

```bash
DUBBO_GO_CONFIG_PATH="./dubbogo.yaml" go run main.go
```

配置通过 Nacos（dataId: `usercenter-config`，group: `nooocode`）或 `/etc/usercenter/config.yaml` 加载。

## SDK

- [Go SDK](sdk/go/) — 完整覆盖 + AI 网关 + 单元测试（6 用例）
- [Python SDK](sdk/python/) — `pip install usercenter-client`
- [TypeScript SDK](sdk/typescript/) — `npm install usercenter-client`

## 构建

```bash
make web          # 构建前端 SPA
make build-image  # Docker 镜像
make gen-doc      # 生成 Swagger 文档
```
