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
