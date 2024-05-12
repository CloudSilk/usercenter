package provider

import (
	"context"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type MenuProvider struct {
	apipb.UnimplementedMenuServer
}

func (u *MenuProvider) Add(ctx context.Context, in *apipb.MenuInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := model.AddMenu(model.PBToMenu(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *MenuProvider) Update(ctx context.Context, in *apipb.MenuInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := model.UpdateMenu(model.PBToMenu(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *MenuProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := model.DeleteMenu(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *MenuProvider) Query(ctx context.Context, in *apipb.QueryMenuRequest) (*apipb.QueryMenuResponse, error) {
	resp := &apipb.QueryMenuResponse{
		Code: commonmodel.Success,
	}
	model.QueryMenu(in, resp, false)
	return resp, nil
}

func (u *MenuProvider) GetAll(ctx context.Context, in *apipb.QueryMenuRequest) (*apipb.GetAllMenuResponse, error) {
	resp := &apipb.GetAllMenuResponse{
		Code: commonmodel.Success,
	}
	menus, err := model.GetAllMenus(in)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.MenusToPB(menus)
	}

	return resp, nil
}

func (u *MenuProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetMenuDetailResponse, error) {
	resp := &apipb.GetMenuDetailResponse{
		Code: commonmodel.Success,
	}
	menu, err := model.GetMenuByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	resp.Data = model.MenuToPB(menu)
	return resp, nil
}

func (u *MenuProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	model.ExportAllMenus(in, resp)

	return resp, nil
}
