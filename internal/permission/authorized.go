package permission

import (
	"sort"

	"gorm.io/gorm"
)

// MenuAuthItem 接口:RoleMenu(permission)和 TenantMenu(tenant)都实现它。
// 用接口约束而非具体类型联合,打破 menu→permission/tenant 的循环依赖。
type MenuAuthItem interface {
	GetMenuID() string
	GetFuncs() []string
	GetShow() bool
}

// GetAuthorizedMenu 获取有权限的菜单(从 model/user.go 迁入,接口化泛型)。
// hidden 为 true 时,隐藏在菜单中不显示的数据。
func GetAuthorizedMenu[T MenuAuthItem](db *gorm.DB, authorizedMenu map[string]T, hidden bool) ([]*Menu, error) {
	parents := make(map[string]*Menu)

	var allMenus []*Menu
	treeMap := make(map[string]*Menu)
	err := db.Order("sort").Preload("MenuFuncs").Find(&allMenus).Error
	if err != nil {
		return nil, err
	}
	for _, v := range allMenus {
		treeMap[v.ID] = v
	}

	for _, roleMenu := range authorizedMenu {
		m := treeMap[roleMenu.GetMenuID()]
		if m == nil || (hidden && !roleMenu.GetShow()) || (hidden && m.Hidden) {
			continue
		}

		funcs := roleMenu.GetFuncs()
		if len(funcs) == 0 {
			m.MenuFuncs = []*MenuFunc{}
		} else {
			var menuFuncs []*MenuFunc
			for _, fn := range m.MenuFuncs {
				for _, f := range funcs {
					if fn.Name == f {
						menuFuncs = append(menuFuncs, fn)
					}
				}
			}
			m.MenuFuncs = menuFuncs
		}

		if m.ParentID == "" {
			parents[m.ID] = m
		} else {
			parent := treeMap[m.ParentID]
			parent.Children = append(parent.Children, m)
		}
	}
	var result []*Menu
	for _, m := range parents {
		SortMenu(m)
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Sort < result[j].Sort
	})
	return result, nil
}

// SortMenu 递归排序菜单子节点(从 model/user.go 迁入)
func SortMenu(m *Menu) {
	if len(m.Children) > 0 {
		sort.Slice(m.Children, func(i, j int) bool {
			return m.Children[i].Sort < m.Children[j].Sort
		})
		for _, child := range m.Children {
			SortMenu(child)
		}
	}
}
