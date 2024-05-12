package token

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
	"github.com/patrickmn/go-cache"
)

var DefaultTokenCache TokenCache

func InitTokenCache(key, redisAddr, redisUserName, redisPWD string, expired int) {
	SetSecretKey(key)
	if expired < 1 {
		expired = 120
	}
	if redisAddr != "" {
		DefaultTokenCache = NewRedis(redisAddr, redisUserName, redisPWD, expired)
	} else {
		DefaultTokenCache = NewMemory(expired)
	}
}

type TokenCache interface {
	//DelByUserID 删除该用户所有token
	DelByUserID(userID string) error
	//Del 删除该用户指定Token
	Del(userID, token string) error
	//Exists 判断Token是否存在
	Exists(userID, token string) (bool, error)
	//StoreToken 存储Token
	StoreToken(userID, token string) error
	//TokenExpired Token过期时间，单位分钟
	TokenExpired() int
	StorePrivateKey(sessionID string, privateKey string) error
	GetPrivateKey(sessionID string) (string, bool)
	DelPrivateKey(sessionID string) error

	StorePublicKey(sessionID string, publicKey string) error
	GetPublicKey(sessionID string) (string, bool)
	DelPublicKey(sessionID string) error
}

func NewRedis(addr, userName, pwd string, expired int) TokenCache {
	r := &Redis{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: pwd,
			Username: userName,
			DB:       0,
			// TLSConfig: &tls.Config{
			// 	InsecureSkipVerify: true,
			// },
		}),
		tokenExpired: expired,
	}

	pong, err := r.client.Ping(context.Background()).Result()
	if err != nil {
		panic(err)
	}
	fmt.Println(pong)
	return r
}

type Redis struct {
	client       *redis.Client
	tokenExpired int
}

func (r *Redis) DelByUserID(userID string) error {
	it := r.client.Scan(context.Background(), 0, r.getKey(userID, "*"), 50).Iterator()
	for it.Next(context.Background()) {
		key := it.Val()
		token, err := r.client.Get(context.Background(), key).Result()
		if err != nil {
			continue
		}
		sessionID, err := GetSessionID(token)
		if err != nil {
			continue
		}
		r.DelPrivateKey(sessionID)
		r.DelPublicKey(sessionID)
	}

	_, err := r.client.Del(context.Background(), r.getKey(userID, "*")).Result()
	if err != nil {
		return err
	}

	return err
}

func (r *Redis) Del(userID, token string) error {
	field := r.getField(token)
	if field == "" {
		return nil
	}
	err := r.client.Del(context.Background(), r.getKey(userID, field)).Err()
	if err != nil {
		return err
	}

	sessionID, err := getSessionID(field)
	if err != nil {
		return err
	}
	err = r.DelPrivateKey(sessionID)
	if err != nil {
		return err
	}

	return r.DelPublicKey(sessionID)
}

func (r *Redis) Exists(userID, token string) (bool, error) {
	field := r.getField(token)
	if field == "" {
		return false, nil
	}
	userID, err := GetUserID(token)
	if err != nil {
		return false, err
	}
	exists, err := r.client.Exists(context.Background(), r.getKey(userID, field)).Result()
	return exists == 1, err
}

func (r *Redis) StoreToken(userID, token string) error {
	field := r.getField(token)
	if field == "" {
		return nil
	}
	_, err := r.client.Set(context.Background(), r.getKey(userID, field), token, defaultExpired).Result()
	return err
}

func (r *Redis) GetPrivateKey(sessionID string) (string, bool) {
	privateKey, err := r.client.Get(context.Background(), r.getSessionKey(sessionID)).Result()
	if err != nil {
		return "", false
	}
	return privateKey, true
}

func (r *Redis) StorePrivateKey(sessionID string, privateKey string) error {
	_, err := r.client.Set(context.Background(), r.getSessionKey(sessionID), privateKey, defaultExpired).Result()
	return err
}

func (r *Redis) DelPrivateKey(sessionID string) error {
	_, err := r.client.Del(context.Background(), r.getSessionKey(sessionID)).Result()
	return err
}

func (r *Redis) StorePublicKey(sessionID string, publicKey string) error {
	_, err := r.client.Set(context.Background(), r.getSessionPublicKey(sessionID), publicKey, defaultExpired).Result()
	return err
}
func (r *Redis) GetPublicKey(sessionID string) (string, bool) {
	key, err := r.client.Get(context.Background(), r.getSessionPublicKey(sessionID)).Result()
	if err != nil {
		return "", false
	}
	return key, true
}
func (r *Redis) DelPublicKey(sessionID string) error {
	_, err := r.client.Del(context.Background(), r.getSessionPublicKey(sessionID)).Result()
	return err
}

func (r *Redis) TokenExpired() int {
	return r.tokenExpired
}

func (r *Redis) getKey(userID, token string) string {
	return fmt.Sprintf("token:%s:%s", userID, token)
}

func (r *Redis) getSessionKey(sessionID string) string {
	return fmt.Sprintf("session:private:key:%s", sessionID)
}

func (r *Redis) getSessionPublicKey(sessionID string) string {
	return fmt.Sprintf("session:publick:key:%s", sessionID)
}

func (r *Redis) getField(token string) string {
	array := strings.Split(token, ".")
	if len(array) == 3 {
		return array[2]
	}
	return ""
}

type Memory struct {
	tokenCache      *cache.Cache
	privateKeyCache *cache.Cache
	publicKeyCache  *cache.Cache
	userCache       map[string]map[string]struct{}
	tokenLock       sync.RWMutex
	tokenExpired    int
}

func (m *Memory) DelByUserID(userID string) error {
	m.tokenLock.Lock()
	tokens, ok := m.userCache[userID]
	sigs := make([]string, 0)
	if ok {
		for sig := range tokens {
			sigs = append(sigs, sig)
		}
		delete(m.userCache, userID)
	}

	m.tokenLock.Unlock()

	for _, sig := range sigs {
		m.tokenCache.Delete(sig)

		sessionID, err := getSessionID(sig)
		if err != nil {
			log.Error(nil, err, sig)
			continue
		}
		m.DelPrivateKey(sessionID)
		m.DelPublicKey(sessionID)
	}

	return nil
}

func (m *Memory) Del(userID, token string) error {
	m.tokenLock.Lock()
	tokens, ok := m.userCache[userID]
	if !ok {
		m.tokenLock.Unlock()
		return nil
	}

	array := strings.Split(token, ".")
	if len(array) == 3 {
		delete(tokens, array[2])
	}

	m.tokenLock.Unlock()

	if len(array) == 3 {
		m.tokenCache.Delete(array[2])
	}

	sessionID, err := getSessionID(array[1])
	if err != nil {
		return err
	}
	m.DelPrivateKey(sessionID)
	return m.DelPublicKey(sessionID)
}

func (m *Memory) Exists(userID, token string) (bool, error) {
	m.tokenLock.RLock()
	ok := false
	array := strings.Split(token, ".")
	if len(array) == 3 {
		_, ok = m.tokenCache.Get(array[2])
	}
	m.tokenLock.RUnlock()
	return ok, nil
}

func (m *Memory) StoreToken(userID, token string) error {
	m.tokenLock.Lock()
	_, ok := m.userCache[userID]
	if !ok {
		m.userCache[userID] = make(map[string]struct{})
	}

	array := strings.Split(token, ".")
	if len(array) == 3 {
		m.userCache[userID][array[2]] = struct{}{}
		m.tokenCache.SetDefault(array[2], token)
	}

	m.tokenLock.Unlock()
	return nil
}

func (m *Memory) TokenExpired() int {
	return m.tokenExpired
}

func (r *Memory) GetPrivateKey(sessionID string) (string, bool) {
	privateKey, ok := r.privateKeyCache.Get(sessionID)
	if !ok {
		return "", false
	}
	return privateKey.(string), true
}
func (r *Memory) StorePrivateKey(sessionID string, privateKey string) error {
	r.privateKeyCache.SetDefault(sessionID, privateKey)
	return nil
}

func (r *Memory) DelPrivateKey(sessionID string) error {
	r.privateKeyCache.Delete(sessionID)
	return nil
}

func (r *Memory) StorePublicKey(sessionID string, publicKey string) error {
	r.publicKeyCache.SetDefault(sessionID, publicKey)
	return nil
}
func (r *Memory) GetPublicKey(sessionID string) (string, bool) {
	publicKey, ok := r.publicKeyCache.Get(sessionID)
	if !ok {
		return "", false
	}
	return publicKey.(string), true
}
func (r *Memory) DelPublicKey(sessionID string) error {
	r.publicKeyCache.Delete(sessionID)
	return nil
}

func NewMemory(expired int) TokenCache {
	m := &Memory{
		tokenCache:      cache.New(defaultExpired, 10*time.Minute),
		privateKeyCache: cache.New(defaultExpired, 10*time.Minute),
		publicKeyCache:  cache.New(defaultExpired, 10*time.Minute),
		userCache:       make(map[string]map[string]struct{}),
		tokenExpired:    expired,
	}
	m.tokenCache.OnEvicted(func(sig string, t interface{}) {
		fmt.Println("expired", sig, t)
		token := t.(string)
		currentUser, err := DecodeToken(token)
		if err != nil {
			if !errors.As(err, &jwt.ErrTokenExpired) {
				return
			}
		}

		m.tokenLock.Lock()
		tokens, ok := m.userCache[fmt.Sprint(currentUser.Id)]
		if !ok {
			m.tokenLock.Unlock()
			return
		}

		array := strings.Split(token, ".")
		if len(array) == 3 {
			delete(tokens, array[2])
		}

		m.tokenLock.Unlock()
	})
	return m
}
