package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/pkg/utils/middleware"
	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AddProject godoc
// @Summary 新增
// @Description 新增
// @Tags 项目管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.ProjectInfo true "Add Project"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/project/add [post]
func AddProject(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.ProjectInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建项目请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}

	//只有平台租户才能为其他租户创建用户
	tenantID := ucm.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}

	id, err := model.CreateProject(model.PBToProject(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateProject godoc
// @Summary 更新
// @Description 更新
// @Tags 项目管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.ProjectInfo true "Update Project"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/project/update [put]
func UpdateProject(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.ProjectInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,更新项目请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.UpdateProject(model.PBToProject(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteProject godoc
// @Summary 删除
// @Description 删除
// @Tags 项目管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete Project"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/project/delete [delete]
func DeleteProject(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.DelRequest{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,删除项目请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.DeleteProject(req.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryProject godoc
// @Summary 分页查询
// @Description 分页查询
// @Tags 项目管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param tenantID query string false "租户ID"
// @Param name query string false "名称"
// @Success 200 {object} apipb.QueryProjectResponse
// @Router /api/core/auth/project/query [get]
func QueryProject(c *gin.Context) {
	req := &apipb.QueryProjectRequest{}
	resp := &apipb.QueryProjectResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	//只有平台租户才能查询其他租户的项目
	tenantID := ucm.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	model.QueryProject(req, resp, false)

	c.JSON(http.StatusOK, resp)
}

// GetProjectDetail godoc
// @Summary 查询明细
// @Description 查询明细
// @Tags 项目管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetProjectDetailResponse
// @Router /api/core/auth/project/detail [get]
func GetProjectDetail(c *gin.Context) {
	resp := &apipb.GetProjectDetailResponse{
		Code: apipb.Code_Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := model.GetProjectByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.ProjectToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// GetAllProject godoc
// @Summary 查询所有
// @Description 查询所有
// @Tags 项目管理管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.QueryProjectResponse
// @Router /api/core/auth/project/all [get]
func GetAllProject(c *gin.Context) {
	resp := &apipb.QueryProjectResponse{
		Code: apipb.Code_Success,
	}
	list, err := model.GetAllProjects()
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = model.ProjectsToPB(list)
	c.JSON(http.StatusOK, resp)
}

// ImportProject
// @Summary 导入
// @Description 导入
// @Tags 项目管理管理
// @Accept  mpfd
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/project/import [post]
func ImportProject(c *gin.Context) {
	resp := &apipb.QueryProjectResponse{
		Code: apipb.Code_Success,
	}
	//从项目管理中读取文件
	file, fileHeader, err := c.Request.FormFile("files")
	if err != nil {
		fmt.Println(err)
		resp.Code = apipb.Code_BadRequest
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

	var list []*apipb.ProjectInfo
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
		err = model.UpdateProjectAll(model.PBToProject(f))
		if err == gorm.ErrRecordNotFound {
			_, err = model.CreateProject(model.PBToProject(f))
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

// ExportProject godoc
// @Summary 导出
// @Description 导出
// @Tags 项目管理管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param tenantID query string false "租户ID"
// @Param name query string false "名称"
// @Param ids query []string false "IDs"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/project/export [get]
func ExportProject(c *gin.Context) {
	req := &apipb.QueryProjectRequest{}
	resp := &apipb.QueryProjectResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	req.PageIndex = 1
	req.PageSize = 1000
	model.QueryProject(req, resp, true)
	if resp.Code != apipb.Code_Success {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.Header("Content-Type", "application/octet-stream")

	c.Header("Content-Disposition", "attachment;filename=Project.json")
	c.Header("Content-Transfer-Encoding", "binary")
	buf, _ := json.Marshal(resp.Data)
	c.Writer.Write(buf)
}

func RegisterProjectRouter(r *gin.Engine) {
	g := r.Group("/api/core/auth/project")

	g.POST("add", AddProject)
	g.PUT("update", UpdateProject)
	g.GET("query", QueryProject)
	g.DELETE("delete", DeleteProject)
	g.GET("detail", GetProjectDetail)
	g.GET("all", GetAllProject)
	g.GET("export", ExportProject)
	g.POST("import", ImportProject)
}
