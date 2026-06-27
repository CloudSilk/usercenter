package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/internal/permission"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type MenuProvider struct {
	apipb.UnimplementedMenuServer
}

func (u *MenuProvider) Add(ctx context.Context, in *apipb.MenuInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := permission.AddMenu(permission.PBToMenu(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *MenuProvider) Update(ctx context.Context, in *apipb.MenuInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := permission.UpdateMenu(permission.PBToMenu(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *MenuProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := permission.DeleteMenu(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *MenuProvider) Query(ctx context.Context, in *apipb.QueryMenuRequest) (*apipb.QueryMenuResponse, error) {
	resp := &apipb.QueryMenuResponse{
		Code: apipb.Code_Success,
	}
	permission.QueryMenu(in, resp, false)
	return resp, nil
}

func (u *MenuProvider) GetAll(ctx context.Context, in *apipb.QueryMenuRequest) (*apipb.GetAllMenuResponse, error) {
	resp := &apipb.GetAllMenuResponse{
		Code: apipb.Code_Success,
	}
	menus, err := permission.GetAllMenus(in)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = permission.MenusToPB(menus)
	}

	return resp, nil
}

func (u *MenuProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetMenuDetailResponse, error) {
	resp := &apipb.GetMenuDetailResponse{
		Code: apipb.Code_Success,
	}
	menu, err := permission.GetMenuByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	resp.Data = permission.MenuToPB(menu)
	return resp, nil
}

func (u *MenuProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	permission.ExportAllMenus(in, resp)

	return resp, nil
}
