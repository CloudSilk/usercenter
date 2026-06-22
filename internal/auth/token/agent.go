package token

import (
	"encoding/json"
	"fmt"
	"time"

	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/golang-jwt/jwt/v5"
)

// PrincipalType 标识 token 对应的主体类型(ADR-002: token 结构冻结,
// 复用现有 claims["type"] 字段承载 principal kind)
//
// 约定(REDESIGN ADR-002):
//   - type=0: 人类用户(默认,兼容现有 token)
//   - type=1: AI Agent(非人类身份 NHI)
//   - type=2: 机器服务账号(M2M)
const (
	PrincipalTypeHuman   int32 = 0
	PrincipalTypeAgent   int32 = 1
	PrincipalTypeService int32 = 2
)

// AgentClaims Agent token 的 claim 结构(与人类 token 物理分离)
type AgentClaims struct {
	AgentID     string   `json:"agentID"`
	OwnerUserID string   `json:"ownerUserID"` // 拥有该 Agent 的用户(委派链追溯)
	TenantID    string   `json:"tenantID"`
	RoleIDs     []string `json:"roleIDs"`
}

// EncodeAgentPrincipal 为 AI Agent 签发独立 token(ADR-002 核心)。
//
// 与人类 token 的区别:
//   - claims["type"] = PrincipalTypeAgent (1),DecodeToken 据此分流
//   - claims["agentID"] + claims["ownerUserID"] 承载 Agent 身份
//   - 不含人类独有字段(userName/nickname/avatar/wechat)
//   - 存入同一 TokenCache(按 agentID 为 key)
func EncodeAgentPrincipal(agentID, ownerUserID, tenantID string, roleIDs []string) (string, error) {
	expired := DefaultTokenCache.TokenExpired()

	jwtToken := jwt.New(jwt.SigningMethodHS256)
	claims := make(jwt.MapClaims)
	claims["exp"] = time.Now().Add(time.Minute * time.Duration(expired)).Unix()
	claims["iat"] = time.Now().Unix()
	claims["id"] = agentID // 复用 id 字段作为主体标识(兼容 TokenCache key)
	claims["type"] = float64(PrincipalTypeAgent)
	claims["agentID"] = agentID
	claims["ownerUserID"] = ownerUserID
	claims["tenantID"] = tenantID
	roleIDsJSON, _ := json.Marshal(roleIDs)
	claims["roleIDs"] = string(roleIDsJSON)

	jwtToken.Claims = claims
	tokenString, err := jwtToken.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}
	err = DefaultTokenCache.StoreToken(fmt.Sprint(agentID), tokenString)
	if err != nil {
		return "", err
	}
	return tokenString, err
}

// DecodeAgentPrincipal 从 token 解析 Agent 身份。
// 调用方应先用 DecodeToken 判断 type=PrincipalTypeAgent 后再调本函数。
func DecodeAgentPrincipal(t string) (*AgentClaims, error) {
	jwtToken, err := jwt.Parse(t, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if jwtToken == nil {
		return nil, err
	}
	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	return ExtractAgentClaims(claims), nil
}

// ExtractAgentClaims 从 MapClaims 提取 Agent 身份
func ExtractAgentClaims(claims jwt.MapClaims) *AgentClaims {
	ac := &AgentClaims{}
	if v, ok := claims["agentID"].(string); ok {
		ac.AgentID = v
	}
	if v, ok := claims["ownerUserID"].(string); ok {
		ac.OwnerUserID = v
	}
	if v, ok := claims["tenantID"].(string); ok {
		ac.TenantID = v
	}
	if v, ok := claims["roleIDs"].(string); ok {
		_ = json.Unmarshal([]byte(v), &ac.RoleIDs)
	}
	return ac
}

// IsAgentToken 判断 CurrentUser 是否为 Agent 身份(供 Authenticate 分流用)
func IsAgentToken(u *apipb.CurrentUser) bool {
	return u != nil && u.Type == PrincipalTypeAgent
}

// IsServiceToken 判断 CurrentUser 是否为 Service 身份
func IsServiceToken(u *apipb.CurrentUser) bool {
	return u != nil && u.Type == PrincipalTypeService
}
