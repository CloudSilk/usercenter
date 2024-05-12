package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type FormComponentProvider struct {
	apipb.UnimplementedFormComponentServer
}

func (u *FormComponentProvider) Add(ctx context.Context, in *apipb.FormComponentInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	id, err := model.CreateFormComponent(model.PBToFormComponent(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	return resp, nil
}

func (u *FormComponentProvider) Update(ctx context.Context, in *apipb.FormComponentInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.UpdateFormComponent(model.PBToFormComponent(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *FormComponentProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.DeleteFormComponent(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *FormComponentProvider) Query(ctx context.Context, in *apipb.QueryFormComponentRequest) (*apipb.QueryFormComponentResponse, error) {
	resp := &apipb.QueryFormComponentResponse{
		Code: apipb.Code_Success,
	}
	model.QueryFormComponent(in, resp, false)
	return resp, nil
}

func (u *FormComponentProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetFormComponentDetailResponse, error) {
	resp := &apipb.GetFormComponentDetailResponse{
		Code: apipb.Code_Success,
	}
	f, err := model.GetFormComponentByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.FormComponentToPB(f)
	}
	return resp, nil
}
