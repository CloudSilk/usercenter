package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type ProjectProvider struct {
	apipb.UnimplementedProjectServer
}

func (u *ProjectProvider) Add(ctx context.Context, in *apipb.ProjectInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	id, err := model.CreateProject(model.PBToProject(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	return resp, nil
}

func (u *ProjectProvider) Update(ctx context.Context, in *apipb.ProjectInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.UpdateProject(model.PBToProject(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *ProjectProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := model.DeleteProject(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *ProjectProvider) Query(ctx context.Context, in *apipb.QueryProjectRequest) (*apipb.QueryProjectResponse, error) {
	resp := &apipb.QueryProjectResponse{
		Code: apipb.Code_Success,
	}
	model.QueryProject(in, resp, false)
	return resp, nil
}

func (u *ProjectProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetProjectDetailResponse, error) {
	resp := &apipb.GetProjectDetailResponse{
		Code: apipb.Code_Success,
	}
	f, err := model.GetProjectByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.ProjectToPB(f)
	}
	return resp, nil
}

func (u *ProjectProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	model.ExportAllProjects(in, resp)

	return resp, nil
}
