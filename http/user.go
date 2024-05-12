package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	ucmodel "github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/utils/middleware"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Login
// @Summary 登录
// @Description 登录
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param product body apipb.LoginRequest true "个人信息"
// @Success 200 {object} apipb.LoginResponse
// @Router /api/core/auth/user/login [post]
func Login(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.LoginRequest{}
	resp := &apipb.LoginResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建User请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	ucmodel.Login(req, resp)

	c.JSON(http.StatusOK, resp)
}

// Profile
// @Summary 获取个人信息
// @Description 获取个人信息
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param returnMenu query bool false "返回Menu"
// @Success 200 {object} apipb.UserProfile
// @Router /api/core/auth/user/profile [get]
func Profile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	returnMenu := c.Query("returnMenu")
	userProfile, err := ucmodel.GetUserProfile(userID, returnMenu == "" || returnMenu == "true")
	if err != nil {
		c.JSON(http.StatusOK, map[string]interface{}{
			"code":    model.InternalServerError,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{
		"code": model.Success,
		"data": userProfile,
	})
}

// UpdateProfile
// @Summary 更新个人信息
// @Description 更新个人信息
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param product body apipb.UserProfile true "个人信息"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/profile [put]
func UpdateProfile(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.UserProfile{}
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,更新个人信息请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}

	err = ucmodel.UpdateProfile(ucmodel.UserProfileToUser(req), false)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// AddUser godoc
// @Summary 新增用户
// @Description 新增用户
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.UserInfo true "用户信息"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/add [post]
func AddUser(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.UserInfo{}
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建User请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	//只有平台租户才能为其他租户创建用户
	tenantID := middleware.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	err = ucmodel.CreateUser(ucmodel.PBToUser(req), false)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateUser
// @Summary 更新用户
// @Description 更新用户
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.UserInfo true "用户信息"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/update [put]
func UpdateUser(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.UserInfo{}
	resp := &apipb.CommonResponse{
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
	//只有平台租户才能更改用户的租户
	tenantID := middleware.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	err = ucmodel.UpdateUser(ucmodel.PBToUser(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteUser
// @Summary 删除用户
// @Description 删除用户
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/delete [delete]
func DeleteUser(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.DelRequest{}
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建User请求参数无效:%v", transID, err)
		return
	}
	err = ucmodel.DeleteUser(req.Id)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// EnableUser
// @Summary 禁用/启用用户
// @Description 禁用/启用用户
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.EnableRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/enable [post]
func EnableUser(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.EnableRequest{}
	resp := &model.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建User请求参数无效:%v", transID, err)
		return
	}

	err = ucmodel.EnableUser(req.Id, req.Enable)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryUser
// @Summary 分页查询
// @Description 分页查询
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param userName query string false "用户名"
// @Param nickname query string false "昵称"
// @Param idCard query string false "身份证号"
// @Param mobile query string false "手机号"
// @Param title query string false "职位"
// @Param type query int false "用户类型,从1开始,为0时查询全部"
// @Param tenantID query string false "租户ID"
// @Param group query string false "分组ID，例如属于某个组织的，或者某个个人"
// @Success 200 {object} apipb.QueryUserResponse
// @Router /api/core/auth/user/query [get]
func QueryUser(c *gin.Context) {
	req := &apipb.QueryUserRequest{}
	resp := &apipb.QueryUserResponse{
		Code: model.Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	//只有平台租户才能查询其他租户的角色
	tenantID := middleware.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	ucmodel.QueryUser(req, resp, false)

	c.JSON(http.StatusOK, resp)
}

// GetAllUsers
// @Summary 查询所有用户
// @Description 查询所有用户
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param type query int false "用户类型,从1开始,为0时查询全部"
// @Param tenantID query string false "租户ID"
// @Param group query string false "分组ID，例如属于某个组织的，或者某个个人"
// @Success 200 {object} apipb.GetAllUsersResponse
// @Router /api/core/auth/user/all [get]
func GetAllUsers(c *gin.Context) {
	resp := &apipb.GetAllUsersResponse{
		Code: model.Success,
	}
	req := &apipb.GetAllUsersRequest{}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	//只有平台租户才能查询其他租户的角色
	tenantID := middleware.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	users, err := ucmodel.GetAllUsers(req)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = ucmodel.UsersToPB(users)
	c.JSON(http.StatusOK, resp)
}

// GetUserDetail
// @Summary 查询明细
// @Description 查询明细
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param id query string true "用户ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetUserDetailResponse
// @Router /api/core/auth/user/detail [get]
func GetUserDetail(c *gin.Context) {
	resp := &apipb.GetUserDetailResponse{
		Code: model.Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = model.BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := ucmodel.GetUserById(idStr)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = ucmodel.UserToPB(&data)
	}
	c.JSON(http.StatusOK, resp)
}

// ResetPwd
// @Summary 重置密码
// @Description 重置密码
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.GetDetailRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/resetpwd [post]
func ResetPwd(c *gin.Context) {
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	req := &apipb.GetDetailRequest{}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.ResetPwd(req.Id, ucmodel.DefaultPwd)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// ChangePwd
// @Summary 修改密码
// @Description 修改密码
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.ChangePwdRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/changepwd [post]
func ChangePwd(c *gin.Context) {
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	req := &apipb.ChangePwdRequest{}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	if req.NewPwd != req.NewConfirmPwd {
		resp.Code = model.BadRequest
		resp.Message = "新密码和确认密码不一样"
		c.JSON(http.StatusOK, resp)
		return
	}
	if req.Id == "" {
		req.Id = middleware.GetUserID(c)
	}
	err = ucmodel.UpdatePwd(req.Id, req.OldPwd, req.NewPwd)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// Logout
// @Summary 退出登录
// @Description 退出登录
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/user/logout [post]
func Logout(c *gin.Context) {
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	t := middleware.GetAccessToken(c)
	err := ucmodel.Logout(t)
	if err != nil {
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// ExportUser godoc
// @Summary 导出
// @Description 导出
// @Tags 用户管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param nickname query string false "Nickname"
// @Param userName query string false "UserName"
// @Param ids query []string false "IDs"
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
	ucmodel.QueryUser(req, resp, true)
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

// ImportUser
// @Summary 导入
// @Description 导入
// @Tags 用户管理
// @Accept  mpfd
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/auth/user/import [post]
func ImportUser(c *gin.Context) {
	resp := &apipb.QueryUserResponse{
		Code: apipb.Code_Success,
	}
	//从用户中读取文件
	file, fileHeader, err := c.Request.FormFile("files")
	if err != nil {
		fmt.Println(err)
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	//defer 结束时关闭文件
	defer file.Close()
	fmt.Println("filename: " + fileHeader.Filename)
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	var list []*apipb.UserInfo
	err = json.Unmarshal(buf, &list)
	if err != nil {
		fmt.Println(err)
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	successCount := 0
	failCount := 0
	for _, f := range list {
		err = ucmodel.UpdateUser(ucmodel.PBToUser(f))
		if err == gorm.ErrRecordNotFound {
			err = ucmodel.CreateUser(ucmodel.PBToUser(f), false)
		}
		if err != nil {
			failCount++
			fmt.Println(err)
		} else {
			successCount++
		}
	}
	resp.Message = fmt.Sprintf("导入成功数量:%d,导入失败数量:%d", successCount, failCount)
	c.JSON(http.StatusOK, resp)
}

// UpdateBasicsByToken godoc
// @Summary 根据token更新
// @Description 根据token更新
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.BasicsInfo true "Update Customer"
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

	info := &ucmodel.User{
		TenantModel: model.TenantModel{
			Model: model.Model{
				ID: ucm.GetUserID(c),
			},
		},
		Gender:   req.Gender,
		Age:      req.Age,
		Nickname: req.Nickname,
		Height:   req.Height,
	}

	if err := ucmodel.UpdateBasics(info); err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// GetBasicsByToken godoc
// @Summary 根据token查询明细
// @Description 根据token查询明细
// @Tags 用户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetBasicsResponse
// @Router /api/core/auth/user/token/detail [get]
func GetBasicsByToken(c *gin.Context) {
	resp := &apipb.GetBasicsResponse{
		Code: apipb.Code_Success,
	}

	if user, err := ucmodel.GetUserById(ucm.GetUserID(c)); err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = &apipb.BasicsInfo{Id: user.ID, Gender: user.Gender, Age: user.Age, Nickname: user.Nickname, Height: user.Height}
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterUserRouter(r *gin.Engine) {
	userGroup := r.Group("/api/core/auth/user")
	userGroup.POST("login", Login)
	userGroup.POST("logout", Logout)
	userGroup.GET("profile", Profile)
	userGroup.PUT("profile", UpdateProfile)
	userGroup.POST("add", AddUser)
	userGroup.PUT("update", UpdateUser)
	userGroup.GET("query", QueryUser)
	userGroup.DELETE("delete", DeleteUser)
	userGroup.POST("enable", EnableUser)
	userGroup.GET("all", GetAllUsers)
	userGroup.GET("detail", GetUserDetail)
	userGroup.POST("resetpwd", ResetPwd)
	userGroup.POST("changepwd", ChangePwd)
	userGroup.GET("export", ExportUser)
	userGroup.POST("import", ImportUser)
	userGroup.PUT("token/update", UpdateBasicsByToken)
	userGroup.GET("token/detail", GetBasicsByToken)
}
