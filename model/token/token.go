package token

import (
	internaltoken "github.com/CloudSilk/usercenter/internal/auth/token"
	apipb "github.com/CloudSilk/usercenter/proto"
)

func InitTokenCache(key, redisAddr, redisUserName, redisPWD string, expired int) {
	internaltoken.InitTokenCache(key, redisAddr, redisUserName, redisPWD, expired)
}

func SetSecretKey(key string) {
	internaltoken.SetSecretKey(key)
}

func EncodeToken(user *apipb.CurrentUser) (string, error) {
	return internaltoken.EncodeToken(user)
}

func DecodeToken(t string) (*apipb.CurrentUser, error) {
	return internaltoken.DecodeToken(t)
}

func GetUserID(t string) (string, error) {
	return internaltoken.GetUserID(t)
}

func GetSessionID(t string) (string, error) {
	return internaltoken.GetSessionID(t)
}
