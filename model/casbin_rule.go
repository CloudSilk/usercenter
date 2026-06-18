package model

import (
	"context"
	"errors"
	"strings"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/util"
	"github.com/go-redis/redis/v8"
)

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

func InitCasbin() {
	enforcer = NewEnforcer()
}

// @author: [guoxf](https://github.com/guoxf)
// @function: UpdateCasbin
// @description: 更新casbin权限
// @param: roleID string, casbinInfos []CasbinRule
// @return: error
func UpdateCasbin(roleID string, casbinInfos []*CasbinRule) error {
	ClearCasbin(0, roleID)
	rules := [][]string{}
	for _, v := range casbinInfos {
		rules = append(rules, []string{roleID, v.Path, v.Method, v.CheckAuth})
	}

	success, err := enforcer.AddNamedPolicies("p", rules)
	if err != nil {
		return err
	}
	if !success {
		return errors.New("存在相同api,添加失败,请联系管理员")
	}
	invalidateAuthCache()
	return nil
}

// @author: [guoxf](https://github.com/guoxf)
// @function: UpdateCasbinApi
// @description: API更新随动
// @param: oldPath string, newPath string, oldMethod string, newMethod string
// @return: error
func UpdateCasbinApi(oldPath string, newPath string, oldMethod string, newMethod string) error {
	err := dbClient.DB().Table("casbin_rule").Model(&CasbinRule{}).Where("v1 = ? AND v2 = ?", oldPath, oldMethod).Updates(map[string]interface{}{
		"v1": newPath,
		"v2": newMethod,
	}).Error
	if err != nil {
		return err
	}
	// 直接写 DB 不会触发 watcher 自动通知，需手动重载本实例策略并清空鉴权缓存，
	// 使 API 路径/方法变更立即生效（其他实例依赖 watcher 或缓存 TTL 最终一致）
	if enforcer != nil {
		_ = enforcer.LoadPolicy()
	}
	invalidateAuthCache()
	return nil
}

// @author: [guoxf](https://github.com/guoxf)
// @function: GetPolicyPathByRoleID
// @description: 获取权限列表
// @param: roleID string
// @return: pathMaps []CasbinRule
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

// @author: [guoxf](https://github.com/guoxf)
// @function: ClearCasbin
// @description: 清除匹配的权限
// @param: v int, p ...string
// @return: bool
func ClearCasbin(v int, p ...string) (bool, error) {
	ok, err := enforcer.RemoveFilteredPolicy(v, p...)
	invalidateAuthCache()
	return ok, err
}

// @author: [guoxf](https://github.com/guoxf)
// @function: NewEnforcer
// @description: 持久化到数据库  引入自定义规则
// @return: *casbin.Enforcer
func NewEnforcer() *casbin.Enforcer {
	a, err := NewAdapterByDBWithCustomTable(dbClient.DB(), &CasbinRule{})
	if err != nil {
		panic(err)
	}
	m, _ := casbinmodel.NewModelFromString(rbacModel)

	e, err := casbin.NewEnforcer(m, a)
	if err != nil {
		panic(err)
	}
	e.AddFunction("ParamsMatch", ParamsMatchFunc)
	e.EnableAutoSave(false)
	_ = e.LoadPolicy()

	// 多实例同步：若配置了 Redis，注册 watcher。
	// 任一实例通过 ClearCasbin/UpdateCasbin 等改了策略，casbin 会自动调 watcher.Update() 发布通知，
	// 其他实例订阅后重新 LoadPolicy 并清空鉴权缓存。
	if casbinRedisAddr != "" {
		if w, wErr := newRedisWatcher(casbinRedisAddr, casbinRedisUserName, casbinRedisPwd); wErr == nil {
			e.SetWatcher(w)
			_ = w.SetUpdateCallback(func(string) {
				_ = e.LoadPolicy()
				invalidateAuthCache()
			})
		} else {
			log.Errorf(context.Background(), "casbin redis watcher init failed: %v", wErr)
		}
	}
	return e
}

// redisWatcher 基于 Redis Pub/Sub 的最小 Casbin Watcher 实现，
// 复用项目已有的 go-redis/v8，避免引入额外依赖。
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

// @author: [guoxf](https://github.com/guoxf)
// @function: ParamsMatch
// @description: 自定义规则函数
// @param: fullNameKey1 string, key2 string
// @return: bool
func ParamsMatch(fullNameKey1 string, key2 string) bool {
	key1 := strings.Split(fullNameKey1, "?")[0]
	// 剥离路径后再使用casbin的keyMatch2
	return util.KeyMatch2(key1, key2)
}

// @author: [guoxf](https://github.com/guoxf)
// @function: ParamsMatchFunc
// @description: 自定义规则函数
// @param: args ...interface{}
// @return: interface{}, error
func ParamsMatchFunc(args ...interface{}) (interface{}, error) {
	name1 := args[0].(string)
	name2 := args[1].(string)
	// fmt.Println("====>ParamsMatchFunc", args)
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
