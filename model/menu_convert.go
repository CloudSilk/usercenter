package model

import (
	commonmodel "github.com/CloudSilk/pkg/model"
	apipb "github.com/CloudSilk/usercenter/proto"
)

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
	//递归退出条件
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
