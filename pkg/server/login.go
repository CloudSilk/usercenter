package server

import (
	"fmt"
	"strings"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/google/uuid"
)

type LoginRequest struct {
	UserName   string
	Password   string
	DeviceType int32
	DeviceName string
	ClientIP   string
	UserAgent  string
}

type LoginResult struct {
	Code        int32
	Message     string
	Token       string
	UserID      string
	UserName    string
	DisplayName string
	TenantID    string
}

// Login performs UserCenter password authentication and persists a manageable session.
// Hosts can expose a product-specific response without bypassing UserCenter controls.
func Login(request LoginRequest) (LoginResult, error) {
	sessionID := uuid.NewString()
	response := &apipb.LoginResponse{Code: apipb.Code_Success}
	user.LoginWithSession(&apipb.LoginRequest{
		UserName: request.UserName,
		Password: request.Password,
	}, response, user.LoginSessionContext{
		ID: sessionID, DeviceType: request.DeviceType, ClientIP: request.ClientIP,
	})
	result := LoginResult{Code: int32(response.Code), Message: response.Message, Token: response.Data}
	if response.Code != apipb.Code_Success {
		return result, nil
	}

	current, err := token.DecodeToken(response.Data)
	if err != nil || current == nil {
		return LoginResult{}, fmt.Errorf("decode issued login token: %w", err)
	}
	tokenSignature := token.GetTokenSignature(response.Data)
	if current.SessionID == "" || tokenSignature == "" {
		_ = token.DefaultTokenCache.Del(current.Id, response.Data)
		return LoginResult{}, fmt.Errorf("issued login token has no manageable session")
	}
	deviceName := strings.TrimSpace(request.DeviceName)
	if deviceName == "" {
		deviceName = "Web"
	}
	if err := session.CreateSession(&session.Session{
		Model: commonmodel.Model{ID: current.SessionID}, PrincipalID: current.Id,
		TenantID: current.TenantID, TokenSig: tokenSignature, DeviceType: current.DeviceType,
		DeviceName: deviceName, IP: request.ClientIP, UserAgent: request.UserAgent,
	}); err != nil {
		_ = token.DefaultTokenCache.Del(current.Id, response.Data)
		return LoginResult{}, fmt.Errorf("record login session: %w", err)
	}
	result.UserID = current.Id
	result.UserName = current.UserName
	result.DisplayName = current.Nickname
	result.TenantID = current.TenantID
	return result, nil
}
