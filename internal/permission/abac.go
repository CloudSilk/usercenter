package permission

// ABAC + DataScope(REDESIGN #7)
// 在 RBAC 基础上增加属性级 + 数据范围级授权

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/store"
)

// DataScope 数据范围(行级授权)
type DataScope int32

const (
	DataScopeAll    DataScope = 0 // 全部数据
	DataScopeTenant DataScope = 1 // 本租户
	DataScopeDept   DataScope = 2 // 本部门(group)
	DataScopeSelf   DataScope = 3 // 仅本人
	DataScopeCustom DataScope = 4 // 自定义条件
)

// ABACPolicy ABAC 策略规则
type ABACPolicy struct {
	ID        uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID  string `json:"tenantID" gorm:"index;size:36"`
	RoleID    string `json:"roleID" gorm:"index;size:36;comment:关联角色"`
	Resource  string `json:"resource" gorm:"index;size:100;comment:资源类型(user/order/...)"`
	Action    string `json:"action" gorm:"index;size:50;comment:操作(read/write/delete)"`
	DataScope int32  `json:"dataScope" gorm:"comment:数据范围"`
	Condition string `json:"condition" gorm:"size:1000;comment:ABAC条件表达式(JSON)"`
	Enable    bool   `json:"enable" gorm:"index;default:true"`
	Priority  int32  `json:"priority" gorm:"default:0;comment:优先级(高优先匹配)"`
}

func (ABACPolicy) TableName() string { return "abac_policy" }

// ABACContext ABAC 判定上下文
type ABACContext struct {
	Principal principal.Principal
	Resource  string                 // 资源类型
	Action    string                 // 操作
	Attrs     map[string]interface{} // 资源属性(如 tenant_id, owner_id, dept)
}

// EvaluateABAC 判定 ABAC 策略(返回是否允许 + 数据范围)
func EvaluateABAC(ctx context.Context, abacCtx *ABACContext) (allowed bool, scope DataScope, err error) {
	if store.DB() == nil {
		return true, DataScopeAll, nil // 无 DB = 开发模式,全放行
	}

	// 查匹配策略(按角色 + 资源 + 操作)
	p := abacCtx.Principal
	if p == nil {
		return false, DataScopeSelf, nil
	}

	// 平台超管全放行
	if isSuperAdmin(p) {
		return true, DataScopeAll, nil
	}

	var policies []*ABACPolicy
	query := store.DB().Where("enable = ? AND resource = ? AND action = ?",
		true, abacCtx.Resource, abacCtx.Action)

	// 按角色过滤
	if len(p.Roles()) > 0 {
		query = query.Where("role_id IN ?", p.Roles())
	}
	if p.TenantID() != "" {
		query = query.Where("tenant_id IN (?, '')", p.TenantID())
	}

	if e := query.Order("priority desc").Find(&policies).Error; e != nil {
		return false, DataScopeSelf, e
	}

	if len(policies) == 0 {
		// 无 ABAC 策略 = 回退 RBAC(已由 EnforceCached 判定)
		return true, DataScopeTenant, nil
	}

	// 取最高优先级策略
	top := policies[0]
	if !evaluateCondition(top.Condition, abacCtx) {
		return false, DataScopeSelf, nil
	}
	return true, DataScope(top.DataScope), nil
}

// ApplyDataScope 将 DataScope 转为 GORM WHERE 条件
func ApplyDataScope(scope DataScope, p principal.Principal) string {
	switch scope {
	case DataScopeAll:
		return ""
	case DataScopeTenant:
		if p.TenantID() != "" {
			return fmt.Sprintf("tenant_id = '%s'", p.TenantID())
		}
		return ""
	case DataScopeDept:
		return fmt.Sprintf("`group` = '%s'", p.Subject())
	case DataScopeSelf:
		return fmt.Sprintf("(id = '%s' OR created_by = '%s')", p.Subject(), p.Subject())
	default:
		return ""
	}
}

// evaluateCondition ABAC 条件评估引擎
// JSON 格式:{"field": {"op": "value"}, "field2": {"op": "value"}, ...}
// 支持操作符:eq, neq, in, not_in, gt, lt, contains, startswith
// 支持变量:${principal.id}, ${principal.tenantID}, ${principal.kind}
// 多条件 AND 关系(全满足才放行)
// 示例:{"owner_id":{"eq":"${principal.id}"}, "level":{"gt":5}}
func evaluateCondition(condition string, ctx *ABACContext) bool {
	if condition == "" {
		return true
	}
	conds, err := parseConditions(condition)
	if err != nil || len(conds) == 0 {
		return true // 解析失败 = 放行(不阻断正常请求)
	}
	for field, checks := range conds {
		resVal, ok := ctx.Attrs[field]
		if !ok {
			resVal = "" // 资源无此属性,按空值处理
		}
		for op, condVal := range checks {
			resolved := resolveVar(condVal, ctx)
			if !matchOp(op, resVal, resolved) {
				return false // AND 关系:任一不满足 = 拒绝
			}
		}
	}
	return true
}

// parsedCondition field → {op: value}
type parsedCondition = map[string]map[string]interface{}

func parseConditions(json string) (parsedCondition, error) {
	result := make(parsedCondition)
	// 简易 JSON 解析(不引 encoding/json 避免性能开销)
	// 格式:{"field":{"op":"value"},...}
	s := strings.TrimSpace(json)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, errInvalidCondition
	}
	s = s[1 : len(s)-1]
	if s == "" {
		return result, nil
	}
	for _, pair := range splitTopLevel(s, ',') {
		colon := strings.Index(pair, ":")
		if colon < 0 {
			continue
		}
		field := strings.Trim(strings.TrimSpace(pair[:colon]), `"`)
		valPart := strings.TrimSpace(pair[colon+1:])
		if !strings.HasPrefix(valPart, "{") {
			continue
		}
		valPart = strings.Trim(valPart, "{}")
		oc := strings.Index(valPart, ":")
		if oc < 0 {
			continue
		}
		op := strings.Trim(strings.TrimSpace(valPart[:oc]), `"`)
		val := strings.Trim(strings.TrimSpace(valPart[oc+1:]), `"`)
		if result[field] == nil {
			result[field] = make(map[string]interface{})
		}
		result[field][op] = val
	}
	return result, nil
}

func resolveVar(val interface{}, ctx *ABACContext) interface{} {
	s, ok := val.(string)
	if !ok {
		return val
	}
	switch s {
	case "${principal.id}":
		if ctx.Principal != nil {
			return ctx.Principal.Subject()
		}
	case "${principal.tenantID}":
		if ctx.Principal != nil {
			return ctx.Principal.TenantID()
		}
	case "${principal.kind}":
		if ctx.Principal != nil {
			return int32(ctx.Principal.Kind())
		}
	}
	return s
}

func matchOp(op string, resVal, condVal interface{}) bool {
	resStr := fmt.Sprintf("%v", resVal)
	condStr := fmt.Sprintf("%v", condVal)
	switch op {
	case "eq":
		return resStr == condStr
	case "neq":
		return resStr != condStr
	case "in":
		items := strings.Split(condStr, ",")
		for _, item := range items {
			if strings.TrimSpace(item) == resStr {
				return true
			}
		}
		return false
	case "not_in":
		items := strings.Split(condStr, ",")
		for _, item := range items {
			if strings.TrimSpace(item) == resStr {
				return false
			}
		}
		return true
	case "gt", "lt":
		var rv, cv float64
		fmt.Sscanf(resStr, "%f", &rv)
		fmt.Sscanf(condStr, "%f", &cv)
		if op == "gt" {
			return rv > cv
		}
		return rv < cv
	case "contains":
		return strings.Contains(resStr, condStr)
	case "startswith":
		return strings.HasPrefix(resStr, condStr)
	default:
		return true // 未知 op = 放行
	}
}

func splitTopLevel(s string, sep byte) []string {
	var result []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case sep:
			if depth == 0 {
				result = append(result, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

var errInvalidCondition = errors.New("invalid ABAC condition")

func isSuperAdmin(p principal.Principal) bool {
	for _, r := range p.Roles() {
		if r == "super_admin" || r == "1" {
			return true
		}
	}
	return false
}

// CRUD for ABAC policies

func CreateABACPolicy(policy *ABACPolicy) error {
	return store.DB().Create(policy).Error
}

func UpdateABACPolicy(policy *ABACPolicy) error {
	return store.DB().Save(policy).Error
}

func DeleteABACPolicy(id uint64) error {
	return store.DB().Delete(&ABACPolicy{}, id).Error
}

func GetABACPolicies(roleID string) (list []*ABACPolicy, err error) {
	err = store.DB().Where("role_id = ?", roleID).Order("priority desc").Find(&list).Error
	return
}

// Suppress unused
var _ = strings.Contains
