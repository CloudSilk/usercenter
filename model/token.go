package model

import "github.com/CloudSilk/usercenter/internal/auth/token"

func InitTokenCache(key, redisAddr, redisUserName, redisPWD string, expired int) {
	token.InitTokenCache(key, redisAddr, redisUserName, redisPWD, expired)
}
