# Usercenter 重新设计 — 架构决策记录 (ADR)

> 状态:Draft(待团队评审)
> 日期:2026-06-19
> 产出方式:4 专家 agent(IAM 安全 / AI-Native / DX / 架构)两轮交叉辩论收敛

---

## 1. 背景(Context)

现有 usercenter 是 Go 实现的单体库(~2 万行),核心痛点:

- **`model/` 上帝包**:6583 行,混合领域实体 + 业务逻辑 + 数据访问 + 基础设施(Casbin adapter 1059 行、token、audit、alert)
- **三套协议平行手写**:proto(Dubbo Triple)/ http(Gin)/ provider(Dubbo RPC),同一能力(如 User)在三处各写一遍
- **人类身份单一模型**:所有主体都是 `User`,无机器身份/API Key/Agent 概念
- **纯 RBAC(roleID→URL)**:无 ABAC、无数据范围、无 scope
- **租户隔离靠自觉**:`Login` 查询不带 `tenant_id`(`model/user.go:493`),跨租户越权基本靠前端传参

**AI 时代新需求**:LLM API Key 管理、Agent 身份、Token/成本计量、配额、委派、Prompt 权限——这些在传统 IAM 里没有,是重新设计的核心差异化。

---

## 2. 三个关键决策(分歧裁决)

### ADR-001:AI 能力归属 → **领域包内,不拆独立 Policy Plane 服务**

**Decision**:AI 能力(API Key 管理、用量计量、配额判定)作为 usercenter 的领域包(`internal/apikey`、`internal/usage`)实现,usercenter 承担**策略面(Policy Plane)的存储与判定**,不拆成独立服务。LLM 调用本身在业务服务/网关,usercenter 只是被查询的策略后端。

**Rationale**:
- 独立服务纯增攻击面(新网络端点 `/evaluate`、独立鉴权密钥、策略数据同步链路),违背"能不开网络边界就不开"原则
- 独立服务是单点(fail-closed 全站不可用 / fail-open 全站越权),且无法与 usercenter 做"改库+清缓存"的事务原子化,加剧权限回收延迟
- **最关键**:LLM 在网关、policy 在远端 usercenter,这条边界必然在 3 处留可利用缝隙(见下)

**⚠️ Policy 归属的安全红线(不可妥协)**:
> ABAC 的资源参数判定必须发生在能拿到 LLM 完整 tool call 的位置,**不能下沉到只有工具粒度的远端策略服务**。
>
> 缝隙证据:被 prompt injection 污染的 LLM 会生成 `read_order(id=<别人的订单>)`。usercenter 能判"能不能调 read_order",但**看不到那个越权 id**(参数在网关)。两半协调失败 = 越权。这条不可被以"性能/解耦"名义重新提出拆分。

**演进触发条件**(满足 ≥2 条才考虑拆独立服务,优先走网关插件而非独立 service):
1. Policy 决策 P99 延迟 > 同进程内存判定(0.1-0.5ms)的 3 倍,且串在主请求链路
2. 决策输入数据(配额计数/动态黑名单)写入频率 > 主库写入频率 10 倍
3. 决策服务弹性需求 > 主服务 2 倍(如大促鉴权 QPS 涨 10 倍但用户 CRUD 不涨)
4. 团队 > 8 人且 Policy Plane 与 CRUD 由不同小组负责(康威定律)
5. Policy 判定需独立合规/审计隔离

**演进路径**:接口先行(进程内 package)→ 网关插件(Envoy WASM/APISIX,零网络跳转)→ 独立 Policy Service(最后选项)。每一步是前一步超集,**代码不重写只换实现**。

---

### ADR-002:Principal 抽象时机 → **Batch 1 引入 interface,但冻结 token 结构 + Agent 独立签发**

**Decision**:
1. **Batch 1 就在 core 定义 `Principal` interface**(`Subject()/Tenant()/Roles()/Permissions()/Attrs()`),`CurrentUser` 实现它——这是加法,不破坏现有代码
2. **token claim 结构 Batch 1 冻结不改**:复用现有 `type` 字段(`token.go:43`,约定 0=user/1=agent),不碰 claim schema,**零旧 token 作废**
3. **Agent 身份走独立签发路径**(`EncodeAgentPrincipal`),claim 塞 `agentID`,DecodeToken 按 `type` 分流。人类 token 与 agent token 物理上是两种
4. **4 条 CI 退出门 Batch 1 就立**(见 §4),反永久债

**Rationale(为何 Principal 不能后置)**:
后置会让 4 个 P0 能力做不了或返工——
- NHI:Agent 没有 userName/wechatOpenID,token claims 无法区分人/Agent
- 计量:`UsageRecord.userID` 字段语义二义
- 配额:按 userID 限流 = 10 个 Agent 共享一人额度
- 委派:权限从平铺 roleID 变链式交集,鉴权逻辑重写

**Rationale(为何窗口期是活漏洞,不是技术债)**:
窗口期"机器身份借人类通道"可被利用的 3 个攻击,证据链:
| 攻击场景 | 代码证据 | 后果 |
|---------|---------|------|
| A. 静默越权放大 | `token.go:42`(roleIDs 全量进 claims)+ `auth.go:89`(旧链路只认 roleID 不认 scope) | Agent 继承用户全量角色,PoLP 结构性破坏 |
| B. 审计举证不能 | `audit.go:13`(actor 字段写死人类语义) | Agent 冒充人类,合规留痕失效,举证不能 |
| C. 吊销熔断失效 | `token_cache.go:81`(DelByUserID 只覆盖人类 key space) | Agent token 泄露后无秒级止血手段 |

**Rationale(为何 token 结构不动)**:
devx 代码取证纠正了两个夸大——
- `token.go:43` **已有 `type` 字段**,claim 结构能承载 principal kind,无需改 schema
- 全仓**单一鉴权入口**(`auth.go:80` `c.Set("User")`),渐进不会引入第二 key,悬空抽象不成立

故:Principal interface 前置(消除活漏洞),token schema 冻结(零旧 token 作废 + 零热迁移风险)。

---

### ADR-003:依赖瘦身 → **砍多驱动/重复职能,保留深度耦合**

**Decision**:
- **砍**:`gorm.io/driver/{sqlserver,postgres}`(全仓只在 `adapter.go:304` 一个 if 分支,无真实部署)、`jinzhu/copier`(1 处引用)、`patrickmn/go-cache`(4 处,统一到 Redis)、`swaggo/swag`(proto 单契约源生成 OpenAPI 后淘汰)
- **留**:`dubbo-go`(14 Provider 深度绑定)、`casbin`(RBAC 地基)、`gorm+mysql+sqlite`(生产+测试单一路径)、`silenceper/wechat`(access_token 管理是安全敏感基础设施,自写 ~200 行含限频风险,SDK 只 9 个轻量依赖)、`nacos`(配置+注册生产依赖)

**治理机制**(`dependency-policy.yaml` 双向约束,避免"渐进=不动"):
- 每条依赖带 `reason + owner + review-cycle`(quarterly/never)
- `banned` 清单命中即 CI 阻断
- 三道闸门:PR 时新增依赖比对白名单、每月 drift-scan @owner、引用计数 ≤2 且 ≥90 天自动开 issue 推动删除

---

## 3. 目标架构(Modular Monolith)

```
cmd/usercenter/           # main.go(组装)
internal/
  principal/              # ★ 身份抽象:Human/Agent/Service Principal interface
  auth/                   # 认证:OIDC/Passkey/MFA/token(JWT+JWKS+轮换)
  permission/             # 授权:RBAC+ABAC+DataScope+CAS(Tool 权限)
  tenant/                 # 租户:行级强制隔离 GORM scope
  user/                   # 人类用户档案
  apikey/                 # ★ AI Capability Key + Provider 路由(可远程化接口)
  usage/                  # ★ Token/成本计量中心(可远程化接口)
  audit/                  # 结构化全量审计(WORM/分区)
  session/                # 会话列表 + 单会话吊销 + 风险信号
adapters/                 # 协议薄封装(一套 service,三套适配)
  http/  dubbo/  grpc/
api/proto/                # ★ 单一契约源 → 生成 gRPC/REST/SDK/OpenAPI
sdk/                      # 多语言 SDK(Go/Python/TS,自动生成)
```

**架构原则**:
- 领域包对外只暴露 `Service` 接口,内部 model/repo 不导出
- 领域间只通过接口调用,禁止跨包 import 对方 struct(linter `depguard` 强制)
- 每个领域包 3 层:`domain`(struct+校验)→ `repo`(gorm)→ `service`(业务编排),协议层是 service 的薄封装
- 存储只留 postgres + sqlite(测试),砍 mysql/sqlserver

---

## 4. Batch 1 精确范围(地基,8 周)

| 周次 | 交付 | 退出标志 |
|------|------|---------|
| W1 | 阶段0:抽 `internal/core`,领域逻辑原样搬入,旧 Provider/HTTP delegate 到 core,签名零变更 | 旧接口全绿,Principal 未引入 |
| W2-3 | 阶段1:引入 `Principal` interface + 单向 `PrincipalAdapter` + 影子双跑(新 Agent token 走 `EncodeAgentPrincipal` 独立签发,人类 token 不动) | 影子 7 天,不一致率 <0.01% |
| W4-5 | 阶段2:14 Provider 逐个迁移,CI 监控 `PrincipalAdapter` 引用单调下降 | adapter 引用计数下降趋势 |
| W6-8 | 阶段3:删 `PrincipalAdapter`,债清 | 见下方 4 条 CI 闸门 |

**4 条 CI 退出门(`debt-check` job,任一红阻断合并 main)**:
1. `grep -rnE 'PrincipalFromUser\|PrincipalAdapter\|fromLegacyUser' --include=*.go | grep -v _test.go` == 0
2. `model.Authenticate` 返回类型中 `*apipb.CurrentUser` 出现 == 0(返回 Principal)
3. `token.EncodeToken` 中读取旧 struct 字段的直接访问 == 0(走 Principal.Encode)
4. metric `usercenter_principal_shadow_mismatch_total` 连续 14 天 == 0

**反拖延机制**:阶段 1→3 硬上限 6 周,超期触发架构 review。adapter 单向数据流(只读旧模型产 Principal,禁止反向),每领域最多 1 个 adapter 实例。

---

## 5. 完整能力清单(24 项,4 Batch)

### Batch 1 — 地基(P0)
1. `Principal` 抽象(Human/Agent/Service)+ Agent 独立签发
2. 租户 GORM 行级强制 scope(修复 `user.go:493` 不带 tenant_id 的越权)
3. proto 单契约源 + 三协议适配(消除三处手写重复)
4. 领域包拆分(model god-package → internal/*)
5. 结构化全量审计(认证/授权/管理全覆盖)

### Batch 2 — AI 差异化(P0/P1)
6. Access+Refresh 双 Token + 标准 OIDC 端点
7. RBAC + ABAC + DataScope(替换纯 roleID→URL)
8. AI Capability Key 管理(加密存储 + 轮转 + 故障转移池)
9. Token/成本计量(`UsageRecord` 实时聚合,含 principalType 旁注消歧)
10. Agent 细粒度 Scope + 用户委派 consent(`act` claim)
11. MFA + Step-up + WebAuthn/Passkey

### Batch 3 — 安全深化(P1)
12. Argon2id 哈希 + 透明 rehash
13. 账号+IP+设备 三维防爆破(联动拦截)
14. Session 列表 + 单会话吊销 + 异地提醒
15. Zero Trust 连续验证(风险分 + IP 漂移)
16. 密钥轮换(KMS/Vault + JWKS + kid)
17. PII 最小化(身份证/手机字段级加密 + 脱敏)

### Batch 4 — 生态(P1/P2)
18. SCIM 2.0 自动供给
19. 多语言 SDK 自动生成(Buf)
20. Webhook/Event(进程内 + outbox)
21. 依赖瘦身 + `dependency-policy.yaml`
22. Config as Code(policies.yaml + `userctl apply`)

---

## 6. 迁移路径(渐进,不炸)

1. **core 纯库先行**(阶段0):新 `internal/*` 领域包,零框架依赖
2. **适配层包裹旧接口**:Dubbo Provider / Gin handler 改 thin wrapper 调 core,旧签名冻结
3. **影子模式**:新鉴权与旧版并行,对比 decision,差异告警(不阻断)
4. **数据迁移**:`userctl migrate`(Casbin rule → 新策略表,User → Principal)
5. **切流**:`/v2/` 新 API,`/v1/` 保留 deprecated

---

## 附录:团队辩论纪要

**收敛的三个分歧**:
1. AI 能力归属:ai-native 认输(领域包),iam 安全加固(3 处边界缝隙),architect 演进路径 → **领域包内**
2. Principal 时机:ai-native(4 能力返工)+ iam(3 攻击场景)主张 Batch 1;devx 代码取证纠正 2 个夸大(token 已有 type 字段、单一鉴权入口),给出折中 → **Batch 1 interface + 冻结 token + Agent 独立签发**
3. 依赖瘦身:architect 诚实改判 wechat 保留(核实 SDK 只 9 轻依赖),devx 双向治理机制 → **砍多驱动,保留深度耦合**

**未纳入本次范围**(留待后续):微服务拆分、ES 审计检索、多数据中心、DDD 全家桶。
