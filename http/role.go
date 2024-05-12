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

// AddRole
// @Summary 新增角色
// @Description 新增角色
// @Tags 角色管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.RoleInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/add [post]
func AddRole(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.RoleInfo{}
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建Role请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	//只有平台租户才能为其他租户创建角色
	tenantID := middleware.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	err = ucmodel.CreateRole(ucmodel.PBToRole(req))
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateRole
// @Summary 更新角色
// @Description 更新角色
// @Tags 角色管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.RoleInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/update [put]
func UpdateRole(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.RoleInfo{}
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建Role请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	//只有平台租户才能更改角色的租户
	tenantID := middleware.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	err = ucmodel.UpdateRole(ucmodel.PBToRole(req))
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteRole
// @Summary 删除角色
// @Description 软删除角色
// @Tags 角色管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/delete [delete]
func DeleteRole(c *gin.Context) {
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
		log.Warnf(context.Background(), "TransID:%s,新建Role请求参数无效:%v", transID, err)
		return
	}
	err = ucmodel.DeleteRole(req.Id)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryRole
// @Summary 分页查询
// @Description 分页查询
// @Tags 角色管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param tenantID query string false "租户ID"
// @Param name query string false "名称"
// @Success 200 {object} apipb.QueryRoleResponse
// @Router /api/core/auth/role/query [get]
func QueryRole(c *gin.Context) {
	req := &apipb.QueryRoleRequest{}
	resp := &apipb.QueryRoleResponse{
		Code: model.Success,
	}
	//只有平台租户才能查询其他租户的角色
	tenantID := middleware.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	ucmodel.QueryRole(req, resp, false)

	c.JSON(http.StatusOK, resp)
}

// GetRoleDetail
// @Summary 查询明细
// @Description 查询明细
// @Tags 角色管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetRoleDetailResponse
// @Router /api/core/auth/role/detail [get]
func GetRoleDetail(c *gin.Context) {
	resp := &apipb.GetRoleDetailResponse{
		Code: model.Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = model.BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := ucmodel.GetRoleByID(idStr)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = ucmodel.RoleToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// GetAllRole
// @Summary 查询所有角色
// @Description 查询所有角色
// @Tags 角色管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param tenantID query string false "租户ID"
// @Param containerComm query bool false "是否包含公共角色"
// @Success 200 {object} apipb.QueryRoleResponse
// @Router /api/core/auth/role/all [get]
func GetAllRole(c *gin.Context) {
	resp := &apipb.QueryRoleResponse{
		Code: model.Success,
	}
	req := &apipb.GetAllRoleRequest{}
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
	roles, err := ucmodel.GetAllRole(req.TenantID, req.ContainerComm)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = ucmodel.RolesToPB(roles)
	resp.Records = int64(len(roles))
	resp.Pages = 1
	c.JSON(http.StatusOK, resp)
}

// ExportRole godoc
// @Summary 导出
// @Description 导出
// @Tags 角色管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param ids query []string false "IDs"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/export [get]
func ExportRole(c *gin.Context) {
	req := &apipb.QueryRoleRequest{}
	resp := &apipb.QueryRoleResponse{
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
	ucmodel.QueryRole(req, resp, true)
	if resp.Code != apipb.Code_Success {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.Header("Content-Type", "application/octet-stream")

	c.Header("Content-Disposition", "attachment;filename=Role.json")
	c.Header("Content-Transfer-Encoding", "binary")
	buf, _ := json.Marshal(resp.Data)
	c.Writer.Write(buf)
}

// ImportRole
// @Summary 导入
// @Description 导入
// @Tags 角色管理
// @Accept  mpfd
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/import [post]
func ImportRole(c *gin.Context) {
	resp := &apipb.QueryRoleResponse{
		Code: apipb.Code_Success,
	}
	//从角色中读取文件
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

	var list []*apipb.RoleInfo
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
		err = ucmodel.UpdateRole(ucmodel.PBToRole(f))
		if err == gorm.ErrRecordNotFound {
			err = ucmodel.CreateRole(ucmodel.PBToRole(f))
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

func RegisterRoleRouter(r *gin.Engine) {
	roleGroup := r.Group("/api/core/auth/role")
	roleGroup.POST("add", AddRole)
	roleGroup.PUT("update", UpdateRole)
	roleGroup.GET("query", QueryRole)
	roleGroup.DELETE("delete", DeleteRole)
	roleGroup.GET("all", GetAllRole)
	roleGroup.GET("detail", GetRoleDetail)
	roleGroup.GET("export", ExportRole)
	roleGroup.POST("import", ImportRole)
}
