package provider

import (
	"context"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/tenant"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type RoleProvider struct {
	apipb.UnimplementedRoleServer
}

func (u *RoleProvider) Add(ctx context.Context, in *apipb.RoleInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := permission.CreateRole(permission.PBToRole(in), tenant.GetTenantUserCount)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *RoleProvider) Update(ctx context.Context, in *apipb.RoleInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := permission.UpdateRole(permission.PBToRole(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *RoleProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := permission.DeleteRole(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *RoleProvider) Query(ctx context.Context, in *apipb.QueryRoleRequest) (*apipb.QueryRoleResponse, error) {
	resp := &apipb.QueryRoleResponse{
		Code: commonmodel.Success,
	}
	permission.QueryRole(in, resp, false)
	return resp, nil
}

func (u *RoleProvider) GetAll(ctx context.Context, in *apipb.GetAllRoleRequest) (*apipb.GetAllRoleResponse, error) {
	resp := &apipb.GetAllRoleResponse{
		Code: commonmodel.Success,
	}
	roles, err := permission.GetAllRole(in.TenantID, in.ContainerComm)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = permission.RolesToPB(roles)
	}

	return resp, nil
}

func (u *RoleProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetRoleDetailResponse, error) {
	resp := &apipb.GetRoleDetailResponse{
		Code: commonmodel.Success,
	}
	role, err := permission.GetRoleByID(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	resp.Data = permission.RoleToPB(role)
	return resp, nil
}

func (u *RoleProvider) StatisticCount(ctx context.Context, in *apipb.StatisticRoleCountRequest) (*apipb.StatisticCountResponse, error) {
	resp := &apipb.StatisticCountResponse{
		Code: commonmodel.Success,
	}
	count, err := permission.StatisticRoleCount(in.TenantID)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Count = int32(count)
	}
	return resp, nil
}

func (u *RoleProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	permission.ExportAllRoles(in, resp)

	return resp, nil
}
