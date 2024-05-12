package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type SystemConfigProvider struct {
	apipb.UnimplementedSystemConfigServer
}

func (u *SystemConfigProvider) Add(ctx context.Context, in *apipb.SystemConfigInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	id, err := model.CreateSystemConfig(model.PBToSystemConfig(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	return resp, nil
}

func (u *SystemConfigProvider) Update(ctx context.Context, in *apipb.SystemConfigInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.UpdateSystemConfig(model.PBToSystemConfig(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *SystemConfigProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.DeleteSystemConfig(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *SystemConfigProvider) Query(ctx context.Context, in *apipb.QuerySystemConfigRequest) (*apipb.QuerySystemConfigResponse, error) {
	resp := &apipb.QuerySystemConfigResponse{
		Code: apipb.Code_Success,
	}
	model.QuerySystemConfig(in, resp, false)
	return resp, nil
}

func (u *SystemConfigProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetSystemConfigDetailResponse, error) {
	resp := &apipb.GetSystemConfigDetailResponse{
		Code: apipb.Code_Success,
	}
	f, err := model.GetSystemConfigByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.SystemConfigToPB(f)
	}
	return resp, nil
}

func (u *SystemConfigProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	model.ExportAllSystemConfigs(in, resp)

	return resp, nil
}
