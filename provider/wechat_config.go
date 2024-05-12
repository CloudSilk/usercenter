package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type WechatConfigProvider struct {
	apipb.UnimplementedWechatConfigServer
}

func (u *WechatConfigProvider) Add(ctx context.Context, in *apipb.WechatConfigInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	id, err := model.CreateWechatConfig(model.PBToWechatConfig(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	return resp, nil
}

func (u *WechatConfigProvider) Update(ctx context.Context, in *apipb.WechatConfigInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.UpdateWechatConfig(model.PBToWechatConfig(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *WechatConfigProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.DeleteWechatConfig(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *WechatConfigProvider) Query(ctx context.Context, in *apipb.QueryWechatConfigRequest) (*apipb.QueryWechatConfigResponse, error) {
	resp := &apipb.QueryWechatConfigResponse{
		Code: apipb.Code_Success,
	}
	model.QueryWechatConfig(in, resp, false)
	return resp, nil
}

func (u *WechatConfigProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetWechatConfigDetailResponse, error) {
	resp := &apipb.GetWechatConfigDetailResponse{
		Code: apipb.Code_Success,
	}
	f, err := model.GetWechatConfigByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.WechatConfigToPB(f)
	}
	return resp, nil
}

func (u *WechatConfigProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	model.ExportAllWechatConfigs(in, resp)

	return resp, nil
}
