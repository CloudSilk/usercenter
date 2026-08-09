# UserCenter 使用手册

> 版本:v4.0(React + shadcn/ui 多文件管理后台)
> 适用分支:refactor/redesign-batch1

---

## 目录

1. [快速开始](#1-快速开始)
2. [配置说明](#2-配置说明)
3. [认证体系](#3-认证体系)
4. [用户管理](#4-用户管理)
5. [角色与权限](#5-角色与权限)
6. [多租户](#6-多租户)
7. [AI 能力](#7-ai-能力)
8. [安全加固](#8-安全加固)
9. [SCIM 2.0 用户供给](#9-scim-20-用户供给)
10. [SDK 使用](#10-sdk-使用)
11. [运维指南](#11-运维指南)
12. [API 参考](#12-api-参考)
13. [AI 网关（OpenAI 兼容代理）](#13-ai-网关openai-兼容代理)
14. [OIDC 身份提供者](#14-oidc-身份提供者)
15. [可观测性与告警](#15-可观测性与告警)
16. [可视化管理后台](#16-可视化管理后台)

---

## 1. 快速开始

### 1.1 环境要求

- Go 1.25+
- MySQL 5.7+ 或 SQLite(开发)
- Redis(可选,用于 token 缓存 + Casbin 多实例同步)
- Nacos(配置中心 + 服务注册)
- Java 11+(SDK 自动生成时)

### 1.2 编译运行

```bash
# 编译
go build -o usercenter main.go

# 运行(需要 Nacos 配置中心)
DUBBO_GO_CONFIG_PATH="./dubbogo.yaml" ./usercenter

# Docker
docker build -f local.Dockerfile -t usercenter .
```

### 1.3 初始化数据库

首次启动会自动执行 `AutoMigrate`,创建全部数据表:

```go
model.InitDB(dbClient, true) // debug=true 触发 AutoMigrate
```

表包括:users, roles, menus, api, casbin_rule, tenants, audit_logs, usage_records, ai_provider, ai_key, model_route, user_sessions, refresh_tokens, mfa_factors, abac_policy, oauth_clients, oauth_consent 等。

### 1.4 首次登录

默认管理员账号在 `init.go` 的 `initDB` 中自动创建(通过 `IsMust` 标记的系统数据)。

```bash
# 登录获取 token
curl -X POST http://localhost:48080/api/core/auth/user/login \
  -H "Content-Type: application/json" \
  -d '{"userName":"admin","password":"yourpassword"}'

# 返回:{"code":20000,"data":"<JWT_TOKEN>"}
```

后续请求携带:
```bash
curl -H "Authorization: Bearer <JWT_TOKEN>" http://localhost:48080/api/core/auth/user/profile
```

---

## 2. 配置说明

### 2.1 Nacos 配置(usercenter-config)

```yaml
mysql: "root:123456@(127.0.0.1:3306)/usercenter?charset=utf8mb4&parseTime=True&loc=Local"
dbType: "mysql"           # mysql 或 sqlite
debug: true               # true=开启 AutoMigrate + Swagger
token:
  key: "your-secure-key-min-16-chars"  # JWT 签名密钥(空值或默认值将拒绝启动)
  redisAddr: "127.0.0.1:6379"          # 空=使用内存缓存
  redisName: ""
  redisPwd: ""
  expired: 120            # token 过期时间(分钟)
superAdminRoleID: "super_admin"
platformTenantID: "platform"
defaultRoleID: "normal_user"
defaultPwd: ""            # 空=重置时生成随机密码
enableTenant: true
loginLock:
  maxErrCount: 5          # 连续登录失败上限
  lockMinutes: 15         # 锁定时长
```

### 2.2 声明式配置(policies.yaml)

角色、ABAC 策略、MFA 要求、限流规则可通过 `config/policies.yaml` 声明式定义,`userctl apply` 加载:

```yaml
roles:
  - id: tenant_admin
    name: 租户管理员
    menus:
      - menu_name: user
        funcs: ["view", "manage"]

abac_policies:
  - resource: user
    action: read
    role_id: normal_user
    data_scope: 3  # 仅本人

mfa_requirements:
  - action: delete_user
    required_acr: "acr:level2"
```

### 2.3 依赖治理(dependency-policy.yaml)

```yaml
allowed:
  - name: github.com/gin-gonic/gin
    reason: "HTTP 路由框架"
    owner: "@backend-infra"
    review: never

banned:
  - name: github.com/jinzhu/copier
    reason: "已改用显式映射"
```

---

## 3. 认证体系

### 3.1 三类主体(Principal)

UserCenter 区分三种身份类型:

| 类型 | type 值 | 说明 | 签发方式 |
|------|---------|------|---------|
| Human(人类) | 0 | 普通用户,通过用户名密码/微信登录 | `EncodeToken` |
| Agent(AI) | 1 | AI Agent、非人类身份(NHI) | `EncodeAgentPrincipal` |
| Service(服务) | 2 | 机器间调用(M2M) | `EncodeTokenFromPrincipal` |

```go
// 人类登录
token, _ := token.EncodeToken(currentUser)

// Agent 签发(独立路径,物理隔离)
agentToken, _ := token.EncodeAgentPrincipal("agent-001", "owner-user-1", "tenant-1", []string{"ai-reader"})

// 从 Principal 签发(通用)
token, _ := token.EncodeTokenFromPrincipal(principal.NewAgent("a1", "u1", "t1", roles))
```

### 3.2 MFA 多因子认证

```go
// 绑定 TOTP(Google Authenticator)
secret, _ := auth.GenerateTOTPSecret()
uri := auth.GenerateTOTPURI(secret, "user@example.com", "UserCenter")
// 展示二维码给用户扫描

// 验证 TOTP 码
valid := auth.VerifyTOTP(secret, "123456")
```

**强制两阶段登录（重要）**：用户绑定任何启用的 TOTP 因子后，所有登录入口（用户名密码 / 微信 / 工号）均不再直接签发 access_token，而是返回 `code=41008`（MfaRequired）+ 一次性 challenge 令牌。前端需引导用户输入 6 位码，调用 `/api/core/auth/user/mfa/verify` 完成第二因素验证后才签发 token。

```
# 第一阶段：密码登录（用户已绑定 MFA）
POST /api/core/auth/user/login
{"userName":"alice","password":"..."}
# → {"code":41008,"message":"需要 MFA 二次验证","data":"mfa_<challenge>"}

# 第二阶段：提交 TOTP 码
POST /api/core/auth/user/mfa/verify
{"mfaToken":"mfa_<challenge>","code":"123456"}
# → {"code":20000,"data":"<JWT_ACCESS_TOKEN>"}
```

| 响应码 | 含义 |
|--------|------|
| 41008 | 需要二次验证（resp.Data 为 challenge 令牌） |
| 41009 | challenge 令牌无效或已过期（5 分钟 / 单次使用） |
| 41010 | MFA 验证码不正确 |

普通登录用户应使用产品侧自助端点（都只作用于当前登录用户）：

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/core/auth/user/mfa/totp/enroll` | 生成临时密钥与 `otpauth://` URI |
| POST | `/api/core/auth/user/mfa/totp/confirm` | 校验 6 位动态码并绑定因子 |
| GET | `/api/core/auth/user/mfa/factors` | 列出当前用户已绑定因子，不返回密钥 |
| DELETE | `/api/core/auth/user/mfa/:id` | 解绑当前用户自己的因子 |

兼容管理站点仍可使用同语义的 `/admin/api/mfa/*` 路径。解绑操作会校验因子归属，不能删除其他用户的 MFA 因子。

敏感操作需要 step-up 认证:

| 操作 | 所需 ACR 级别 |
|------|-------------|
| 普通 API | Level 1(密码) |
| 删除用户/改密码 | Level 2(密码 + TOTP) |
| 导出数据/删租户 | Level 3(密码 + WebAuthn) |

### 3.3 WebAuthn / Passkey

```go
// 初始化(启动时配置 RP)
auth.InitWebAuthn(&auth.WebAuthnConfig{
    RPID:          "usercenter.example.com",
    RPDisplayName: "UserCenter",
    RPOrigins:     []string{"https://usercenter.example.com"},
})

// 注册 Passkey
options, session, _ := auth.BeginRegistration(userID, userName, displayName, existingCreds)
// → 把 options 发给浏览器,浏览器调 navigator.credentials.create()

// 完成注册
credential, _ := auth.FinishRegistration(userID, userName, displayName, existingCreds, session, parsedResponse)

// Passkey 登录
assertion, session, _ := auth.BeginLogin(userID, userName, displayName, creds)
// → 浏览器调 navigator.credentials.get()
_, _ = auth.FinishLogin(userID, creds, session, parsedResponse)
```

### 3.4 OIDC 标准端点

```
GET  /.well-known/openid-configuration  → 发现文档
POST /oauth/authorize                   → 授权码
POST /oauth/token                       → 令牌端点
GET  /oauth/userinfo                    → 用户信息
POST /oauth/revoke                      → 吊销令牌
GET  /.well-known/jwks.json             → JWKS 密钥集
```

---

## 4. 用户管理

### 4.1 API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/core/auth/user/login` | 用户名密码登录 |
| POST | `/api/core/auth/user/logout` | 登出 |
| POST | `/api/core/auth/user/token/refresh` | 轮换当前会话 Token，旧 Token 立即失效 |
| GET | `/api/core/auth/user/profile` | 获取个人信息 |
| PUT | `/api/core/auth/user/profile` | 更新个人信息 |
| POST | `/api/core/auth/user/add` | 创建用户 |
| PUT | `/api/core/auth/user/update` | 更新用户 |
| DELETE | `/api/core/auth/user/delete` | 删除用户 |
| GET | `/api/core/auth/user/query` | 分页查询 |
| GET | `/api/core/auth/user/all` | 查询全部 |
| GET | `/api/core/auth/user/detail` | 查询明细 |
| POST | `/api/core/auth/user/enable` | 启用/禁用 |
| POST | `/api/core/auth/user/resetpwd` | 重置密码 |
| POST | `/api/core/auth/user/changepwd` | 修改密码(仅自己) |
| GET | `/api/core/auth/user/export` | 导出 |
| POST | `/api/core/auth/user/import` | 导入 |

`profile`（GET/PUT）、`changepwd`、`logout`、`token/refresh` 和上述 MFA 自助端点会在 `RunMigration()` 时作为 UserCenter 必需 API 幂等登记：它们要求有效登录，但不要求产品管理角色。因此普通客户可维护自己的账号，生产鉴权模式也不需要宿主重复播种这组权限。`token/refresh` 只轮换当前人类用户的同一设备会话，不创建第二条会话；成功后旧 Token 立即失效。

### 4.2 密码安全

- 创建用户:密码强度校验(≥8 位 + 数字 + 大写 + 小写)
- 重置密码:随机密码(crypto/rand)+ 强制改密(`ForceChangePwd=true`)
- 修改密码:校验旧密码 + 新密码强度 + 仅允许改自己；成功后清除强制改密标记并失效该用户已有访问令牌
- 个人资料更新:服务端强制使用当前登录用户 ID 和租户，不接受请求体覆盖，并写入审计日志
- 存储:scrypt(兼容)或 Argon2id(新标准,透明 rehash)

```go
// Argon2id 加密(推荐)
hash, _ := auth.EncryptedPasswordArgon2("MyPassword123")

// 透明 rehash:登录时检测旧格式 → 升级
if auth.IsScryptHash(user.Password) {
    // 验证成功后自动 rehash 为 Argon2id
}
```

### 4.3 登录保护

- 连续 5 次密码错误 → 账号锁定 15 分钟(可配置)
- 同 IP 1 分钟内 3 次失败 → `[SECURITY ALERT]` 日志告警
- 锁定期内即使密码正确也拒绝登录

---

## 5. 角色与权限

### 5.1 RBAC(角色 → URL)

```go
// 角色关联菜单功能 → 自动生成 Casbin 规则
model.CreateRole(&Role{
    Name:      "租户管理员",
    RoleMenus: []*RoleMenu{
        {MenuID: "user_menu_id", Funcs: "view,manage"},
    },
})
// updateRoleAuth 自动:Menu → MenuFunc → API → Casbin Rule
```

### 5.2 ABAC(属性级授权)

```go
// 创建 ABAC 策略
permission.CreateABACPolicy(&permission.ABACPolicy{
    Resource:  "user",
    Action:    "read",
    RoleID:    "normal_user",
    DataScope: int32(permission.DataScopeSelf), // 仅本人
    Condition: `{"owner_id":{"eq":"${principal.id}"}}`,
})

// 判定
allowed, scope, _ := permission.EvaluateABAC(ctx, &permission.ABACContext{
    Principal: p,
    Resource:  "user",
    Action:    "read",
    Attrs:     map[string]interface{}{"owner_id": "u123"},
})
// → allowed=true, scope=DataScopeSelf
```

### 5.3 DataScope 数据范围

| 值 | 含义 | GORM WHERE |
|----|------|-----------|
| 0 | All | (无过滤) |
| 1 | Tenant | `tenant_id = 'xxx'` |
| 2 | Dept | `` `group` = 'xxx' `` |
| 3 | Self | `(id = 'xxx' OR created_by = 'xxx')` |
| 4 | Custom | ABAC condition 表达式 |

---

## 6. 多租户

### 6.1 租户隔离

Handler 层(已有):
```go
// 非 PlatformTenantID 的用户,查询自动追加 tenant_id
tenantID := middleware.GetTenantID(c)
if tenantID != constants.PlatformTenantID {
    req.TenantID = tenantID
}
```

DB 层(GORM scope 插件,新增):
```go
// 使用 TenantScope 自动追加 WHERE tenant_id = ?
scope := tenant.NewTenantScope(func(id string) bool {
    return id == constants.PlatformTenantID
})

ctx := tenant.WithTenantContext(c.Request.Context(), "tenant-001")
db.Scopes(scope.Apply(ctx)).Find(&users)
// → SELECT * FROM users WHERE tenant_id = 'tenant-001'
```

### 6.2 租户配额

每个租户可配置 `UserCount`(最大用户数)和 `Expired`(有效期),创建用户时自动校验。

---

## 7. AI 能力

### 7.1 AI Key 管理

```go
// 设置加密密钥(启动时)
apikey.SetEncryptionKey([]byte("32-byte-encryption-key-here!!!"))

// 创建服务商
providerID, _ := apikey.CreateProvider(&apikey.AIProvider{
    Name:    "OpenAI",
    BaseURL:  "https://api.openai.com/v1",
    AuthType: "bearer",
})

// 创建 API Key(明文加密存储)
keyID, _ := apikey.CreateKey(&apikey.AIKey{
    ProviderID: providerID,
    Name:       "生产Key",
    Priority:   0,
    Enable:     true,
}, "sk-xxxxxxxxxxxx")

// 轮转 Key
apikey.RotateKey(keyID, "sk-new-key-xxxxxxxx")

// 故障冷却(429 时调用)
apikey.MarkCooldown(keyID, 5*time.Minute)
```

### 7.2 智能路由(故障转移)

```go
// 创建模型路由
apikey.CreateRoute(&apikey.ModelRoute{
    ModelAlias: "gpt-4",
    ProviderID: providerID,
    Priority:   0,
})

// 按优先级选择可用 Key(跳过 cooldown)
selection, _ := apikey.SelectKey("tenant-001", "gpt-4")
// selection.APIKey = 明文 key
// selection.Provider.BaseURL = 服务商地址
```

### 7.3 Token 计量

```go
// 记录一次 LLM 调用
usage.RecordUsage(&usage.UsageRecord{
    PrincipalID:    "agent-001",
    PrincipalKind:  1, // Agent
    TenantID:       "tenant-001",
    ProviderName:   "OpenAI",
    ModelName:      "gpt-4",
    PromptTokens:   1500,
    CompTokens:     800,
    Cost:           0.09,
    RequestID:      "req-xxx",
    Success:        true,
    LatencyMs:      2300,
})

// 查询用量汇总
summary, _ := usage.GetUsageSummary("tenant-001", "agent-001", startTime, endTime)
// summary.TotalTokens / TotalCost / RequestCount

// 按模型维度
byModel, _ := usage.GetUsageByModel("tenant-001", startTime, endTime)
// map["gpt-4"] = &UsageSummary{TotalTokens: 100000, TotalCost: 12.5}

// 检查预算
allowed, daily, monthly, _ := usage.CheckBudget("tenant-001", "agent-001", "gpt-4")
if !allowed {
    return errors.New("超出每日 token 配额")
}
```

### 7.4 AI 网关端点（OpenAI 兼容）

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/chat/completions` | POST | 聊天补全（流式 + 整包） |
| `/v1/embeddings` | POST | 向量嵌入 |
| `/v1/images/generations` | POST | 图像生成 |
| `/v1/audio/transcriptions` | POST | 语音转写（Whisper，multipart） |
| `/v1/audio/speech` | POST | 语音合成（TTS） |
| `/v1/moderations` | POST | 内容审核 |
| `/v1/models` | GET | 可用模型列表 |

### 7.5 网关增强能力

**多轮对话会话**（`session_id` 参数）：
```bash
curl -X POST /v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role":"user","content":"我叫张三"}],
    "session_id": "conv-001"
  }'
# 后续请求带相同 session_id，自动加载最近 20 条历史消息
```

**Prompt 模板自动注入**（`prompt_template_id` + `prompt_vars`）：
```bash
curl -X POST /v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role":"user","content":"写一首关于秋天的诗"}],
    "prompt_template_id": "tpl-001",
    "prompt_vars": {"style": "现代诗", "length": "8行"}
  }'
# 网关自动渲染模板并注入为 system message
```

**语义缓存**：非流式请求命中相同 prompt 时跳过 LLM 调用，响应头返回 `X-Cache: HIT`。默认 TTL 24h，最大 10000 条。

需要验证每次均为真实上游调用时，可传入 `"cache_bypass": true`。网关会在转发上游前移除该扩展字段，并同时跳过本次缓存读取与写入。

**内容审核**：请求体加 `"moderate": true`，输入不安全时拒绝（返回 400）。

**每用户限流**：令牌桶算法，默认 10 req/s burst 20。可通过 `ratelimit.SetBudgetRateLimit(principalID, rate, burst)` 调整。

### 7.6 网关管理 API（`/admin/api`）

| 端点 | 说明 |
|------|------|
| `GET /conversations` | 对话会话列表 |
| `GET /conversations/:id/messages` | 会话消息历史 |
| `GET /gateway-logs` | 网关请求日志（分页） |
| `GET /gateway-logs/stats` | 网关统计（token/cost/缓存率） |
| `GET /ai-cache/stats` | 语义缓存命中率 |
| `DELETE /ai-cache` | 清空缓存 |
| `GET /ratelimit/stats` | 限流桶状态 |
| `GET /prompts/:id/versions` | Prompt 版本历史 |
| `POST /prompts/:id/rollback` | 回滚 Prompt 版本 |

---

## 8. 安全加固

### 8.1 PII 字段加密

```go
auth.SetPIIKey([]byte("32-byte-pii-key-here!!!!!!!!!"))

// 加密存储
encrypted, _ := auth.EncryptPII("13800138000")

// 解密使用
plaintext, _ := auth.DecryptPII(encrypted)

// 脱敏展示
masked := auth.MaskPII("13800138000", "mobile")  // → 138****8000
masked := auth.MaskPII("test@example.com", "email") // → te***@example.com
masked := auth.MaskPII("110101199001011234", "idCard") // → 110101********1234
```

### 8.2 Session 管理

```go
// 登录时记录会话
session.RecordSession(&session.Session{
    PrincipalID: "user-001",
    TokenSig:    "token-signature",
    DeviceType:  0, // web
    IP:          "192.168.1.1",
    UserAgent:   "Chrome/120",
})

// 列出活跃会话
sessions, _ := session.ListSessions("user-001")

// 吊销单会话(可疑设备)
session.RevokeSession("session-xxx", "异地登录")

// 全设备登出
count, _ := session.RevokeAllByPrincipal("user-001", "", "all_devices_logout")

// 异地检测
anomaly, lastIP := session.DetectAnomaly("user-001", "10.0.0.1", 60)
if anomaly {
    // 触发告警/要求 MFA
}
```

### 8.3 Zero Trust 风险评分

```go
score := auth.CalculateRiskScore(&auth.RiskSignals{
    Principal:     p,
    CurrentIP:     "203.0.113.1",
    TokenIssuedIP: "192.168.1.1",
    TokenAge:      3 * time.Hour,
    NewDevice:     true,
    FailedAuths:   2,
})

switch score.Action {
case "allow":    // 正常放行
case "step_up":  // 要求 MFA/重新认证
    return errors.New("risk: step-up authentication required")
case "deny":     // 拒绝
    return errors.New("risk: access denied")
}
```

### 8.4 密钥轮换

```go
// 初始化密钥管理器
auth.InitKeyManager("current-secret-key")

// 轮换:旧密钥进入 grace period(24h 内仍可验签旧 token)
newKID := auth.RotateKey("new-secret-key")

// 获取 JWKS(暴露给客户端)
jwks := auth.GetJWKS("https://usercenter.example.com")
// {keys: [{kid: "abc123", kty: "oct", alg: "HS256"}]}
```

---

## 9. SCIM 2.0 用户供给

### 9.1 接入 HR 系统

SCIM 端点用于飞书/钉钉/Workday 等 HR 系统自动同步用户(入职建号/离职停号)。

```go
// main.go 注册 SCIM 路由
scim.RegisterSCIMRouter(r, "scim-bearer-token-xxx")
```

### 9.2 SCIM API

**用户端点**：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/scim/v2/Users?filter=userName eq "test"` | 查询用户(支持 filter) |
| POST | `/scim/v2/Users` | 创建用户 |
| GET | `/scim/v2/Users/:id` | 获取用户（含 groups[] 角色） |
| PUT | `/scim/v2/Users/:id` | 替换用户 |
| PATCH | `/scim/v2/Users/:id` | 增量更新(activate/deactivate) |
| DELETE | `/scim/v2/Users/:id` | 删除用户 |

**组端点**（Group ↔ Role 映射）：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/scim/v2/Groups?filter=displayName co "admin"` | 列出组(角色)，支持 filter |
| POST | `/scim/v2/Groups` | 创建组（内部创建 Role） |
| GET | `/scim/v2/Groups/:id` | 获取组 + members 列表 |
| PUT | `/scim/v2/Groups/:id` | 替换组 displayName |
| PATCH | `/scim/v2/Groups/:id` | 增删成员 / 替换成员 / 改名 |
| DELETE | `/scim/v2/Groups/:id` | 删除组（删除 Role） |

```bash
# SCIM 用户请求示例
curl -X POST http://localhost:48080/scim/v2/Users \
  -H "Authorization: Bearer scim-bearer-token-xxx" \
  -H "X-Tenant-ID: tenant-001" \
  -H "Content-Type: application/json" \
  -d '{
    "schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
    "userName": "newemployee",
    "active": true,
    "emails": [{"value": "new@company.com", "type": "work"}],
    "name": {"familyName": "张"}
  }'

# SCIM 组：添加成员（把用户加入角色）
curl -X PATCH http://localhost:48080/scim/v2/Groups/<role-id> \
  -H "Authorization: Bearer scim-bearer-token-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
    "Operations": [{"op": "add", "value": {"members": [{"value": "<user-id>"}]}}]
  }'
```

### 9.3 Filter 支持

```
userName eq "alice"
active eq true and userName co "test"
emails.value co "@company.com"
externalId eq "EMP001"
displayName co "admin"   # Groups
```

---

## 10. SDK 使用

### 10.1 Go SDK

```go
import "github.com/CloudSilk/usercenter/sdk/go"

client := usercenterclient.NewClient("http://localhost:48080")

// 登录
token, _ := client.Login("admin", "password")

// 创建用户
resp, _ := client.AddUser(&usercenterclient.User{
    UserName: "newuser",
    Nickname: "新用户",
    Enable:   true,
    Email:    "new@company.com",
})

// 查询用户
list, _ := client.QueryUsers(1, 10, map[string]string{"userName": "new"})

// 修改密码
client.ChangePassword("oldpass", "NewPass123")

// 健康检查
client.Health()
```

### 10.2 Python SDK

```python
from usercenter_client import UserCenterClient

client = UserCenterClient("http://localhost:48080")
client.login("admin", "password")
client.add_user({"userName": "newuser", "nickname": "新用户", "enable": True})
users = client.query_users(page_index=1, page_size=10, userName="new")
client.change_password("oldpass", "NewPass123")
```

### 10.3 TypeScript SDK

```typescript
import { UserCenterClient } from "@usercenter/sdk";

const client = new UserCenterClient("http://localhost:48080");
await client.login("admin", "password");
await client.addUser({ userName: "newuser", nickname: "新用户", enable: true });
const list = await client.queryUsers(1, 10, { userName: "new" });
await client.changePassword("oldpass", "NewPass123");
```

---

## 11. 运维指南

### 11.1 健康检查

```bash
curl http://localhost:48080/health
# {"status":"ok"}
```

### 11.2 优雅关闭

发送 SIGTERM/SIGINT,服务最多等待 5 秒处理完现有请求后退出。

### 11.3 CI 债务检查

```bash
bash scripts/debt-check.sh
# Gate 1: FromCurrentUser production calls = 0
# Gate 2: AuthenticatePrincipal returns Principal = 1
# Gate 3: EncodeTokenFromPrincipal exists = 1
# === ALL GATES PASSED ===
```

### 11.4 proto → OpenAPI 生成

```bash
# 安装 Buf
go install github.com/bufbuild/buf/cmd/buf@latest

# 生成(OpenAPI + Go + Triple)
buf generate

# 产物:
# docs/*.swagger.json  ← 16 个 proto 的 OpenAPI 文档
# proto/*.pb.go        ← Go 消息代码
# proto/*_triple.pb.go ← Dubbo Triple 代码
```

### 11.5 SDK 生成

```bash
# 从 OpenAPI 生成三语言 SDK
npx @openapitools/openapi-generator-cli generate -c sdk/openapitools.json

# 产物:
# sdk/go/          ← Go SDK
# sdk/python/      ← Python SDK
# sdk/typescript/  ← TypeScript SDK
```

### 11.6 Swagger 文档

```
# Debug 模式(debug=true)访问:
http://localhost:48080/swagger/usercenter/index.html
```

---

## 12. API 参考

### 12.1 REST API 路由总表

| 领域 | 前缀 | 端点 |
|------|------|------|
| 用户 | `/api/core/auth/user` | login/logout/profile/add/update/delete/query/all/detail/enable/resetpwd/changepwd/export/import |
| 角色 | `/api/core/auth/role` | add/update/delete/query/all/detail/export/import |
| 菜单 | `/api/core/auth/menu` | add/update/delete/query/detail/tree/export/import |
| API | `/api/core/auth/api` | add/update/delete/query/enable/all/detail/export/import |
| 租户 | `/api/core/auth/tenant` | add/update/delete/query/all/detail/copy/enable/export/import |
| 应用 | `/api/core/auth/app` | add/update/delete/query/all/detail |
| 项目 | `/api/core/auth/project` | add/update/delete/query/detail/all/export/import |
| 表单组件 | `/api/core/auth/form/component` | add/update/delete/query/detail |
| 系统配置 | `/api/core/system/config` | add/update/query/delete/detail |
| 字典 | `/api/core/dictionaries` | add/update/query/delete/detail |
| 多语言 | `/api/core/language` | add/update/query/delete/detail |
| 网站 | `/api/core/website` | add/update/query/delete/detail |
| 微信配置 | `/api/core/wechat/config` | add/update/query/delete/detail |
| 微信登录 | `/api/wechat` | mini/login · register/check · phone/bind · web/login · qrcode · notify |
| SCIM | `/scim/v2` | Users CRUD+PATCH · Groups · ServiceProviderConfig · Schemas |
| 系统 | `/` | health |

### 12.2 响应格式

所有 REST API 返回统一格式:

```json
{
  "code": 20000,
  "message": "success",
  "data": {}
}
```

错误码:

| 码 | 含义 |
|----|------|
| 20000 | 成功 |
| 40000 | 参数错误 |
| 40001 | 未授权 |
| 40002 | 资源不存在 |
| 41001 | 用户名或密码错误 |
| 41002 | 用户不存在 |
| 41003 | 无权限 |
| 41004 | Token 无效 |
| 41005 | Token 过期 |
| 41006 | 用户已禁用/锁定 |
| 50000 | 服务器错误 |

---

## 附录

### A. internal/ 包索引

| 包 | 职责 |
|----|------|
| `internal/principal` | 身份抽象(Human/Agent/Service) |
| `internal/store` | 共享 DB 访问层 |
| `internal/auth` | 认证(password/token/argon2/oidc/mfa/webauthn/pii/riskscore/keyrotation/scope) |
| `internal/auth/token` | JWT 签发/解码/缓存/Agent token |
| `internal/permission` | 授权(casbin/abac/api/menu/role/ratelimit/event) |
| `internal/user` | 用户档案 + CRUD + 登录 + PB |
| `internal/tenant` | 租户 + GORM scope |
| `internal/apikey` | AI Key 管理(AES-GCM + 轮转 + 路由) |
| `internal/usage` | Token 计量 + 预算 |
| `internal/audit` | 审计日志(PrincipalKind) |
| `internal/alert` | 安全告警 |
| `internal/session` | 会话管理 + 异地检测 |
| `internal/scim` | SCIM 2.0 用户供给 |
| `internal/systemconfig` | 系统配置 |
| 其他 8 个 | dictionaries/language/website/wechatconfig/project/app/formcomponent |

### B. 关键设计决策(ADR 摘要)

1. **AI 能力在领域包内**(不拆独立服务)
2. **Principal 抽象 Batch 1 前置**(Agent 独立签发,token 结构冻结)
3. **依赖瘦身**(砍多驱动,保留 Dubbo/WeChat/Nacos)
4. **Modular Monolith**(20 领域包,model 纯委托层)

---

## 13. AI 网关(OpenAI 兼容代理)

UserCenter 内置一个 **OpenAI 兼容的 AI 网关**：应用只认 UserCenter 一个端点 + 一张 UserCenter token，
Key 管理 / 智能路由 / 用量计量 / 配额 / Prompt 模板全部收口在此。

### 13.1 聊天补全(流式 + 整包)

```
POST /v1/chat/completions   # OpenAI 兼容
GET  /v1/models             # 可用模型别名
```

- 按 `model` 经 `ModelRoute` 选 Key（主从池 + 故障转移，跳过 cooldown）。
- `stream:true` 时 **SSE 逐块透传**（自动捕获 `stream_options.include_usage` 的用量）；否则整包转发。
- 上游 **429 自动冷却该 Key（5min）并重试下一路由**。
- 调用前校验租户/Agent **配额**（`UsageBudget`），超额返回 HTTP 429 并记 `ai_quota_exceeded` 审计。
- 调用后记录真实用量（prompt/completion token、延迟、所用 Key）。

```bash
curl -X POST http://localhost:48080/v1/chat/completions \
  -H "Authorization: Bearer <usercenter_token>" -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}'
```

### 13.2 Prompt 模板

```
POST   /admin/api/prompts           # 创建（content 含 {{变量}} 占位）
GET    /admin/api/prompts
PUT    /admin/api/prompts/:id
DELETE /admin/api/prompts/:id
POST   /admin/api/prompts/:id/render   # {"vars":{"k":"v"}} → 渲染结果
```

变量采用**白名单替换**（非 text/template，杜绝注入），未提供的变量替换为空。`variables` 留空时自动从内容扫描。

---

## 14. OIDC 身份提供者

UserCenter 可作为**标准 OIDC IdP**，第三方应用以 OAuth Client 接入做单点登录：

| 端点 | 说明 |
|------|------|
| `GET /.well-known/openid-configuration` | 发现文档 |
| `GET /.well-known/jwks.json` | JWKS（KeyManager 密钥标识） |
| `POST /oauth/authorize` | 用户 Bearer → 一次性 code，支持 **PKCE S256** |
| `POST /oauth/token` | `authorization_code` / `client_credentials` / `refresh_token` |
| `GET /oauth/userinfo` | 用户声明（sub/name/tenant_id/role_ids） |
| `POST /oauth/revoke` | 吊销 |

- `id_token` 用 KeyManager 活跃密钥（HS256）签发；`access_token` 复用 UserCenter JWT；`refresh_token` **轮换**（用即废）。
- OAuthClient.Secret 存 **bcrypt**，仅创建/轮转时返回明文一次。
- 客户端认证：Basic 或 Post。

### 14.1 客户端管理

```
POST   /admin/api/oauth-clients           # 创建（返回明文 secret 一次）
GET    /admin/api/oauth-clients
PUT    /admin/api/oauth-clients/:id
POST   /admin/api/oauth-clients/:id/rotate-secret
DELETE /admin/api/oauth-clients/:id
```

### 14.2 客户端凭证流程示例

```bash
curl -X POST http://localhost:48080/oauth/token \
  -u "<client_id>:<client_secret>" \
  -d "grant_type=client_credentials"
# → {"access_token":"...","token_type":"Bearer","expires_in":...}
```

### 14.3 社交登录画廊（GitHub/Google）

UserCenter 可聚合第三方社交登录。配置 `socialLogins` 后：

```yaml
socialLogins:
  - provider: github
    clientID: <gh-client-id>
    clientSecret: <gh-secret>
    redirectURI: http://host/api/oauth/github/callback
  - provider: google
    clientID: <g-client-id>
    clientSecret: <g-secret>
    redirectURI: http://host/api/oauth/google/callback
```

```
GET /api/oauth/:provider/login?redirect=<front>   跳转 provider 授权页（带 state CSRF）
GET /api/oauth/:provider/callback                  换 token→取 profile→匹配/创建用户→签发 token→回跳前端?social_token=
GET /api/social/providers                          已配置 provider 列表（公开，登录页用）
```

回调按 `(provider, sub) → email → 新建` 顺序匹配用户并绑定外部身份（`user_external_identity` 表）。
面板「安全中心」可查看/解绑外部身份；登录页显示已配置的社交登录按钮。

---

## 15. 可观测性与告警

### 15.1 Prometheus 指标

```
GET /metrics   # Prometheus 抓取端点（公开）
```

暴露：`usercenter_http_requests_total` / `usercenter_http_request_duration_seconds`（按 method/route/status）、
`usercenter_aigateway_requests_total` / `_tokens_total` / `_cost_usd_total` / `_quota_exceeded_total`、
`usercenter_users_total` / `_roles_total` / `_sessions_active` / `_tokens_today`。

### 15.2 审计实时流(SSE)

```
GET /admin/api/audit/stream?access_token=<jwt>
```

新产生的审计事件（用户/管理员/AI 网关操作）实时推送给订阅者。token 走 query（EventSource 不能带 Authorization 头）。

### 15.3 Webhook 告警与事件订阅

**告警通道（单 URL）**：配置 `alertWebhookURL` 后，关键告警事件（`ai_quota_exceeded`、`brute_force_login`）以 JSON POST 异步推送到该 URL，供 Slack/钉钉/飞书消费。失败仅记日志，不阻塞业务。

**事件订阅系统（多目标 + 签名）**：通过 `/admin/api/webhooks` 注册订阅，按事件类型（逗号分隔）匹配后推送到各自的 URL，并携带 HMAC-SHA256 签名头（`X-Signature-256`）供接收方验签。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/api/webhooks` | 列出订阅（租户隔离） |
| POST | `/admin/api/webhooks` | 创建订阅（url / events / secret） |
| PUT | `/admin/api/webhooks/:id` | 更新订阅 |
| DELETE | `/admin/api/webhooks/:id` | 删除订阅 |

当前内置触发事件：`user.created`、`user.deleted`、`role.updated`、`tenant.created`。

```bash
# 注册：当有用户创建/删除时，推送到自建回调
curl -X POST http://localhost:48080/admin/api/webhooks \
  -H "Authorization: Bearer <admin-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CRM 同步",
    "url": "https://crm.company.com/hooks/usercenter",
    "events": "user.created,user.deleted",
    "secret": "shared-hmac-secret"
  }'

# 接收方收到（带签名）：
# Headers: X-Event-Type: user.created
#          X-Signature-256: <hex(HMAC-SHA256(body, secret))>
# Body: {"event":"user.created","ts":1234567890,"payload":{"id":"...","userName":"alice","tenantID":"t1"}}
```

### 15.4 API Key 认证（外部服务）

外部服务/脚本可不依赖人类 JWT，使用长生命周期 API Key 认证。Key 以 `uc_` 前缀 + 32 位随机，SHA-256 哈希存储，明文仅在创建时返回一次。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/api/api-keys` | 列出 API Key（仅返回 prefix/hint） |
| POST | `/admin/api/api-keys` | 创建（**返回明文，仅一次**） |
| DELETE | `/admin/api/api-keys/:id` | 吊销 |

```bash
# 创建 API Key
curl -X POST http://localhost:48080/admin/api/api-keys \
  -H "Authorization: Bearer <admin-jwt>" \
  -d '{"name":"ci-runner","principalID":"svc-1","roles":"viewer"}'
# → {"code":20000,"data":{"id":"...","plaintext":"uc_a1b2c3..."}}

# 用 API Key 访问（X-API-Key 头，替代 Bearer JWT）
curl http://localhost:48080/api/core/auth/user/all \
  -H "X-API-Key: uc_a1b2c3..."
```

中间件 `AuthRequired` 先检查 `X-API-Key`（命中即签发 Service 主体），未命中则走 JWT。无效 API Key 不阻断，fallthrough 到 JWT。

### 15.5 密码过期与限流配置

**密码过期**：配置 `pwdExpiredDays`（0=永不过期）后，用户密码超过该天数未修改时登录返回 `code=41007`（密码已过期，请修改密码）。`User.PasswordUpdatedAt` 在创建/重置/修改密码时自动更新。

**REST API 限流配置**：每用户令牌桶限流默认 10 req/s、burst 20。管理员可按 principal 动态调整：

```bash
# 查看当前限流桶状态
GET /admin/api/ratelimit/stats

# 调整某用户的速率/突发
PUT /admin/api/ratelimit/<principalID>
{"ratePerSecond": 5, "burst": 10}
# → 立即生效（桶重置为满）
```

空闲超过 30 分钟的令牌桶由后台协程自动回收，避免瞬时 principal 内存泄漏。

---

## 16. 可视化管理后台

内嵌单页应用（React + TypeScript + Vite，`go:embed` 打包进二进制）：

```
http://<host>:48080/web/admin
```

页面内部通过 `/api/core/auth/user/login` 获取 Token 后访问受保护接口。

### 16.1 项目结构

```
web/admin-ui/          ← React 前端工程
├── src/               ← React + TypeScript 源码
│   └── main.tsx       ← 入口
├── public/            ← 静态资源（favicon 等）
├── index.html         ← Vite 入口 HTML
├── vite.config.ts     ← Vite 构建配置（含 API 代理）
├── tsconfig.json      ← TypeScript 配置
├── package.json       ← npm 依赖
└── components.json    ← shadcn/ui 组件配置
```

### 16.2 本地开发

启动后端 devserver（无需 Nacos/Dubbo，纯 HTTP 直连 MySQL）：

```bash
NO_PROXY=localhost,127.0.0.1 UC_PORT=48180 go run ./cmd/devserver/
# → http://localhost:48180/web/admin   admin / Admin@123456
```

另开终端启动前端 devserver（带 HMR，自动代理 API 到后端）：

```bash
cd web/admin-ui
npm install
npm run dev
# → http://localhost:5173   API 自动代理到 :48180
```

Vite 已配置 `/api`、`/admin/api`、`/v1`、`/oauth` 等前缀的自动代理
（见 `vite.config.ts` 的 `server.proxy`），前端开发时无需后端 CORS 配置。

代理目标通过环境变量覆盖：`UC_API_TARGET=http://10.0.0.1:48080 npm run dev`。

### 16.3 生产构建

```bash
cd web/admin-ui
npm install
npm run build
# → 输出目录：web/admin-ui/dist/
```

构建完成后重新编译 Go 二进制：

```bash
go build -o usercenter main.go
```

`//go:embed web/dist` 会将最新的 `dist/` 目录递归打包进二进制。
`/web/admin` 路由通过 `http.FS(web.DistFS)` 直接服务内嵌资源。

### 16.4 页面功能

目前有 **15 个页面**，按四组导航：

| 分组 | 页面 |
|------|------|
| 组织管理 | 仪表盘（自动刷新）、用户（批量操作）、角色、租户 |
| AI 能力 | AI 网关（流式测试器 + Prompt 模板）、AI Key 管理（一键实测）、用量统计（ECharts） |
| 安全审计 | 会话管理、审计日志、实时监控（SSE 大屏）、安全中心（风险评分 + MFA TOTP 注册） |
| 系统集成 | OAuth 应用、SCIM 配置、系统配置、API 测试 |

