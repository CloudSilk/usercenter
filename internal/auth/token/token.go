package token

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"encoding/base64"
	"encoding/json"

	"github.com/CloudSilk/usercenter/internal/principal"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const defaultExpired = 30 * 24 * time.Hour // 回退默认值，当配置值转换失败时使用

var secretKey = ""

// SetSecretKey 设置Token加密key
func SetSecretKey(key string) {
	secretKey = key
}

// EncodeToken 生产Token
func EncodeToken(user *apipb.CurrentUser) (string, error) {
	tokenString, err := signToken(user)
	if err != nil {
		return "", err
	}
	if err := DefaultTokenCache.StoreToken(fmt.Sprint(user.Id), tokenString); err != nil {
		return "", err
	}
	return tokenString, nil
}

// RotateToken replaces one active access token without deleting the session's
// asymmetric key material. Callers must update the persisted session token
// signature and roll this cache change back if that database update fails.
func RotateToken(user *apipb.CurrentUser, oldToken string) (string, error) {
	tokenString, err := signToken(user)
	if err != nil {
		return "", err
	}
	if err := DefaultTokenCache.ReplaceToken(fmt.Sprint(user.Id), oldToken, tokenString); err != nil {
		return "", err
	}
	return tokenString, nil
}

func signToken(user *apipb.CurrentUser) (string, error) {
	expired := DefaultTokenCache.TokenExpired()

	token := jwt.New(jwt.SigningMethodHS256)
	claims := make(jwt.MapClaims)
	claims["exp"] = time.Now().Add(time.Minute * time.Duration(expired)).Unix()
	claims["iat"] = time.Now().Unix()
	claims["jti"] = uuid.NewString()
	claims["id"] = user.Id
	claims["userName"] = user.UserName
	claims["domain"] = user.Domain
	claims["deviceType"] = user.DeviceType
	claims["clientIP"] = user.ClientIP
	claims["sessionID"] = user.SessionID
	claims["tenantID"] = user.TenantID
	claims["key"] = user.Key
	roleIDs, _ := json.Marshal(user.RoleIDs)
	claims["roleIDs"] = string(roleIDs)
	claims["type"] = user.Type
	claims["group"] = user.Group
	claims["nickname"] = user.Nickname
	claims["avatar"] = user.Avatar
	claims["isVip"] = user.IsVip
	claims["vipExpired"] = user.VipExpired

	token.Claims = claims
	return token.SignedString([]byte(secretKey))
}

// DecodeToken  解析token
func DecodeToken(t string) (*apipb.CurrentUser, error) {
	if t == "" {
		return nil, errors.New("未登录，请先登录!!")
	}
	token, err := jwt.Parse(t,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secretKey), nil
		})
	if token != nil && token.Valid {
		return ExtractorCurrentUser(token), nil
	}

	//刷新token用
	if token != nil && errors.Is(err, jwt.ErrTokenExpired) {
		return ExtractorCurrentUser(token), err
	}

	return nil, err

}

func ExtractorCurrentUser(t *jwt.Token) *apipb.CurrentUser {
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil
	}

	currentUser := apipb.CurrentUser{}
	id, _ := (claims["id"]).(string)
	currentUser.Id = id
	if v, ok := claims["userName"].(string); ok {
		currentUser.UserName = v
	}

	if _, ok := claims["domain"]; ok {
		currentUser.Domain = claims["domain"].(string)
	}
	if _, ok := claims["nickname"]; ok {
		currentUser.Nickname = claims["nickname"].(string)
	}
	if _, ok := claims["avatar"]; ok {
		currentUser.Avatar = claims["avatar"].(string)
	}

	if _, ok := claims["deviceType"]; ok {
		currentUser.DeviceType = int32(claims["deviceType"].(float64))
	}

	if _, ok := claims["type"]; ok {
		currentUser.Type = int32(claims["type"].(float64))
	}

	if _, ok := claims["group"]; ok {
		currentUser.Group = claims["group"].(string)
	}

	if _, ok := claims["tenantID"]; ok {
		currentUser.TenantID = claims["tenantID"].(string)
	}

	if _, ok := claims["clientIP"]; ok {
		currentUser.ClientIP = claims["clientIP"].(string)
	}

	if _, ok := claims["sessionID"]; ok {
		currentUser.SessionID = claims["sessionID"].(string)
	}

	if _, ok := claims["key"]; ok {
		currentUser.Key = claims["key"].(string)
	}
	if _, ok := claims["isVip"]; ok {
		currentUser.IsVip = claims["isVip"].(bool)
	}
	if _, ok := claims["vipExpired"]; ok {
		currentUser.VipExpired = int64(claims["vipExpired"].(float64))
	}

	if _, ok := claims["roleIDs"]; ok {
		str, ok := claims["roleIDs"].(string)
		if ok {
			_ = json.Unmarshal([]byte(str), &currentUser.RoleIDs)
		}
	}

	return &currentUser
}

func GetUserID(t string) (string, error) {
	claims, err := decodeClaims(t)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(claims["id"]), nil
}

func GetSessionID(t string) (string, error) {
	array := strings.Split(t, ".")
	if len(array) != 3 {
		return "", nil
	}
	return getSessionID(array[1])
}

// GetTokenSignature returns the JWT signature segment used as the persisted
// session revocation key. It never returns the full bearer token.
func GetTokenSignature(t string) string {
	array := strings.Split(t, ".")
	if len(array) != 3 {
		return ""
	}
	return array[2]
}

func getSessionID(sig string) (string, error) {
	claims, err := decodeClaimsFromPayload(sig)
	if err != nil {
		return "", err
	}

	sessionID, _ := claims["sessionID"].(string)
	return sessionID, nil
}

func decodeClaims(token string) (map[string]interface{}, error) {
	array := strings.Split(token, ".")
	if len(array) != 3 {
		return nil, nil
	}
	return decodeClaimsFromPayload(array[1])
}

func decodeClaimsFromPayload(payload string) (map[string]interface{}, error) {
	b, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}

	claims := make(map[string]interface{})
	if err := json.Unmarshal(b, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// EncodeTokenFromPrincipal 从 Principal 签发 token(阶段3:Gate 3)。
// 与 EncodeToken 的区别:不依赖 *apipb.CurrentUser,直接用 Principal interface。
func EncodeTokenFromPrincipal(p principal.Principal) (string, error) {
	if p == nil {
		return "", errors.New("nil principal")
	}
	expired := DefaultTokenCache.TokenExpired()

	jwtToken := jwt.New(jwt.SigningMethodHS256)
	claims := make(jwt.MapClaims)
	claims["exp"] = time.Now().Add(time.Minute * time.Duration(expired)).Unix()
	claims["iat"] = time.Now().Unix()
	claims["id"] = p.Subject()
	claims["tenantID"] = p.TenantID()
	claims["type"] = float64(p.Kind() - 1)
	roleIDs, _ := json.Marshal(p.Roles())
	claims["roleIDs"] = string(roleIDs)

	// Agent 专属字段
	if p.Kind() == principal.KindAgent {
		if a, ok := p.(*principal.AgentPrincipal); ok {
			claims["agentID"] = a.Subject()
			claims["ownerUserID"] = a.OwnerUserID()
		}
	}

	jwtToken.Claims = claims
	tokenString, err := jwtToken.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}
	err = DefaultTokenCache.StoreToken(fmt.Sprint(p.Subject()), tokenString)
	return tokenString, err
}
