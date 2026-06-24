// Package usercenterclient 是 usercenter 的 Go SDK
// 从现有 REST API 手写(REDESIGN #19)
// 后续可由 openapi-generator 从 OpenAPI 自动生成替代
package usercenterclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client usercenter REST 客户端
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string // JWT token(Authorization: Bearer xxx)
}

// NewClient 创建客户端
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetToken 设置 JWT token
func (c *Client) SetToken(token string) {
	c.Token = token
}

// CommonResponse 通用响应
type CommonResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
}

// User 用户信息
type User struct {
	ID            string   `json:"id" `
	TenantID      string   `json:"tenantID"`
	ProjectID     string   `json:"projectID"`
	UserName      string   `json:"userName" validate:"required"`
	Nickname      string   `json:"nickname" validate:"required"`
	UserRoles     []Role   `json:"userRoles"`
	RoleIDs       []string `json:"roleIDs"`
	Enable        bool     `json:"enable"`
	Email         string   `json:"email"`
	Mobile        string   `json:"mobile"`
	IDCard        string   `json:"idCard"`
	Avatar        string   `json:"avatar"`
	Title         string   `json:"title"`
	RealName      string   `json:"realName"`
	Type          int32    `json:"type"`
	Group         string   `json:"group"`
	IsMust        bool     `json:"isMust"`
}

// Role 角色
type Role struct {
	ID            string `json:"id"`
	TenantID      string `json:"tenantID"`
	Name          string `json:"name"`
	ParentID      string `json:"parentID"`
	DefaultRouter string `json:"defaultRouter"`
	Description   string `json:"description"`
	CanDel        bool   `json:"canDel"`
	Public        bool   `json:"public"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"` // JWT token
}

// --- 通用请求方法 ---

func (c *Client) do(method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w (body=%s)", err, string(respBody))
		}
	}
	return nil
}

// --- Auth API ---

// Login 用户名密码登录
func (c *Client) Login(userName, password string) (*LoginResponse, error) {
	result := &LoginResponse{}
	err := c.do("POST", "/api/core/auth/user/login", &LoginRequest{
		UserName: userName, Password: password,
	}, result)
	if err != nil {
		return nil, err
	}
	if result.Data != "" {
		c.SetToken(result.Data)
	}
	return result, nil
}

// Logout 登出
func (c *Client) Logout() error {
	return c.do("POST", "/api/core/auth/user/logout", nil, nil)
}

// GetProfile 获取个人信息
func (c *Client) GetProfile() (*User, error) {
	result := struct {
		Code int32  `json:"code"`
		Data *User  `json:"data"`
	}{}
	err := c.do("GET", "/api/core/auth/user/profile", nil, &result)
	return result.Data, err
}

// --- User CRUD ---

// AddUser 创建用户
func (c *Client) AddUser(u *User) (*CommonResponse, error) {
	result := &CommonResponse{}
	err := c.do("POST", "/api/core/auth/user/add", u, result)
	return result, err
}

// UpdateUser 更新用户
func (c *Client) UpdateUser(u *User) (*CommonResponse, error) {
	result := &CommonResponse{}
	err := c.do("PUT", "/api/core/auth/user/update", u, result)
	return result, err
}

// DeleteUser 删除用户
func (c *Client) DeleteUser(id string) (*CommonResponse, error) {
	result := &CommonResponse{}
	err := c.do("DELETE", "/api/core/auth/user/delete", map[string]string{"id": id}, result)
	return result, err
}

// QueryUsers 分页查询用户
func (c *Client) QueryUsers(pageIndex, pageSize int, filters map[string]string) (*UserListResponse, error) {
	params := url.Values{}
	params.Set("pageIndex", fmt.Sprintf("%d", pageIndex))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))
	for k, v := range filters {
		params.Set(k, v)
	}
	result := &UserListResponse{}
	err := c.do("GET", "/api/core/auth/user/query?"+params.Encode(), nil, result)
	return result, err
}

// UserListResponse 用户列表响应
type UserListResponse struct {
	Code    int32   `json:"code"`
	Data    []*User `json:"data"`
	Records int64   `json:"records"`
	Pages   int64   `json:"pages"`
	Total   int64   `json:"total"`
}

// --- Role CRUD ---

// AddRole 创建角色
func (c *Client) AddRole(r *Role) (*CommonResponse, error) {
	result := &CommonResponse{}
	err := c.do("POST", "/api/core/auth/role/add", r, result)
	return result, err
}

// GetAllRoles 获取所有角色
func (c *Client) GetAllRoles() (*RoleListResponse, error) {
	result := &RoleListResponse{}
	err := c.do("GET", "/api/core/auth/role/all", nil, result)
	return result, err
}

// RoleListResponse 角色列表响应
type RoleListResponse struct {
	Code    int32   `json:"code"`
	Data    []*Role `json:"data"`
	Records int64   `json:"records"`
}

// --- Password ---

// ChangePassword 修改密码
func (c *Client) ChangePassword(oldPwd, newPwd string) (*CommonResponse, error) {
	result := &CommonResponse{}
	err := c.do("POST", "/api/core/auth/user/changepwd", map[string]string{
		"oldPwd": oldPwd, "newPwd": newPwd, "newConfirmPwd": newPwd,
	}, result)
	return result, err
}

// ResetPassword 重置用户密码(管理员)
func (c *Client) ResetPassword(userID string) (*CommonResponse, error) {
	result := &CommonResponse{}
	err := c.do("POST", "/api/core/auth/user/resetpwd", map[string]string{"id": userID}, result)
	return result, err
}

// --- Health ---

// Health 健康检查
func (c *Client) Health() error {
	return c.do("GET", "/health", nil, nil)
}

// --- AI Gateway (OpenAI-compatible) ---
//
// 与 do() 不同,这些方法返回原始响应体([]byte),不解析 JSON,
// 让调用方自行处理(尤其 stream / 非标准错误格式)。

// doRaw 发送请求并返回原始响应体。返回未经解析的 bytes。
func (c *Client) doRaw(method, path string, body interface{}, contentType string) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		switch v := body.(type) {
		case []byte:
			bodyReader = bytes.NewReader(v)
		case io.Reader:
			bodyReader = v
		default:
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshal body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonBody)
		}
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return respBody, nil
}

// ChatCompletion 对话补全。POST /v1/chat/completions
// messages 形如 [{"role":"user","content":"hello"}]。
// opts 透传可选字段:stream / session_id / prompt_template_id / prompt_vars / moderate / moderate_output。
func (c *Client) ChatCompletion(model string, messages []map[string]string, opts map[string]any) ([]byte, error) {
	payload := map[string]any{
		"model":    model,
		"messages": messages,
	}
	for k, v := range opts {
		payload[k] = v
	}
	return c.doRaw("POST", "/v1/chat/completions", payload, "application/json")
}

// Embeddings 文本向量化。POST /v1/embeddings
func (c *Client) Embeddings(model string, input string) ([]byte, error) {
	return c.doRaw("POST", "/v1/embeddings", map[string]string{
		"model": model, "input": input,
	}, "application/json")
}

// ImageGeneration 图像生成。POST /v1/images/generations
func (c *Client) ImageGeneration(model string, prompt string) ([]byte, error) {
	return c.doRaw("POST", "/v1/images/generations", map[string]string{
		"model": model, "prompt": prompt,
	}, "application/json")
}

// AudioTranscription 语音转文字(multipart)。POST /v1/audio/transcriptions
// filePath 为本地音频文件路径。
func (c *Client) AudioTranscription(model string, filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// multipart/form-data 必须手工构造 body 并设置 Content-Type(含 boundary),
	// 因此直接走 http.NewRequest 而非复用 doRaw 的 JSON 分支。
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("model", model); err != nil {
		return nil, fmt.Errorf("write model field: %w", err)
	}
	part, err := w.CreateFormFile("file", filePath)
	if err != nil {
		return nil, fmt.Errorf("create file field: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copy file content: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	// w.FormDataContentType() 含正确的 boundary,交给 doRaw 使用。
	return c.doRaw("POST", "/v1/audio/transcriptions", &buf, w.FormDataContentType())
}

// AudioSpeech 文本转语音。POST /v1/audio/speech
func (c *Client) AudioSpeech(model string, input string, voice string) ([]byte, error) {
	return c.doRaw("POST", "/v1/audio/speech", map[string]string{
		"model": model, "input": input, "voice": voice,
	}, "application/json")
}

// Moderation 内容审核。POST /v1/moderations
func (c *Client) Moderation(model string, input string) ([]byte, error) {
	return c.doRaw("POST", "/v1/moderations", map[string]string{
		"model": model, "input": input,
	}, "application/json")
}

// ListModels 列出可用模型。GET /v1/models
func (c *Client) ListModels() ([]byte, error) {
	return c.doRaw("GET", "/v1/models", nil, "")
}
