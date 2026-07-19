package permission

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/util"
	"github.com/go-redis/redis/v8"
	"github.com/patrickmn/go-cache"
)

// enforcer Casbin 权限引擎单例
var enforcer *casbin.Enforcer

// 多实例 Casbin 同步用的 Redis 配置（可选；为空则单机模式，不注册 watcher）
var (
	casbinRedisAddr     string
	casbinRedisUserName string
	casbinRedisPwd      string
)

// SetCasbinRedis 设置 Casbin watcher 用的 Redis 地址。必须在 InitDB 之前调用。
func SetCasbinRedis(addr, userName, pwd string) {
	casbinRedisAddr = addr
	casbinRedisUserName = userName
	casbinRedisPwd = pwd
}

// InitCasbin 初始化 Casbin enforcer
func InitCasbin() {
	enforcer = NewEnforcer()
}

// UpdateCasbin 更新 casbin 权限
func UpdateCasbin(roleID string, casbinInfos []*CasbinRule) error {
	if _, err := ClearCasbin(0, roleID); err != nil {
		return err
	}
	if len(casbinInfos) > 0 {
		for _, info := range casbinInfos {
			info.Ptype = "p"
			info.RoleID = roleID
		}
		if err := store.DB().Create(&casbinInfos).Error; err != nil {
			return err
		}
	}
	if enforcer != nil {
		if err := enforcer.LoadPolicy(); err != nil {
			return err
		}
	}
	InvalidateAuthCache()
	return nil
}

// UpdateCasbinApi API 路径/方法变更随动
func UpdateCasbinApi(oldPath string, newPath string, oldMethod string, newMethod string) error {
	err := store.DB().Table("casbin_rule").Model(&CasbinRule{}).Where("v1 = ? AND v2 = ?", oldPath, oldMethod).Updates(map[string]interface{}{
		"v1": newPath,
		"v2": newMethod,
	}).Error
	if err != nil {
		return err
	}
	if enforcer != nil {
		_ = enforcer.LoadPolicy()
	}
	InvalidateAuthCache()
	return nil
}

// GetPolicyPathByRoleID 获取权限列表
func GetPolicyPathByRoleID(roleID string) (pathMaps []*CasbinRule) {
	list := enforcer.GetFilteredPolicy(0, roleID)
	for _, v := range list {
		pathMaps = append(pathMaps, &CasbinRule{
			Path:   v[1],
			Method: v[2],
		})
	}
	return pathMaps
}

// ClearCasbin 清除匹配的权限
func ClearCasbin(v int, p ...string) (bool, error) {
	fields := []string{"v0", "v1", "v2", "v3", "v4", "v5"}
	if v < 0 || v+len(p) > len(fields) {
		return false, errors.New("invalid casbin filter")
	}
	db := store.DB().Where("ptype = ?", "p")
	for index, value := range p {
		db = db.Where(fields[v+index]+" = ?", value)
	}
	result := db.Delete(&CasbinRule{})
	if result.Error != nil {
		return false, result.Error
	}
	if enforcer != nil {
		if err := enforcer.LoadPolicy(); err != nil {
			return false, err
		}
	}
	InvalidateAuthCache()
	return result.RowsAffected > 0, nil
}

// NewEnforcer 创建 Casbin enforcer（持久化到数据库 + 自定义规则）
func NewEnforcer() *casbin.Enforcer {
	a, err := NewAdapterByDBWithCustomTable(store.DB(), &CasbinRule{})
	if err != nil {
		panic(fmt.Sprintf("创建 Casbin 适配器失败: %v", err))
	}
	m, _ := casbinmodel.NewModelFromString(rbacModel)

	e, err := casbin.NewEnforcer(m, a)
	if err != nil {
		panic(fmt.Sprintf("创建 Casbin Enforcer 失败: %v", err))
	}
	e.AddFunction("ParamsMatch", ParamsMatchFunc)
	e.EnableAutoSave(false)
	_ = e.LoadPolicy()

	if casbinRedisAddr != "" {
		if w, wErr := newRedisWatcher(casbinRedisAddr, casbinRedisUserName, casbinRedisPwd); wErr == nil {
			e.SetWatcher(w)
			_ = w.SetUpdateCallback(func(string) {
				_ = e.LoadPolicy()
				InvalidateAuthCache()
			})
		} else {
			log.Errorf(context.Background(), "casbin redis watcher init failed: %v", wErr)
		}
	}
	return e
}

// redisWatcher 基于 Redis Pub/Sub 的最小 Casbin Watcher 实现
type redisWatcher struct {
	client   *redis.Client
	channel  string
	callback func(string)
	stopCh   chan struct{}
}

func newRedisWatcher(addr, userName, pwd string) (*redisWatcher, error) {
	client := redis.NewClient(&redis.Options{Addr: addr, Username: userName, Password: pwd})
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		return nil, err
	}
	return &redisWatcher{
		client:  client,
		channel: "usercenter:casbin:sync",
		stopCh:  make(chan struct{}),
	}, nil
}

func (w *redisWatcher) SetUpdateCallback(cb func(string)) error {
	w.callback = cb
	go func() {
		sub := w.client.Subscribe(context.Background(), w.channel)
		defer sub.Close()
		ch := sub.Channel()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if w.callback != nil {
					w.callback(msg.Payload)
				}
			case <-w.stopCh:
				return
			}
		}
	}()
	return nil
}

func (w *redisWatcher) Update() error {
	return w.client.Publish(context.Background(), w.channel, "update").Err()
}

func (w *redisWatcher) Close() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
	_ = w.client.Close()
}

// ParamsMatch 自定义规则函数：剥离 query 后用 KeyMatch2
func ParamsMatch(fullNameKey1 string, key2 string) bool {
	key1 := strings.Split(fullNameKey1, "?")[0]
	return util.KeyMatch2(key1, key2)
}

// ParamsMatchFunc Casbin function adapter
func ParamsMatchFunc(args ...interface{}) (interface{}, error) {
	name1 := args[0].(string)
	name2 := args[1].(string)
	return ParamsMatch(name1, name2), nil
}

const rbacModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act, checkAuth

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub=="1" || (r.sub == p.sub && ParamsMatch(r.obj,p.obj) && r.act == p.act)`

// --- 鉴权缓存（从 model/auth.go 迁入，属于权限判定基础设施）---

// authResultCache 缓存 Casbin Enforce 鉴权结果，key 为 sub|obj|act
var authResultCache = cache.New(2*time.Minute, 5*time.Minute)

// EnforceCached 带内存缓存+panic 恢复的鉴权判定（原 model/auth.go 的 enforceCached）
func EnforceCached(sub, obj, act string) (ok bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("casbin enforce panic (sub=%s obj=%s act=%s): %v\n%s", sub, obj, act, r, debug.Stack())
			ok = false
		}
	}()
	key := sub + "|" + obj + "|" + act
	if v, ok := authResultCache.Get(key); ok {
		return v.(bool), nil
	}
	ok, err = enforcer.Enforce(sub, obj, act)
	if err != nil {
		return false, err
	}
	authResultCache.SetDefault(key, ok)
	return ok, nil
}

// InvalidateAuthCache 清空全部鉴权缓存（权限规则变更后调用）
func InvalidateAuthCache() {
	authResultCache.Flush()
}

// ReloadCasbinPolicy refreshes the in-process enforcer after a transaction
// replaces role authorization rules directly in the shared database.
func ReloadCasbinPolicy() error {
	if enforcer != nil {
		if err := enforcer.LoadPolicy(); err != nil {
			return err
		}
	}
	InvalidateAuthCache()
	return nil
}
