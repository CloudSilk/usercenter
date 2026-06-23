package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/internal/website"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type WebSiteProvider struct {
	apipb.UnimplementedWebSiteServer
}

func (u *WebSiteProvider) Add(ctx context.Context, in *apipb.WebSiteInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	id, err := website.CreateWebSite(website.PBToWebSite(in))
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
	err := website.UpdateWebSite(website.PBToWebSite(in))
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
	err := website.DeleteWebSite(in.Id)
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
	website.QueryWebSite(in, resp, false)
	return resp, nil
}

func (u *WebSiteProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetWebSiteDetailResponse, error) {
	resp := &apipb.GetWebSiteDetailResponse{
		Code: apipb.Code_Success,
	}
	f, err := website.GetWebSiteByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = website.WebSiteToPB(f)
	}
	return resp, nil
}

func (u *WebSiteProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	website.ExportAllWebSites(in, resp)

	return resp, nil
}
