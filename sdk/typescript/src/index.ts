/**
 * UserCenter TypeScript SDK
 * 从 REST API 生成(REDESIGN #19)
 */

export interface CommonResponse {
  code: number;
  message: string;
}

export interface User {
  id?: string;
  tenantID?: string;
  projectID?: string;
  userName: string;
  nickname: string;
  userRoles?: Role[];
  roleIDs?: string[];
  enable?: boolean;
  email?: string;
  mobile?: string;
  idCard?: string;
  avatar?: string;
  title?: string;
  realName?: string;
  type?: number;
  group?: string;
  isMust?: boolean;
}

export interface Role {
  id?: string;
  tenantID?: string;
  name: string;
  parentID?: string;
  defaultRouter?: string;
  description?: string;
  canDel?: boolean;
  public?: boolean;
  isMust?: boolean;
  enable?: boolean;
}


export interface Menu {
  id?: string;
  tenantID?: string;
  name: string;
  parentID?: string;
  path?: string;
  icon?: string;
  sort?: number;
  enable?: boolean;
}

export interface Tenant {
  id?: string;
  name: string;
  contact?: string;
  cellPhone?: string;
  address?: string;
  enable?: boolean;
  province?: string;
  city?: string;
  area?: string;
  userCount?: number;
  roleCount?: number;
}

export interface APIInfo {
  id?: string;
  tenantID?: string;
  name: string;
  path: string;
  method: string;
  group?: string;
  enable?: boolean;
}

export interface MenuListResponse extends CommonResponse {
  data?: Menu[];
  records?: number;
  pages?: number;
  total?: number;
}

export interface TenantListResponse extends CommonResponse {
  data?: Tenant[];
  records?: number;
}

export interface RoleListResponse extends CommonResponse {
  data?: Role[];
  records?: number;
  pages?: number;
  total?: number;
}

export interface RoleDetailResponse extends CommonResponse {
  data?: Role;
}

export interface APIListResponse extends CommonResponse {
  data?: APIInfo[];
  records?: number;
  pages?: number;
  total?: number;
}

export interface MenuDetailResponse extends CommonResponse {
  data?: Menu;
}
export interface LoginResponse extends CommonResponse {
  data?: string; // JWT token
}

export interface UserListResponse extends CommonResponse {
  data?: User[];
  records?: number;
  pages?: number;
  total?: number;
}

export class UserCenterClient {
  private baseURL: string;
  private token: string;

  constructor(baseURL: string, token?: string) {
    this.baseURL = baseURL.replace(/\/+$/, "");
    this.token = token || "";
  }

  setToken(token: string): void {
    this.token = token;
  }

  private async request<T>(method: string, path: string, body?: any): Promise<T> {
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (this.token) headers["Authorization"] = `Bearer ${this.token}`;
    const resp = await fetch(`${this.baseURL}${path}`, {
      method, headers, body: body ? JSON.stringify(body) : undefined,
    });
    return resp.json() as Promise<T>;
  }

  /**
   * 与 request 相同,但返回原始响应体(ArrayBuffer),不解析 JSON。
   * 供 AI gateway 端点使用 —— 对二进制响应(如 audio/speech)和 stream 都安全,
   * 调用方自行决定如何解码。
   */
  private async requestRaw(method: string, path: string, body?: BodyInit, headers?: Record<string, string>): Promise<ArrayBuffer> {
    const finalHeaders: Record<string, string> = { ...headers };
    if (!(body instanceof FormData) && !finalHeaders["Content-Type"]) {
      finalHeaders["Content-Type"] = "application/json";
    }
    if (this.token) finalHeaders["Authorization"] = `Bearer ${this.token}`;
    const resp = await fetch(`${this.baseURL}${path}`, {
      method, headers: finalHeaders, body,
    });
    return resp.arrayBuffer();
  }

  // Auth
  async login(userName: string, password: string): Promise<LoginResponse> {
    const resp = await this.request<LoginResponse>("POST", "/api/core/auth/user/login", { userName, password });
    if (resp.data) this.setToken(resp.data);
    return resp;
  }

  async logout(): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/user/logout");
  }

  async getProfile(): Promise<User> {
    const resp = await this.request<{ code: number; data: User }>("GET", "/api/core/auth/user/profile");
    return resp.data;
  }

  // User CRUD
  async addUser(user: User): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/user/add", user);
  }

  async updateUser(user: User): Promise<CommonResponse> {
    return this.request("PUT", "/api/core/auth/user/update", user);
  }

  async deleteUser(id: string): Promise<CommonResponse> {
    return this.request("DELETE", "/api/core/auth/user/delete", { id });
  }

  async queryUsers(pageIndex = 1, pageSize = 10, filters?: Record<string, string>): Promise<UserListResponse> {
    const params = new URLSearchParams({ pageIndex: String(pageIndex), pageSize: String(pageSize), ...filters });
    return this.request("GET", `/api/core/auth/user/query?${params}`);
  }

  // Password
  async changePassword(oldPwd: string, newPwd: string): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/user/changepwd", { oldPwd, newPwd, newConfirmPwd: newPwd });
  }

  async resetPassword(userId: string): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/user/resetpwd", { id: userId });
  }

  // Role
  async addRole(role: Role): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/role/add", role);
  }

  async getAllRoles(): Promise<{ code: number; data: Role[] }> {
    return this.request("GET", "/api/core/auth/role/all");
  }


  // Menu CRUD
  async addMenu(menu: Menu): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/menu/add", menu);
  }

  async updateMenu(menu: Menu): Promise<CommonResponse> {
    return this.request("PUT", "/api/core/auth/menu/update", menu);
  }

  async deleteMenu(id: string): Promise<CommonResponse> {
    return this.request("DELETE", "/api/core/auth/menu/delete", { id });
  }

  async queryMenus(pageIndex = 1, pageSize = 10, filters?: Record<string, string>): Promise<MenuListResponse> {
    const params = new URLSearchParams({ pageIndex: String(pageIndex), pageSize: String(pageSize), ...filters });
    return this.request("GET", "/api/core/auth/menu/query?" + params);
  }

  async getMenuDetail(id: string, tenantID?: string): Promise<MenuDetailResponse> {
    const params = new URLSearchParams({ id });
    if (tenantID) params.set("tenantID", tenantID);
    return this.request("GET", "/api/core/auth/menu/detail?" + params);
  }

  async getMenuTree(): Promise<MenuListResponse> {
    return this.request("GET", "/api/core/auth/menu/tree");
  }

  // Tenant CRUD
  async addTenant(tenant: Tenant): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/tenant/add", tenant);
  }

  async updateTenant(tenant: Tenant): Promise<CommonResponse> {
    return this.request("PUT", "/api/core/auth/tenant/update", tenant);
  }

  async deleteTenant(id: string): Promise<CommonResponse> {
    return this.request("DELETE", "/api/core/auth/tenant/delete", { id });
  }

  async queryTenants(pageIndex = 1, pageSize = 10, filters?: Record<string, string>): Promise<TenantListResponse> {
    const params = new URLSearchParams({ pageIndex: String(pageIndex), pageSize: String(pageSize), ...filters });
    return this.request("GET", "/api/core/auth/tenant/query?" + params);
  }

  async getAllTenants(): Promise<TenantListResponse> {
    return this.request("GET", "/api/core/auth/tenant/all");
  }

  async getTenantDetail(id: string): Promise<{ code: number; data: Tenant }> {
    return this.request("GET", "/api/core/auth/tenant/detail?id=" + id);
  }

  async enableTenant(id: string, enable: boolean): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/tenant/enable", { id, enable });
  }

  // Role CRUD
  async updateRole(role: Role): Promise<CommonResponse> {
    return this.request("PUT", "/api/core/auth/role/update", role);
  }

  async deleteRole(id: string): Promise<CommonResponse> {
    return this.request("DELETE", "/api/core/auth/role/delete", { id });
  }

  async enableRole(id: string, enable: boolean): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/role/enable", { id, enable });
  }

  async queryRoles(pageIndex = 1, pageSize = 10, filters?: Record<string, string>): Promise<RoleListResponse> {
    const params = new URLSearchParams({ pageIndex: String(pageIndex), pageSize: String(pageSize), ...filters });
    return this.request("GET", "/api/core/auth/role/query?" + params);
  }

  async getRoleDetail(id: string, tenantID?: string): Promise<RoleDetailResponse> {
    const params = new URLSearchParams({ id });
    if (tenantID) params.set("tenantID", tenantID);
    return this.request("GET", "/api/core/auth/role/detail?" + params);
  }

  // API CRUD
  async addApi(api: APIInfo): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/api/add", api);
  }

  async updateApi(api: APIInfo): Promise<CommonResponse> {
    return this.request("PUT", "/api/core/auth/api/update", api);
  }

  async deleteApi(id: string): Promise<CommonResponse> {
    return this.request("DELETE", "/api/core/auth/api/delete", { id });
  }

  async queryApis(pageIndex = 1, pageSize = 10, filters?: Record<string, string>): Promise<APIListResponse> {
    const params = new URLSearchParams({ pageIndex: String(pageIndex), pageSize: String(pageSize), ...filters });
    return this.request("GET", "/api/core/auth/api/query?" + params);
  }

  async getAllApis(): Promise<APIListResponse> {
    return this.request("GET", "/api/core/auth/api/all");
  }

  async getApiDetail(id: string): Promise<{ code: number; data: APIInfo }> {
    return this.request("GET", "/api/core/auth/api/detail?id=" + id);
  }

  async enableApi(id: string, enable: boolean): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/api/enable", { id, enable });
  }

  // User extras
  async enableUser(id: string, enable: boolean): Promise<CommonResponse> {
    return this.request("POST", "/api/core/auth/user/enable", { id, enable });
  }

  async getAllUsers(filters?: Record<string, string>): Promise<UserListResponse> {
    const params = new URLSearchParams(filters || {});
    return this.request("GET", "/api/core/auth/user/all?" + params);
  }

  async getUserDetail(id: string): Promise<{ code: number; data: User }> {
    return this.request("GET", "/api/core/auth/user/detail?id=" + id);
  }

  async updateProfile(profile: Partial<User>): Promise<CommonResponse> {
    return this.request("PUT", "/api/core/auth/user/profile", profile);
  }

  // OIDC / OAuth2
  async oidcDiscovery(): Promise<any> {
    return this.request("GET", "/.well-known/openid-configuration");
  }

  async jwks(): Promise<any> {
    return this.request("GET", "/.well-known/jwks");
  }

  async oauthToken(code: string, redirectUri: string, clientId: string, clientSecret: string): Promise<any> {
    const params = new URLSearchParams({ grant_type: "authorization_code", code, redirect_uri: redirectUri, client_id: clientId, client_secret: clientSecret });
    return this.requestRaw("POST", "/oauth/token?" + params, undefined, {});
  }

  async oauthUserinfo(): Promise<any> {
    return this.request("GET", "/oauth/userinfo");
  }

  async oauthRevoke(token: string): Promise<CommonResponse> {
    return this.request("POST", "/oauth/revoke", { token });
  }

  // Health
  // Health
  async health(): Promise<boolean> {
    try {
      const resp = await fetch(`${this.baseURL}/health`, { method: "GET", signal: AbortSignal.timeout(5000) });
      return resp.ok;
    } catch {
      return false;
    }
  }

  // --- AI Gateway (OpenAI-compatible) ---
  // 这些方法返回原始响应体(ArrayBuffer),不解析 JSON,让调用方自行处理。

  /**
   * 对话补全。POST /v1/chat/completions
   * @param messages 形如 [{role:"user",content:"hello"}]
   * @param opts 可选字段:stream / session_id / prompt_template_id /
   *   prompt_vars / moderate / moderate_output
   */
  async chatCompletion(
    model: string,
    messages: Array<{ role: string; content: string }>,
    opts?: Record<string, any>,
  ): Promise<ArrayBuffer> {
    const body = { model, messages, ...opts };
    return this.requestRaw("POST", "/v1/chat/completions", JSON.stringify(body));
  }

  /** 文本向量化。POST /v1/embeddings */
  async embeddings(model: string, input: string): Promise<ArrayBuffer> {
    return this.requestRaw("POST", "/v1/embeddings", JSON.stringify({ model, input }));
  }

  /** 图像生成。POST /v1/images/generations */
  async imageGeneration(model: string, prompt: string): Promise<ArrayBuffer> {
    return this.requestRaw("POST", "/v1/images/generations", JSON.stringify({ model, prompt }));
  }

  /**
   * 语音转文字(multipart)。POST /v1/audio/transcriptions
   * @param file Blob/File 对象(Node 18+ 或浏览器均可提供)
   */
  async audioTranscription(model: string, file: Blob): Promise<ArrayBuffer> {
    const form = new FormData();
    form.append("model", model);
    form.append("file", file);
    // FormData 让 fetch 自动设置 multipart/form-data; Content-Type 头由浏览器/fetch 生成。
    return this.requestRaw("POST", "/v1/audio/transcriptions", form);
  }

  /** 文本转语音。POST /v1/audio/speech */
  async audioSpeech(model: string, input: string, voice: string): Promise<ArrayBuffer> {
    return this.requestRaw("POST", "/v1/audio/speech", JSON.stringify({ model, input, voice }));
  }

  /** 内容审核。POST /v1/moderations */
  async moderation(model: string, input: string): Promise<ArrayBuffer> {
    return this.requestRaw("POST", "/v1/moderations", JSON.stringify({ model, input }));
  }

  /** 列出可用模型。GET /v1/models */
  async listModels(): Promise<ArrayBuffer> {
    return this.requestRaw("GET", "/v1/models");
  }
}
