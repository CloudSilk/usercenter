package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// PhoneCodeSender is implemented by an embedding application's SMS provider.
type PhoneCodeSender interface {
	SendPhoneCode(context.Context, string, string, int) error
}

// PhoneUserReadyHook prepares application-specific user resources before the
// native usercenter access token is returned.
type PhoneUserReadyHook func(context.Context, string, bool) error

type PhoneAuthConfig struct {
	Enabled          bool
	TenantID         string
	HashKey          string
	CodeTTLSeconds   int
	CooldownSeconds  int
	MaxAttempts      int
	PhoneHourlyLimit int
	IPHourlyLimit    int
	ExposeDebugCode  bool
	Sender           PhoneCodeSender
	UserReady        PhoneUserReadyHook
}

// ConfigurePhoneAuth installs the reusable SMS login configuration.
func ConfigurePhoneAuth(cfg PhoneAuthConfig) error {
	var send user.PhoneCodeSender
	if cfg.Sender != nil {
		send = cfg.Sender.SendPhoneCode
	}
	return user.ConfigurePhoneAuth(user.PhoneAuthConfig{
		Enabled:          cfg.Enabled,
		TenantID:         cfg.TenantID,
		HashKey:          cfg.HashKey,
		CodeTTLSeconds:   cfg.CodeTTLSeconds,
		CooldownSeconds:  cfg.CooldownSeconds,
		MaxAttempts:      cfg.MaxAttempts,
		PhoneHourlyLimit: cfg.PhoneHourlyLimit,
		IPHourlyLimit:    cfg.IPHourlyLimit,
		ExposeDebugCode:  cfg.ExposeDebugCode,
		SendCode:         send,
		UserReady:        user.PhoneUserReadyHook(cfg.UserReady),
	})
}

type phoneCodeRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type phoneLoginRequest struct {
	Phone      string `json:"phone" binding:"required"`
	Code       string `json:"code" binding:"required"`
	DeviceType *int32 `json:"deviceType"`
	DeviceName string `json:"deviceName"`
}

type phoneResponse struct {
	Code    apipb.Code `json:"code"`
	Message string     `json:"message,omitempty"`
	Data    any        `json:"data,omitempty"`
}

// RegisterPublicPhoneAuthRouter registers endpoints that must be mounted
// before an embedding application's global authentication middleware.
func RegisterPublicPhoneAuthRouter(r *gin.Engine) {
	group := r.Group("/api/core/auth/user/phone")
	group.GET("status", phoneAuthStatus)
	group.POST("code", sendPhoneAuthCode)
	group.POST("login", phoneAuthLogin)
}

func phoneAuthStatus(c *gin.Context) {
	c.JSON(http.StatusOK, phoneResponse{
		Code: apipb.Code_Success,
		Data: gin.H{"enabled": user.PhoneAuthEnabled()},
	})
}

func sendPhoneAuthCode(c *gin.Context) {
	var req phoneCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code_BadRequest, Message: "手机号不能为空"})
		return
	}
	result, err := user.IssuePhoneCode(
		c.Request.Context(), req.Phone, c.ClientIP(), phoneRequestID(c),
	)
	if err != nil {
		writePhoneAuthError(c, err)
		return
	}
	data := gin.H{
		"phone_masked": result.PhoneMasked,
		"expires_in":   result.ExpiresIn,
		"retry_after":  result.RetryAfter,
	}
	if result.DebugCode != "" {
		data["debug_code"] = result.DebugCode
	}
	c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code_Success, Data: data})
}

func phoneAuthLogin(c *gin.Context) {
	var req phoneLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code_BadRequest, Message: "手机号和验证码不能为空"})
		return
	}
	loginSession, deviceName := loginSessionContext(c, req.DeviceType, req.DeviceName)
	result, err := user.LoginByPhoneCodeWithSession(
		c.Request.Context(), req.Phone, req.Code, c.ClientIP(), phoneRequestID(c), loginSession,
	)
	if err != nil {
		writePhoneAuthError(c, err)
		return
	}
	if result.Auth == nil {
		c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code_InternalServerError, Message: "登录服务异常"})
		return
	}
	var persisted *persistedLoginSession
	if result.Auth.Code == apipb.Code_Success {
		persisted, err = persistLoginSession(c, result.Auth.Data, deviceName)
		if err != nil {
			result.Auth.Code = apipb.Code_InternalServerError
			result.Auth.Message = err.Error()
			result.Auth.Data = ""
		}
	}
	recordLoginAttempt(
		c, result.Username, result.UserID, "phone", result.Auth.Code == 41008,
		loginSession, deviceName, result.Auth, persisted,
	)
	if result.Auth.Code != apipb.Code_Success {
		c.JSON(http.StatusOK, result.Auth)
		return
	}
	c.JSON(http.StatusOK, phoneResponse{
		Code: apipb.Code_Success,
		Data: gin.H{
			"token":        result.Auth.Data,
			"username":     result.Username,
			"is_new_user":  result.IsNewUser,
			"phone_masked": result.PhoneMasked,
		},
	})
}

func writePhoneAuthError(c *gin.Context, err error) {
	var retry *user.PhoneRateLimitError
	switch {
	case errors.As(err, &retry):
		c.Header("Retry-After", strconv.Itoa(retry.After))
		c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code(42900), Message: retry.Error()})
	case errors.Is(err, user.ErrInvalidPhone),
		errors.Is(err, user.ErrInvalidPhoneCode):
		c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code_BadRequest, Message: err.Error()})
	case errors.Is(err, user.ErrPhoneCodeExpired):
		c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code(41011), Message: err.Error()})
	case errors.Is(err, user.ErrPhoneCodeUsed):
		c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code(41012), Message: err.Error()})
	case errors.Is(err, user.ErrPhoneCodeLocked):
		c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code(41013), Message: err.Error()})
	case errors.Is(err, user.ErrPhoneLoginDisabled),
		errors.Is(err, user.ErrPhoneDelivery):
		c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code_InternalServerError, Message: err.Error()})
	default:
		c.JSON(http.StatusOK, phoneResponse{Code: apipb.Code_InternalServerError, Message: "手机登录失败"})
	}
}

func phoneRequestID(c *gin.Context) string {
	if value := strings.TrimSpace(c.GetHeader("X-Request-ID")); value != "" {
		return value
	}
	return strings.TrimSpace(middleware.GetTransID(c))
}
