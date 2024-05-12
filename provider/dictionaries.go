package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type DictionariesProvider struct {
	apipb.UnimplementedDictionariesServer
}

func (u *DictionariesProvider) Add(ctx context.Context, in *apipb.DictionariesInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	id, err := model.CreateDictionaries(model.PBToDictionariesArray(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	return resp, nil
}

func (u *DictionariesProvider) Update(ctx context.Context, in *apipb.DictionariesInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.UpdateDictionaries(model.PBToDictionariesArray(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *DictionariesProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.DeleteDictionaries(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *DictionariesProvider) Query(ctx context.Context, in *apipb.QueryDictionariesRequest) (*apipb.QueryDictionariesResponse, error) {
	resp := &apipb.QueryDictionariesResponse{
		Code: apipb.Code_Success,
	}
	model.QueryDictionaries(in, resp, false)
	return resp, nil
}

func (u *DictionariesProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetDictionariesDetailResponse, error) {
	resp := &apipb.GetDictionariesDetailResponse{
		Code: apipb.Code_Success,
	}
	f, err := model.GetDictionariesByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.DictionariesToPB(f)
	}
	return resp, nil
}

func (u *DictionariesProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	model.ExportAllDictionaries(in, resp)

	return resp, nil
}
