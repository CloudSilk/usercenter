package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/internal/dictionaries"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type DictionariesProvider struct {
	apipb.UnimplementedDictionariesServer
}

func (u *DictionariesProvider) Add(ctx context.Context, in *apipb.DictionariesInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	id, err := dictionaries.CreateDictionaries(dictionaries.PBToDictionariesArray(in))
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
	err := dictionaries.UpdateDictionaries(dictionaries.PBToDictionariesArray(in))
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
	err := dictionaries.DeleteDictionaries(in.Id)
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
	dictionaries.QueryDictionaries(in, resp, false)
	return resp, nil
}

func (u *DictionariesProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetDictionariesDetailResponse, error) {
	resp := &apipb.GetDictionariesDetailResponse{
		Code: apipb.Code_Success,
	}
	f, err := dictionaries.GetDictionariesByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = dictionaries.DictionariesToPB(f)
	}
	return resp, nil
}

func (u *DictionariesProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	dictionaries.ExportAllDictionaries(in, resp)

	return resp, nil
}
