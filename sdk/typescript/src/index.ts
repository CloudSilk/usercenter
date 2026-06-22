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

  // Health
  async health(): Promise<boolean> {
    try {
      const resp = await fetch(`${this.baseURL}/health`, { method: "GET", signal: AbortSignal.timeout(5000) });
      return resp.ok;
    } catch {
      return false;
    }
  }
}
