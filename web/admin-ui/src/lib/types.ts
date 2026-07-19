export interface User {
  id: string
  tenantID: string
  userName: string
  nickname: string
  mobile: string
  email: string
  title: string
  realName: string
  eid: string
  enable: boolean
  roleIDs: string[]
  createdAt?: string
}

export interface Role {
  id: string
  name: string
  code: string
  description: string
  canDel: boolean
  public?: boolean
  isMust?: boolean
  enable?: boolean
  tenantName?: string
  roleMenus?: RoleMenu[]
}

export interface RoleMenu {
  menuID: string
  funcs: string
  show?: boolean
}

export interface MenuInfo {
  id: string
  name: string
  parentID: string
  children?: MenuInfo[]
}

export interface Tenant {
  id: string
  name: string
  contact: string
  cellPhone: string
  staffSize: number
  enable: boolean
  userCount?: number
  roleCount?: number
}

export interface AIProvider {
  id: string
  tenantID: string
  name: string
  baseURL: string
  authType: string
  healthy: boolean
  description: string
}

export interface AIKey {
  id: string
  tenantID: string
  providerID: string
  name: string
  keyHint: string
  priority: number
  enable: boolean
  cooldownEnd: number
  last429: number
}

export interface ModelRoute {
  id: string
  modelAlias: string
  providerID: string
  priority: number
  enable: boolean
  description: string
}

export interface UsageRecord {
  id: string
  principalID: string
  principalKind: number
  providerID: string
  providerName: string
  model: string
  promptTokens: number
  completionTokens: number
  totalTokens: number
  cost: number
  success: boolean
  errorCode: string
  latencyMs: number
  createdAt: string
}

export interface UsageSummary {
  totalTokens: number
  totalCost: number
  requestCount: number
}

export interface Session {
  id: string
  principalID: string
  deviceName: string
  deviceType: number
  ip: string
  location: string
  userAgent: string
  revoked: boolean
  revokedReason: string
  lastActiveAt: number
}

export interface AuditLog {
  id: string
  userID: string
  userName: string
  principalKind: number
  action: string
  targetID: string
  ip: string
  detail: string
  createdAt: string
}

export interface PromptTemplate {
  id: string
  name: string
  category: string
  modelAlias: string
  content: string
  variables: string
  description: string
  enable: boolean
}

export interface OAuthClient {
  id: string
  name: string
  redirectURIs: string
  grantTypes: string
  scopes: string
  enable: boolean
}

export interface MFAFactor {
  id: string
  principalID: string
  type: string
  name: string
  enable: boolean
}

export interface ExternalIdentity {
  id: string
  provider: string
  providerUserID: string
  providerLogin: string
}

export interface SystemConfig {
  id: string
  key: string
  value: string
  description: string
}

export interface Stats {
  userCount: number
  roleCount: number
  sessionCount: number
  todayTokens: number
}

export interface AlertConfig {
  loginFailThreshold: number
  authFailThreshold: number
  windowMinutes: number
}

export interface DashboardStats extends Stats {
  providers: number
  audits: number
}

export interface SocialProvider {
  name: string
  clientID: string
  icon: string
  authURL: string
}

export interface UserProfile {
  user: User
  tenant: Tenant
  roles: Role[]
  funcCodes: string[]
  menus: MenuInfo[]
}

export interface ApiResponse<T = unknown> {
  code: number
  message?: string
  data?: T
  records?: T[]
  total?: number
  pages?: number
}
