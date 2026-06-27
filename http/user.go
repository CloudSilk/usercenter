package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/CloudSilk/pkg/constants"
	cmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/alert"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/utils/middleware"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Login godoc
// @Summary 登录
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param data body apipb.LoginRequest true "登录信息"
// @Success 200 {object} apipb.LoginResponse
// @Router /api/core/auth/user/login [post]
func Login(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.LoginRequest{}
	resp := &apipb.LoginResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建User请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	user.Login(req, resp)
	// 登录失败时触发安全告警（基于 IP 的暴力破解检测）
	if resp.Code == 41001 {
		alert.AlertLoginFailure(req.UserName, c.ClientIP())
	}

	c.JSON(http.StatusOK, resp)
}

// MFALoginVerify 完成 MFA 二阶段登录。
// body: {"mfaToken": "...", "code": "123456"}。成功返回 access_token（resp.Data）。
func MFALoginVerify(c *gin.Context) {
	var req struct {
		MFAToken string `json:"mfaToken" binding:"required"`
		Code     string `json:"code" binding:"required"`
	}
	resp := &apipb.LoginResponse{Code: apipb.Code_Success}
	if err := c.BindJSON(&req); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	user.CompleteMFALogin(req.MFAToken, req.Code, resp)
	c.JSON(http.StatusOK, resp)
}

// Profile godoc
// @Summary 获取个人信息
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param returnMenu query bool false "返回Menu"
// @Success 200 {object} apipb.UserProfile
// @Router /api/core/auth/user/profile [get]
func Profile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	returnMenu := c.Query("returnMenu")
	userProfile, err := user.GetUserProfile(userID, returnMenu == "" || returnMenu == "true")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    apipb.Code_InternalServerError,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": apipb.Code_Success,
		"data": userProfile,
	})
}

// UpdateProfile godoc
// @Summary 更新个人信息
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param data body apipb.UserProfile true "个人信息"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/profile [put]
func UpdateProfile(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.UserProfile{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,更新个人信息请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}

	err = user.UpdateProfile(user.UserProfileToUser(req), false)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// AddUser godoc
// @Summary 新增用户
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param data body apipb.UserInfo true "用户信息"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/add [post]
func AddUser(c *gin.Context, req *apipb.UserInfo) (*apipb.CommonResponse, error) {
	tenantID := middleware.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	if err := user.CreateUser(user.PBToUser(req), false); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// AddUserHandler 泛型路由注册入口
var AddUserHandler = AutoHandler(AddUser)

// UpdateUser godoc
// @Summary 更新用户
// @Tags 用户管理
// @Param data body apipb.UserInfo true "用户信息"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/update [put]
func UpdateUser(c *gin.Context, req *apipb.UserInfo) (*apipb.CommonResponse, error) {
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	if err := user.UpdateUser(user.PBToUser(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteUser godoc
// @Summary 删除用户
// @Tags 用户管理
// @Param data body apipb.DelRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/delete [delete]
func DeleteUser(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	if err := user.DeleteUser(req.Id); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	audit.RecordAuditWithKind(store.DB(), middleware.GetUserID(c), middleware.GetUserName(c), int32(middleware.GetPrincipalKind(c)), audit.AuditActionDeleteUser, req.Id, c.ClientIP(), "")
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// EnableUser godoc
// @Summary 禁用/启用用户
// @Tags 用户管理
// @Param data body apipb.EnableRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/enable [post]
func EnableUser(c *gin.Context, req *apipb.EnableRequest) (*apipb.CommonResponse, error) {
	if err := user.EnableUser(req.Id, req.Enable); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QueryUser godoc
// @Summary 分页查询
// @Tags 用户管理
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QueryUserResponse
// @Router /api/core/auth/user/query [get]
func QueryUser(c *gin.Context, req *apipb.QueryUserRequest) (*apipb.QueryUserResponse, error) {
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	resp := &apipb.QueryUserResponse{Code: apipb.Code_Success}
	user.QueryUser(req, resp, false)
	return resp, nil
}

// GetAllUsers godoc
// @Summary 查询所有用户
// @Tags 用户管理
// @Param type query int false "用户类型"
// @Param tenantID query string false "租户ID"
// @Param group query string false "分组ID"
// @Success 200 {object} apipb.GetAllUsersResponse
// @Router /api/core/auth/user/all [get]
func GetAllUsers(c *gin.Context) {
	resp := &apipb.GetAllUsersResponse{
		Code: apipb.Code_Success,
	}
	req := &apipb.GetAllUsersRequest{}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	tenantID := middleware.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	users, err := user.GetAllUsers(req)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = user.UsersToPB(users)
	c.JSON(http.StatusOK, resp)
}

// GetUserDetail godoc
// @Summary 查询明细
// @Tags 用户管理
// @Param id query string true "用户ID"
// @Success 200 {object} apipb.GetUserDetailResponse
// @Router /api/core/auth/user/detail [get]
func GetUserDetail(c *gin.Context) {
	resp := &apipb.GetUserDetailResponse{
		Code: apipb.Code_Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}

	data, err := user.GetUserById(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = user.UserToPB(&data)
	}
	c.JSON(http.StatusOK, resp)
}

// ResetPwd godoc
// @Summary 重置密码
// @Tags 用户管理
// @Param data body apipb.GetDetailRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/resetpwd [post]
func ResetPwd(c *gin.Context) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	req := &apipb.GetDetailRequest{}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	// 越权校验：非平台租户只能重置本租户用户的密码
	tenantID := middleware.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		userTenantID, err := user.GetUserTenantID(req.Id)
		if err != nil || userTenantID != tenantID {
			resp.Code = 41003 // NoPermission
			resp.Message = "无权重置该用户密码"
			c.JSON(http.StatusOK, resp)
			return
		}
	}
	err = user.ResetPwd(req.Id, user.DefaultPwd)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		audit.RecordAuditWithKind(store.DB(), middleware.GetUserID(c), middleware.GetUserName(c), int32(middleware.GetPrincipalKind(c)), audit.AuditActionResetPwd, req.Id, c.ClientIP(), "")
	}
	c.JSON(http.StatusOK, resp)
}

// ChangePwd godoc
// @Summary 修改密码
// @Tags 用户管理
// @Param data body apipb.ChangePwdRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/changepwd [post]
func ChangePwd(c *gin.Context) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	req := &apipb.ChangePwdRequest{}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	if req.NewPwd != req.NewConfirmPwd {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "新密码和确认密码不一样"
		c.JSON(http.StatusOK, resp)
		return
	}
	req.Id = middleware.GetUserID(c)
	err = user.UpdatePwd(req.Id, req.OldPwd, req.NewPwd)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// Logout godoc
// @Summary 退出登录
// @Tags 用户管理
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/logout [post]
func Logout(c *gin.Context) {
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	t := middleware.GetAccessToken(c)
	err := user.Logout(t)
	if err != nil {
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// ExportUser godoc
// @Summary 导出
// @Tags 用户管理
// @Success 200 {object} apipb.CommonResponse
// @Router /api/auth/user/export [get]
func ExportUser(c *gin.Context) {
	req := &apipb.QueryUserRequest{}
	resp := &apipb.QueryUserResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	tenantID := ucm.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	req.PageIndex = 1
	req.PageSize = 1000
	user.QueryUser(req, resp, true)
	if resp.Code != apipb.Code_Success {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment;filename=User.json")
	c.Header("Content-Transfer-Encoding", "binary")
	buf, _ := json.Marshal(resp.Data)
	c.Writer.Write(buf)
}

// ImportUser godoc
// @Summary 导入
// @Tags 用户管理
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/auth/user/import [post]
func ImportUser(c *gin.Context) {
	resp := &apipb.QueryUserResponse{
		Code: apipb.Code_Success,
	}
	file, _, err := c.Request.FormFile("files")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	defer file.Close()
	buf, err := io.ReadAll(file)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	var list []*apipb.UserInfo
	err = json.Unmarshal(buf, &list)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	successCount := 0
	failCount := 0
	for _, f := range list {
		err = user.UpdateUser(user.PBToUser(f))
		if err == gorm.ErrRecordNotFound {
			err = user.CreateUser(user.PBToUser(f), false)
		}
		if err != nil {
			failCount++
		} else {
			successCount++
		}
	}
	resp.Message = fmt.Sprintf("导入成功数量:%d,导入失败数量:%d", successCount, failCount)
	c.JSON(http.StatusOK, resp)
}

// UpdateBasicsByToken godoc
// @Summary 根据token更新
// @Tags 用户管理
// @Param data body apipb.BasicsInfo true "用户信息"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/token/update [put]
func UpdateBasicsByToken(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.BasicsInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,更新客户请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}

	info := &user.User{
		TenantModel: cmodel.TenantModel{
			Model: cmodel.Model{
				ID: ucm.GetUserID(c),
			},
		},
		Gender:   req.Gender,
		Age:      req.Age,
		Nickname: req.Nickname,
		Height:   req.Height,
	}

	if err := user.UpdateBasics(info); err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// GetBasicsByToken godoc
// @Summary 根据token查询明细
// @Tags 用户管理
// @Success 200 {object} apipb.GetBasicsResponse
// @Router /api/core/auth/user/token/detail [get]
func GetBasicsByToken(c *gin.Context) {
	resp := &apipb.GetBasicsResponse{
		Code: apipb.Code_Success,
	}

	if u, err := user.GetUserById(ucm.GetUserID(c)); err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = &apipb.BasicsInfo{Id: u.ID, Gender: u.Gender, Age: u.Age, Nickname: u.Nickname, Height: u.Height}
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterUserRouter(r *gin.Engine) {
	userGroup := r.Group("/api/core/auth/user")
	userGroup.POST("login", Login)
	userGroup.POST("mfa/verify", MFALoginVerify)
	userGroup.POST("logout", Logout)
	userGroup.GET("profile", Profile)
	userGroup.PUT("profile", UpdateProfile)
	userGroup.POST("add", AddUserHandler)
	userGroup.PUT("update", AutoHandler(UpdateUser))
	userGroup.GET("query", AutoQueryHandler(QueryUser))
	userGroup.DELETE("delete", AutoHandler(DeleteUser))
	userGroup.POST("enable", AutoHandler(EnableUser))
	userGroup.GET("all", GetAllUsers)
	userGroup.GET("detail", GetUserDetail)
	userGroup.POST("resetpwd", ResetPwd)
	userGroup.POST("changepwd", ChangePwd)
	userGroup.GET("export", ExportUser)
	userGroup.POST("import", ImportUser)
	userGroup.PUT("token/update", UpdateBasicsByToken)
	userGroup.GET("token/detail", GetBasicsByToken)
}
