package provider

import (
	"context"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type UserProvider struct {
	apipb.UnimplementedUserServer
}

func (u *UserProvider) LoginByStaffNo(ctx context.Context, in *apipb.LoginByStaffNoRequest) (*apipb.LoginByStaffNoResponse, error) {
	resp := &apipb.LoginByStaffNoResponse{
		Code: commonmodel.Success,
	}
	model.LoginByStaffNo(in, resp)
	return resp, nil
}

func (u *UserProvider) LogoutByUserName(ctx context.Context, in *apipb.LogoutByUserNameRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}

	model.LogoutByUserName(in, resp)

	return resp, nil
}

func (u *UserProvider) Login(ctx context.Context, in *apipb.LoginRequest) (*apipb.LoginResponse, error) {
	resp := &apipb.LoginResponse{
		Code: commonmodel.Success,
	}
	model.Login(in, resp)
	return resp, nil
}

func (u *UserProvider) Add(ctx context.Context, in *apipb.UserInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	user := model.PBToUser(in)
	user.Password = in.Password
	err := model.CreateUser(user, false)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *UserProvider) Update(ctx context.Context, in *apipb.UserInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := model.UpdateUser(model.PBToUser(in))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *UserProvider) Delete(ctx context.Context, in *apipb.DelRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := model.DeleteUser(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *UserProvider) Query(ctx context.Context, in *apipb.QueryUserRequest) (*apipb.QueryUserResponse, error) {
	resp := &apipb.QueryUserResponse{
		Code: commonmodel.Success,
	}
	model.QueryUser(in, resp, false)
	return resp, nil
}

func (u *UserProvider) GetProfile(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetProfileResponse, error) {
	resp := &apipb.GetProfileResponse{
		Code: commonmodel.Success,
	}
	//TODO
	user, err := model.GetUserProfile(in.Id, true)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	resp.Data = user
	return resp, nil
}

func (u *UserProvider) UpdateProfile(ctx context.Context, in *apipb.UserProfile) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := model.UpdateProfile(model.UserProfileToUser(in), false)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *UserProvider) UpdateProfileAndUserName(ctx context.Context, in *apipb.UserProfile) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := model.UpdateProfile(model.UserProfileToUser(in), true)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *UserProvider) Enable(ctx context.Context, in *apipb.EnableRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := model.EnableUser(in.Id, in.Enable)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *UserProvider) GetAll(ctx context.Context, in *apipb.GetAllUsersRequest) (*apipb.GetAllUsersResponse, error) {
	resp := &apipb.GetAllUsersResponse{
		Code: commonmodel.Success,
	}
	users, err := model.GetAllUsers(in)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.UsersToPB(users)
	}

	return resp, nil
}

func (u *UserProvider) GetDetail(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetUserDetailResponse, error) {
	resp := &apipb.GetUserDetailResponse{
		Code: commonmodel.Success,
	}
	user, err := model.GetUserById(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	resp.Data = model.UserToPB(&user)
	return resp, nil
}

func (u *UserProvider) ResetPwd(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := model.ResetPwd(in.Id, model.DefaultPwd)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *UserProvider) ChangePwd(ctx context.Context, in *apipb.ChangePwdRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	err := model.UpdatePwd(in.Id, in.OldPwd, in.NewPwd)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *UserProvider) Logout(ctx context.Context, in *apipb.LogoutRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	//TODO
	err := model.Logout(in.Token)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *UserProvider) StatisticCount(ctx context.Context, in *apipb.StatisticUserCountRequest) (*apipb.StatisticCountResponse, error) {
	resp := &apipb.StatisticCountResponse{
		Code: commonmodel.Success,
	}
	count, err := model.StatisticUserCount(int(in.Type), in.TenantID, in.Group)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Count = int32(count)
	}
	return resp, nil
}

func (u *UserProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	model.ExportAllUsers(in, resp)

	return resp, nil
}

func (u *UserProvider) UpdateBasics(ctx context.Context, in *apipb.BasicsInfo) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	i := &model.User{
		TenantModel: commonmodel.TenantModel{
			Model: commonmodel.Model{
				ID: in.Id,
			},
		},
		Gender:   in.Gender,
		Age:      in.Age,
		Nickname: in.Nickname,
		Height:   in.Height,
	}
	err := model.UpdateBasics(i)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}

func (u *UserProvider) GetBasics(ctx context.Context, in *apipb.GetDetailRequest) (*apipb.GetBasicsResponse, error) {
	resp := &apipb.GetBasicsResponse{
		Code: commonmodel.Success,
	}
	user, err := model.GetUserById(in.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	resp.Data = &apipb.BasicsInfo{Id: user.ID, Gender: user.Gender, Age: user.Age, Nickname: user.Nickname, Height: user.Height}
	return resp, nil
}

func (u *UserProvider) UpdateUserAgeHeightWeight(ctx context.Context, in *apipb.UpdateUserAgeHeightWeightRequest) (*apipb.CommonResponse, error) {
	resp := &apipb.CommonResponse{
		Code: commonmodel.Success,
	}
	i := &model.User{
		TenantModel: commonmodel.TenantModel{
			Model: commonmodel.Model{
				ID: in.Id,
			},
		},
		Age:    in.Age,
		Height: in.Height,
		Weight: in.Weight,
	}
	err := model.UpdateUserAgeHeightWeight(i)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	return resp, nil
}
