package token

import (
	internal "github.com/CloudSilk/usercenter/internal/auth/token"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type TokenCache = internal.TokenCache

var DefaultTokenCache internal.TokenCache

func InitTokenCache(key, redisAddr, redisUserName, redisPWD string, expired int) {
	internal.InitTokenCache(key, redisAddr, redisUserName, redisPWD, expired)
	DefaultTokenCache = internal.DefaultTokenCache
}

func SetSecretKey(key string) {
	internal.SetSecretKey(key)
}

func EncodeToken(user *apipb.CurrentUser) (string, error) {
	return internal.EncodeToken(user)
}

func DecodeToken(t string) (*apipb.CurrentUser, error) {
	return internal.DecodeToken(t)
}

func IsAgentToken(user *apipb.CurrentUser) bool {
	return internal.IsAgentToken(user)
}

