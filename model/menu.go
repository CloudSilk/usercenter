package model

import (
	"github.com/CloudSilk/usercenter/internal/permission"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type Menu = permission.Menu
type MenuParameter = permission.MenuParameter
type MenuFunc = permission.MenuFunc
type MenuFuncApi = permission.MenuFuncApi

func AddMenu(m *Menu) error                        { return permission.AddMenu(m) }
func DeleteMenu(id string) error                   { return permission.DeleteMenu(id) }
func UpdateMenu(m *Menu) error                     { return permission.UpdateMenu(m) }
func GetMenuByID(id string) (*Menu, error)         { return permission.GetMenuByID(id) }
func QueryMenu(req *apipb.QueryMenuRequest, resp *apipb.QueryMenuResponse, preload bool) {
	permission.QueryMenu(req, resp, preload)
}
func GetAllMenus(req *apipb.QueryMenuRequest) ([]*Menu, error) { return permission.GetAllMenus(req) }
func ExportAllMenus(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	permission.ExportAllMenus(req, resp)
}
func PBToMenu(in *apipb.MenuInfo) *Menu             { return permission.PBToMenu(in) }
func MenuToPB(in *Menu) *apipb.MenuInfo             { return permission.MenuToPB(in) }
func MenusToPB(in []*Menu) []*apipb.MenuInfo        { return permission.MenusToPB(in) }
func PBToMenuParameters(in []*apipb.MenuParameter) []*MenuParameter { return permission.PBToMenuParameters(in) }
func MenuParametersToPB(in []*MenuParameter) []*apipb.MenuParameter { return permission.MenuParametersToPB(in) }
func PBToMenuFuncs(in []*apipb.MenuFunc) []*MenuFunc { return permission.PBToMenuFuncs(in) }
func MenuFuncsToPB(in []*MenuFunc) []*apipb.MenuFunc { return permission.MenuFuncsToPB(in) }
func PBToMenuFuncApis(in []*apipb.MenuFuncApi) []MenuFuncApi { return permission.PBToMenuFuncApis(in) }
func MenuFuncApisToPB(in []MenuFuncApi) []*apipb.MenuFuncApi { return permission.MenuFuncApisToPB(in) }
