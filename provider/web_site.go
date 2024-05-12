package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type WebSiteProvider struct {
	apipb.UnimplementedWebSiteServer
}

func (u *WebSiteProvider) Add(ctx context.Context, in *apipb.WebSiteInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	id, err := model.CreateWebSite(model.PBToWebSite(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	return resp, nil
}

func (u *WebSiteProvider) Update(ctx context.Context, in *apipb.WebSiteInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.UpdateWebSite(model.PBToWebSite(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *WebSiteProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.DeleteWebSite(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *WebSiteProvider) Query(ctx context.Context, in *apipb.QueryWebSiteRequest) (*apipb.QueryWebSiteResponse, error) {
	resp := &apipb.QueryWebSiteResponse{
		Code: apipb.Code_Success,
	}
	model.QueryWebSite(in, resp, false)
	return resp, nil
}

func (u *WebSiteProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetWebSiteDetailResponse, error) {
	resp := &apipb.GetWebSiteDetailResponse{
		Code: apipb.Code_Success,
	}
	f, err := model.GetWebSiteByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.WebSiteToPB(f)
	}
	return resp, nil
}

func (u *WebSiteProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	model.ExportAllWebSites(in, resp)

	return resp, nil
}
