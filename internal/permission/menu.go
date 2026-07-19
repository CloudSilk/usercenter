package permission

import (
	"encoding/json"
	"errors"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Menu struct {
	commonmodel.Model
	TenantID    string           `json:"tenantID" gorm:"size:36;index"`
	ProjectID   string           `json:"projectID" gorm:"index;size:36"`
	Level       uint32           `json:"level"`
	ParentID    string           `json:"parentID" gorm:"comment:父菜单ID"`
	Path        string           `json:"path" gorm:"size:200;comment:路由path"`
	Name        string           `json:"name" gorm:"size:100;comment:路由name"`
	Hidden      bool             `json:"hidden" gorm:"comment:是否在列表隐藏"`
	Component   string           `json:"component" gorm:"size:200;comment:对应前端文件路径"`
	Sort        int32            `json:"sort" gorm:"comment:排序标记"`
	Cache       bool             `json:"cache" gorm:"comment:是否缓存"`
	DefaultMenu bool             `json:"defaultMenu" gorm:"comment:是否是基础路由（开发中）"`
	Title       string           `json:"title" gorm:"size:100;comment:菜单名"`
	Icon        string           `json:"icon" gorm:"size:100;comment:菜单图标"`
	CloseTab    bool             `json:"closeTab" gorm:"comment:自动关闭tab"`
	IsMust      bool             `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
	Children    []*Menu          `json:"children" gorm:"-"`
	Parameters  []*MenuParameter `json:"parameters"`
	MenuFuncs   []*MenuFunc      `json:"menuFuncs"`
}

type MenuParameter struct {
	commonmodel.Model
	MenuID string `json:"menuID" gorm:"index"`
	Type   string `json:"type" gorm:"size:50;comment:地址栏携带参数为params还是query"`
	Key    string `json:"key" gorm:"size:100;comment:地址栏携带参数的key"`
	Value  string `json:"value" gorm:"size:200;comment:地址栏携带参数的值"`
}

type MenuFunc struct {
	commonmodel.Model
	MenuID       string        `json:"menuID" gorm:"index"`
	Name         string        `json:"name" gorm:"size:100;comment:功能名称"`
	Title        string        `json:"title" gorm:"size:100;comment:显示名称"`
	Hidden       bool          `json:"hidden" gorm:"comment:是否隐藏"`
	MenuFuncApis []MenuFuncApi `json:"menuFuncApis"`
}

type MenuFuncApi struct {
	commonmodel.Model
	MenuFuncID string `json:"menuFuncID" gorm:"index"`
	APIID      string `json:"apiID" gorm:"column:api_id"`
	API        *API   `json:"apiInfo"`
}

func AddMenu(menu *Menu) error {
	if menu.ParentID != "" {
		parent := &Menu{}
		err := store.DB().Where("id=?", menu.ParentID).First(&parent).Error
		if err != nil {
			return err
		}
		menu.Level = parent.Level + 1
	}
	return store.DB().Create(menu).Error
}

func DeleteMenu(id string) (err error) {
	var affectedUserIDs []string
	err = store.DB().Transaction(func(tx *gorm.DB) error {
		duplication, err := store.Client().CheckDuplication(tx.Model(&Menu{}), "parent_id = ?", id)
		if err != nil {
			return err
		}
		if duplication {
			return errors.New("此菜单存在子菜单不可删除")
		}
		menu, err := GetMenuByID(id)
		if err != nil {
			return err
		}
		var affectedRoleIDs []string
		if err := tx.Model(&RoleMenu{}).
			Where("menu_id = ?", id).
			Distinct("role_id").
			Pluck("role_id", &affectedRoleIDs).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&RoleMenu{}, "menu_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&MenuParameter{}, "menu_id = ?", id).Error; err != nil {
			return err
		}
		var menuFuncIDs []string
		for _, menuFunc := range menu.MenuFuncs {
			menuFuncIDs = append(menuFuncIDs, menuFunc.ID)
		}
		if len(menuFuncIDs) > 0 {
			err = tx.Unscoped().Delete(&MenuFuncApi{}, "menu_func_id in ?", menuFuncIDs).Error
			if err != nil {
				return err
			}
		}
		if err := tx.Unscoped().Delete(&MenuFunc{}, "menu_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&Menu{}, "id = ?", id).Error; err != nil {
			return err
		}
		for _, roleID := range affectedRoleIDs {
			role := &Role{}
			if err := tx.Where("id = ?", roleID).First(role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			selections, err := loadCurrentAuthorizationSelections(tx, role.ID)
			if err != nil {
				return err
			}
			preview, _, err := buildRoleAuthorizationPreview(tx, role, selections, false)
			if err != nil {
				return err
			}
			if err := replaceRoleAuthorizationPolicies(tx, role, preview.Policies); err != nil {
				return err
			}
		}
		if len(affectedRoleIDs) == 0 {
			return nil
		}
		if err := tx.Table("user_roles").
			Where("role_id IN ?", affectedRoleIDs).
			Distinct("user_id").
			Pluck("user_id", &affectedUserIDs).Error; err != nil {
			return err
		}
		if len(affectedUserIDs) == 0 {
			return nil
		}
		return tx.Table("user_session").
			Where("principal_id IN ? AND revoked = ?", affectedUserIDs, false).
			Updates(map[string]interface{}{
				"revoked":        true,
				"revoked_reason": "role menu deleted",
			}).Error
	})
	if err != nil {
		return err
	}
	if err := ReloadCasbinPolicy(); err != nil {
		return err
	}
	if token.DefaultTokenCache != nil {
		for _, userID := range affectedUserIDs {
			if err := token.DefaultTokenCache.DelByUserID(userID); err != nil {
				return err
			}
		}
	}
	return nil
}

func UpdateMenu(menu *Menu) (err error) {
	err = store.DB().Transaction(func(tx *gorm.DB) error {
		if menu.ParentID != "" {
			parent := &Menu{}
			err := tx.Where("id=?", menu.ParentID).First(&parent).Error
			if err != nil {
				return err
			}
			menu.Level = parent.Level + 1
		}
		oldMenu := &Menu{}
		err = tx.Preload("MenuFuncs.MenuFuncApis.API").Preload(clause.Associations).Where("id = ?", menu.ID).First(oldMenu).Error
		if err != nil {
			return err
		}
		var deleteAPIs []string
		var deleteMenuFuncs []string
		var deleteParams []string
		for _, oldMenuFunc := range oldMenu.MenuFuncs {
			flag := false
			for _, newMenuFunc := range menu.MenuFuncs {
				if newMenuFunc.ID == oldMenuFunc.ID {
					flag = true
					for _, oldAPI := range oldMenuFunc.MenuFuncApis {
						apiFlag := false
						for _, newAPI := range newMenuFunc.MenuFuncApis {
							if newAPI.ID == oldAPI.ID {
								apiFlag = true
							}
						}
						if !apiFlag {
							deleteAPIs = append(deleteAPIs, oldAPI.ID)
						}
					}
				}
			}
			if !flag {
				deleteMenuFuncs = append(deleteMenuFuncs, oldMenuFunc.ID)
			}
		}
		for _, oldParam := range oldMenu.Parameters {
			flag := false
			for _, newParam := range menu.Parameters {
				if newParam.ID == oldParam.ID {
					flag = true
				}
			}
			if !flag {
				deleteParams = append(deleteParams, oldParam.ID)
			}
		}
		if len(deleteParams) > 0 {
			err = tx.Unscoped().Delete(&MenuParameter{}, "id in ?", deleteParams).Error
			if err != nil {
				return err
			}
		}
		if len(deleteAPIs) > 0 {
			err = tx.Unscoped().Delete(&MenuFuncApi{}, "id in ?", deleteAPIs).Error
			if err != nil {
				return err
			}
		}
		if len(deleteMenuFuncs) > 0 {
			err = tx.Unscoped().Delete(&MenuFuncApi{}, "menu_func_id in ?", deleteMenuFuncs).Error
			if err != nil {
				return err
			}
			err = tx.Unscoped().Delete(&MenuFunc{}, "id in ?", deleteMenuFuncs).Error
			if err != nil {
				return err
			}
		}
		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Omit("created_at").Save(menu).Error
	})
	return err
}

func GetMenuByID(id string) (*Menu, error) {
	menu := &Menu{}
	err := store.DB().Preload("MenuFuncs.MenuFuncApis.API").Preload(clause.Associations).Where("id = ?", id).First(menu).Error
	return menu, err
}

func QueryMenu(req *apipb.QueryMenuRequest, resp *apipb.QueryMenuResponse, preload bool) {
	db := store.DB().Model(&Menu{})
	if req.Name != "" {
		db = db.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Path != "" {
		db = db.Where("path LIKE ?", "%"+req.Path+"%")
	}
	if req.Title != "" {
		db = db.Where("title LIKE ?", "%"+req.Title+"%")
	}
	if req.ParentID != "" {
		db = db.Where("`parent_id` = ?", req.ParentID)
	}
	if req.Level != 0 {
		db = db.Where("`level` = ?", req.Level)
	}
	if len(req.Ids) > 0 {
		db = db.Where("id in ?", req.Ids)
	}
	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`sort`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}
	var list []*Menu
	if preload {
		resp.Records, resp.Pages, err = store.Client().PageQueryWithPreload(db, req.PageSize, req.PageIndex, orderStr, []string{"MenuFuncs.MenuFuncApis", "Parameters", clause.Associations}, &list)
	} else {
		resp.Records, resp.Pages, err = store.Client().PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
	}
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = MenusToPB(list)
	}
	resp.Total = resp.Records
}

func GetAllMenus(req *apipb.QueryMenuRequest) (menus []*Menu, err error) {
	db := store.DB()
	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	err = db.Find(&menus).Error
	return
}

func ExportAllMenus(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := store.DB().Model(&Menu{}).Preload("MenuFuncs.MenuFuncApis.API").Preload(clause.Associations)
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	var list []*Menu
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

// --- PB 转换(从 model/menu_convert.go 迁入)---

func PBToMenu(in *apipb.MenuInfo) *Menu {
	if in == nil {
		return nil
	}
	return &Menu{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		ProjectID:   in.ProjectID,
		TenantID:    in.TenantID,
		Level:       in.Level,
		ParentID:    in.ParentID,
		Path:        in.Path,
		Name:        in.Name,
		Hidden:      in.Hidden,
		Component:   in.Component,
		Sort:        in.Sort,
		Cache:       in.Cache,
		DefaultMenu: in.DefaultMenu,
		Title:       in.Title,
		Icon:        in.Icon,
		Parameters:  PBToMenuParameters(in.Parameters),
		MenuFuncs:   PBToMenuFuncs(in.MenuFuncs),
		IsMust:      in.IsMust,
	}
}

func MenuToPB(in *Menu) *apipb.MenuInfo {
	if in == nil {
		return nil
	}
	var children []*apipb.MenuInfo
	if len(in.Children) > 0 {
		children = MenusToPB(in.Children)
	}
	return &apipb.MenuInfo{
		Id:          in.ID,
		ProjectID:   in.ProjectID,
		TenantID:    in.TenantID,
		Level:       in.Level,
		ParentID:    in.ParentID,
		Path:        in.Path,
		Name:        in.Name,
		Hidden:      in.Hidden,
		Component:   in.Component,
		Sort:        in.Sort,
		Cache:       in.Cache,
		DefaultMenu: in.DefaultMenu,
		Title:       in.Title,
		Icon:        in.Icon,
		Parameters:  MenuParametersToPB(in.Parameters),
		MenuFuncs:   MenuFuncsToPB(in.MenuFuncs),
		Children:    children,
		IsMust:      in.IsMust,
	}
}

func MenusToPB(in []*Menu) []*apipb.MenuInfo {
	var list []*apipb.MenuInfo
	for _, menu := range in {
		list = append(list, MenuToPB(menu))
	}
	return list
}

func PBToMenuParameters(params []*apipb.MenuParameter) []*MenuParameter {
	var list []*MenuParameter
	for _, param := range params {
		list = append(list, &MenuParameter{
			Model: commonmodel.Model{
				ID: param.Id,
			},
			MenuID: param.MenuID,
			Type:   param.Type,
			Key:    param.Key,
			Value:  param.Value,
		})
	}
	return list
}

func MenuParametersToPB(params []*MenuParameter) []*apipb.MenuParameter {
	var list []*apipb.MenuParameter
	for _, param := range params {
		list = append(list, &apipb.MenuParameter{
			Id:     param.ID,
			MenuID: param.MenuID,
			Type:   param.Type,
			Key:    param.Key,
			Value:  param.Value,
		})
	}
	return list
}

func PBToMenuFuncs(params []*apipb.MenuFunc) []*MenuFunc {
	var list []*MenuFunc
	for _, param := range params {
		list = append(list, &MenuFunc{
			Model: commonmodel.Model{
				ID: param.Id,
			},
			MenuID:       param.MenuID,
			Name:         param.Name,
			Title:        param.Title,
			Hidden:       param.Hidden,
			MenuFuncApis: PBToMenuFuncApis(param.MenuFuncApis),
		})
	}
	return list
}

func MenuFuncsToPB(params []*MenuFunc) []*apipb.MenuFunc {
	var list []*apipb.MenuFunc
	for _, param := range params {
		list = append(list, &apipb.MenuFunc{
			Id:           param.ID,
			MenuID:       param.MenuID,
			Name:         param.Name,
			Title:        param.Title,
			Hidden:       param.Hidden,
			MenuFuncApis: MenuFuncApisToPB(param.MenuFuncApis),
		})
	}
	return list
}

func PBToMenuFuncApis(params []*apipb.MenuFuncApi) []MenuFuncApi {
	var list []MenuFuncApi
	for _, param := range params {
		apiInfo := MenuFuncApi{
			Model: commonmodel.Model{
				ID: param.Id,
			},
			MenuFuncID: param.MenuFuncID,
			APIID:      param.ApiID,
		}
		if param.ApiInfo != nil {
			apiInfo.API = PBToAPI(param.ApiInfo)
		}
		list = append(list, apiInfo)
	}
	return list
}

func MenuFuncApisToPB(params []MenuFuncApi) []*apipb.MenuFuncApi {
	var list []*apipb.MenuFuncApi
	for _, param := range params {
		list = append(list, &apipb.MenuFuncApi{
			Id:         param.ID,
			MenuFuncID: param.MenuFuncID,
			ApiID:      param.APIID,
			ApiInfo:    APIToPB(param.API),
		})
	}
	return list
}
