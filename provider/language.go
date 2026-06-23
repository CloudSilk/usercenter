package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/internal/language"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type LanguageProvider struct {
	apipb.UnimplementedLanguageServer
}

func (u *LanguageProvider) Add(ctx context.Context, in *apipb.LanguageInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	id, err := language.CreateLanguage(language.PBToLanguage(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	return resp, nil
}

func (u *LanguageProvider) Update(ctx context.Context, in *apipb.LanguageInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := language.UpdateLanguage(language.PBToLanguage(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *LanguageProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := language.DeleteLanguage(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *LanguageProvider) Query(ctx context.Context, in *apipb.QueryLanguageRequest) (*apipb.QueryLanguageResponse, error) {
	resp := &apipb.QueryLanguageResponse{
		Code: apipb.Code_Success,
	}
	language.QueryLanguage(in, resp, false)
	return resp, nil
}

func (u *LanguageProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetLanguageDetailResponse, error) {
	resp := &apipb.GetLanguageDetailResponse{
		Code: apipb.Code_Success,
	}
	f, err := language.GetLanguageByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = language.LanguageToPB(f)
	}
	return resp, nil
}

func (u *LanguageProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	language.ExportAllLanguages(in, resp)

	return resp, nil
}
