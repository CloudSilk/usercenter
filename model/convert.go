package model

import (
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	apipb "github.com/CloudSilk/usercenter/proto"
)

func PBToUser(in *apipb.UserInfo) *User {
	if in == nil {
		return nil
	}
	if len(in.VipExpired) == 10 {
		in.VipExpired = in.VipExpired + " 15:59:59"
	}
	vipExpired := utils.ParseTime(in.VipExpired)
	return &User{
		TenantModel: commonmodel.TenantModel{
			Model: commonmodel.Model{
				ID: in.Id,
			},
			TenantID: in.TenantID,
		},
		ProjectID:        in.ProjectID,
		UserName:         in.UserName,
		Nickname:         in.Nickname,
		UserRoles:        PBToUserRoles(in.UserRoles),
		RoleIDs:          in.RoleIDs,
		Enable:           in.Enable,
		Email:            in.Email,
		Mobile:           in.Mobile,
		IDCard:           in.IdCard,
		Avatar:           in.Avatar,
		EID:              in.Eid,
		Title:            in.Title,
		Description:      in.Description,
		RealName:         in.RealName,
		Gender:           in.Gender,
		Type:             in.Type,
		Group:            in.Group,
		WechatUnionID:    in.WechatUnionID,
		WechatOpenID:     in.WechatOpenID,
		City:             in.City,
		Country:          in.Country,
		Province:         in.Province,
		Password:         in.Password,
		WechatConfigID:   in.WechatConfigID,
		IsMust:           in.IsMust,
		IsVip:            time.Until(vipExpired).Seconds() > 0,
		VipExpired:       vipExpired,
		WechatOpenIDMaps: PBToUserWechatOpenIDMaps(in.WechatOpenIDMaps),
	}
}

func UserToPB(in *User) *apipb.UserInfo {
	if in == nil {
		return nil
	}
	user := &apipb.UserInfo{
		Id:               in.ID,
		TenantID:         in.TenantID,
		ProjectID:        in.ProjectID,
		UserName:         in.UserName,
		Nickname:         in.Nickname,
		UserRoles:        UserRolesToPB(in.UserRoles),
		RoleIDs:          in.RoleIDs,
		Enable:           in.Enable,
		Email:            in.Email,
		Mobile:           in.Mobile,
		IdCard:           in.IDCard,
		Avatar:           in.Avatar,
		Eid:              in.EID,
		Title:            in.Title,
		Description:      in.Description,
		RealName:         in.RealName,
		Gender:           in.Gender,
		Type:             in.Type,
		Group:            in.Group,
		City:             in.City,
		Country:          in.Country,
		Province:         in.Province,
		CreatedAt:        in.CreatedAt.Format("2006-01-02 15:04:05"),
		WechatConfigID:   in.WechatConfigID,
		IsMust:           in.IsMust,
		WechatOpenIDMaps: UserWechatOpenIDMapsToPB(in.WechatOpenIDMaps),
		Age:              in.Age,
		Height:           in.Height,
		Weight:           in.Weight,
		IsVip:            in.IsVip,
		VipExpired:       utils.FormatTime(in.VipExpired),
	}
	user.IsVip = time.Until(in.VipExpired).Seconds() > 0
	if in.Tenant != nil {
		user.TenantName = in.Tenant.Name
	}
	return user
}

func UsersToPB(in []*User) []*apipb.UserInfo {
	var list []*apipb.UserInfo
	for _, user := range in {
		list = append(list, UserToPB(user))
	}
	return list
}

func PBToUserRoles(userRoles []*apipb.UserRole) []*UserRole {
	var list []*UserRole
	for _, userRole := range userRoles {
		list = append(list, &UserRole{
			Model: commonmodel.Model{
				ID: userRole.Id,
			},
			UserID: userRole.UserID,
			RoleID: userRole.RoleID,
		})
	}
	return list
}

func UserWechatOpenIDMapsToPB(maps []*UserWechatOpenIDMap) []*apipb.UserWechatOpenIDMap {
	var list []*apipb.UserWechatOpenIDMap
	for _, m := range maps {
		list = append(list, &apipb.UserWechatOpenIDMap{
			Id:             m.ID,
			UserID:         m.UserID,
			UnionID:        m.UnionID,
			OpenID:         m.OpenID,
			WechatConfigID: m.WechatConfigID,
		})
	}
	return list
}

func PBToUserWechatOpenIDMaps(maps []*apipb.UserWechatOpenIDMap) []*UserWechatOpenIDMap {
	var list []*UserWechatOpenIDMap
	for _, m := range maps {
		list = append(list, &UserWechatOpenIDMap{
			Model: commonmodel.Model{
				ID: m.Id,
			},
			UserID:         m.UserID,
			UnionID:        m.UnionID,
			OpenID:         m.OpenID,
			WechatConfigID: m.WechatConfigID,
		})
	}
	return list
}

func UserRolesToPB(userRoles []*UserRole) []*apipb.UserRole {
	var list []*apipb.UserRole
	for _, userRole := range userRoles {
		list = append(list, &apipb.UserRole{
			Id:     userRole.ID,
			UserID: userRole.UserID,
			RoleID: userRole.RoleID,
		})
	}
	return list
}

func PBToRole(in *apipb.RoleInfo) *Role {
	if in == nil {
		return nil
	}

	return &Role{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		TenantID:      in.TenantID,
		ProjectID:     in.ProjectID,
		Name:          in.Name,
		ParentID:      in.ParentID,
		DefaultRouter: in.DefaultRouter,
		Description:   in.Description,
		CanDel:        in.CanDel,
		RoleMenus:     PBToRoleMenus(in.RoleMenus),
		Public:        in.Public,
		IsMust:        in.IsMust,
	}
}

func RoleToPB(in *Role) *apipb.RoleInfo {
	if in == nil {
		return nil
	}
	var children []*apipb.RoleInfo
	//递归退出条件
	if len(in.Children) > 0 {
		children = RolesToPB(in.Children)
	}

	role := &apipb.RoleInfo{
		Id:            in.ID,
		TenantID:      in.TenantID,
		ProjectID:     in.ProjectID,
		Name:          in.Name,
		ParentID:      in.ParentID,
		DefaultRouter: in.DefaultRouter,
		Description:   in.Description,
		CanDel:        in.CanDel,
		RoleMenus:     RoleMenusToPB(in.RoleMenus),
		Children:      children,
		Public:        in.Public,
		IsMust:        in.IsMust,
	}
	if in.Tenant != nil {
		role.TenantName = in.Tenant.Name
	}
	return role
}

func RolesToPB(in []*Role) []*apipb.RoleInfo {
	var list []*apipb.RoleInfo
	for _, role := range in {
		list = append(list, RoleToPB(role))
	}
	return list
}

func PBToRoleMenus(roleMenus []*apipb.RoleMenu) []*RoleMenu {
	var list []*RoleMenu
	for _, roleMenu := range roleMenus {
		list = append(list, &RoleMenu{
			Model: commonmodel.Model{
				ID: roleMenu.Id,
			},
			RoleID: roleMenu.RoleID,
			MenuID: roleMenu.MenuID,
			Funcs:  roleMenu.Funcs,
			Show:   roleMenu.Show,
			Menu:   PBToMenu(roleMenu.Menu),
		})
	}
	return list
}

func RoleMenusToPB(roleMenus []*RoleMenu) []*apipb.RoleMenu {
	var list []*apipb.RoleMenu
	for _, roleMenu := range roleMenus {
		list = append(list, &apipb.RoleMenu{
			Id:     roleMenu.ID,
			RoleID: roleMenu.RoleID,
			MenuID: roleMenu.MenuID,
			Funcs:  roleMenu.Funcs,
			Show:   roleMenu.Show,
			Menu:   MenuToPB(roleMenu.Menu),
		})
	}
	return list
}

func UserProfileToUser(in *apipb.UserProfile) *User {
	if in == nil {
		return nil
	}
	return &User{
		TenantModel: commonmodel.TenantModel{
			Model: commonmodel.Model{
				ID: in.Id,
			},
		},
		Nickname:    in.Nickname,
		Email:       in.Email,
		Mobile:      in.Mobile,
		IDCard:      in.IdCard,
		Avatar:      in.Avatar,
		RealName:    in.RealName,
		Gender:      in.Gender,
		Country:     in.Country,
		Province:    in.Province,
		City:        in.City,
		County:      in.County,
		EID:         in.Eid,
		Birthday:    in.Birthday,
		Description: in.Description,
		UserName:    in.UserName,
	}
}
