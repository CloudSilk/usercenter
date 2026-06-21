package model

import (
	"github.com/CloudSilk/usercenter/internal/auth/token"
	apipb "github.com/CloudSilk/usercenter/proto"
)

func InitTokenCache(key, redisAddr, redisUserName, redisPWD string, expired int) {
	token.InitTokenCache(key, redisAddr, redisUserName, redisPWD, expired)
}

func EncodeToken(user *apipb.CurrentUser) (string, error) {
	return token.EncodeToken(user)
}
