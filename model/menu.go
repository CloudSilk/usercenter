package model

import (
	"github.com/CloudSilk/usercenter/internal/menu"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type Menu = menu.Menu
type MenuParameter = menu.MenuParameter
type MenuFunc = menu.MenuFunc
type MenuFuncApi = menu.MenuFuncApi

func AddMenu(m *Menu) error                        { return menu.AddMenu(m) }
func DeleteMenu(id string) error                   { return menu.DeleteMenu(id) }
func UpdateMenu(m *Menu) error                     { return menu.UpdateMenu(m) }
func GetMenuByID(id string) (*Menu, error)         { return menu.GetMenuByID(id) }
func QueryMenu(req *apipb.QueryMenuRequest, resp *apipb.QueryMenuResponse, preload bool) {
	menu.QueryMenu(req, resp, preload)
}
func GetAllMenus(req *apipb.QueryMenuRequest) ([]*Menu, error) { return menu.GetAllMenus(req) }
func ExportAllMenus(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	menu.ExportAllMenus(req, resp)
}
func PBToMenu(in *apipb.MenuInfo) *Menu             { return menu.PBToMenu(in) }
func MenuToPB(in *Menu) *apipb.MenuInfo             { return menu.MenuToPB(in) }
func MenusToPB(in []*Menu) []*apipb.MenuInfo        { return menu.MenusToPB(in) }
func PBToMenuParameters(in []*apipb.MenuParameter) []*MenuParameter { return menu.PBToMenuParameters(in) }
func MenuParametersToPB(in []*MenuParameter) []*apipb.MenuParameter { return menu.MenuParametersToPB(in) }
func PBToMenuFuncs(in []*apipb.MenuFunc) []*MenuFunc { return menu.PBToMenuFuncs(in) }
func MenuFuncsToPB(in []*MenuFunc) []*apipb.MenuFunc { return menu.MenuFuncsToPB(in) }
func PBToMenuFuncApis(in []*apipb.MenuFuncApi) []MenuFuncApi { return menu.PBToMenuFuncApis(in) }
func MenuFuncApisToPB(in []MenuFuncApi) []*apipb.MenuFuncApi { return menu.MenuFuncApisToPB(in) }
