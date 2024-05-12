package model

import (
	"errors"
	"strings"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/util"
)

var enforcer *casbin.Enforcer

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
	return err
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
	return enforcer.RemoveFilteredPolicy(v, p...)
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
	return e
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
