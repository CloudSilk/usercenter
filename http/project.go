package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/CloudSilk/pkg/constants"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/model"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AddProject godoc
// @Summary 新增
// @Tags 项目管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.ProjectInfo true "Add Project"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/project/add [post]
func AddProject(c *gin.Context, req *apipb.ProjectInfo) (*apipb.CommonResponse, error) {
	//只有平台租户才能为其他租户创建项目
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	id, err := model.CreateProject(model.PBToProject(req))
	if err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success, Message: id}, nil
}

// UpdateProject godoc
// @Summary 更新
// @Tags 项目管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.ProjectInfo true "Update Project"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/project/update [put]
func UpdateProject(c *gin.Context, req *apipb.ProjectInfo) (*apipb.CommonResponse, error) {
	if err := model.UpdateProject(model.PBToProject(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteProject godoc
// @Summary 删除
// @Tags 项目管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete Project"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/project/delete [delete]
func DeleteProject(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	if err := model.DeleteProject(req.Id); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QueryProject godoc
// @Summary 分页查询
// @Tags 项目管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QueryProjectResponse
// @Router /api/core/auth/project/query [get]
func QueryProject(c *gin.Context, req *apipb.QueryProjectRequest) (*apipb.QueryProjectResponse, error) {
	//只有平台租户才能查询其他租户的项目
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	resp := &apipb.QueryProjectResponse{Code: apipb.Code_Success}
	model.QueryProject(req, resp, false)
	return resp, nil
}

// GetProjectDetail godoc
// @Summary 查询明细
// @Tags 项目管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetProjectDetailResponse
// @Router /api/core/auth/project/detail [get]
func GetProjectDetail(c *gin.Context) {
	resp := &apipb.GetProjectDetailResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
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
// @Tags 项目管理管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.QueryProjectResponse
// @Router /api/core/auth/project/all [get]
func GetAllProject(c *gin.Context) {
	resp := &apipb.QueryProjectResponse{Code: apipb.Code_Success}
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

// ImportProject godoc
// @Summary 导入
// @Tags 项目管理管理
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/project/import [post]
func ImportProject(c *gin.Context) {
	resp := &apipb.QueryProjectResponse{Code: apipb.Code_Success}
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

	var list []*apipb.ProjectInfo
	if err := json.Unmarshal(buf, &list); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	successCount, failCount := 0, 0
	for _, f := range list {
		if err := model.UpdateProjectAll(model.PBToProject(f)); err != nil {
			if err == gorm.ErrRecordNotFound {
				_, err = model.CreateProject(model.PBToProject(f))
			}
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

// ExportProject godoc
// @Summary 导出
// @Tags 项目管理管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/project/export [get]
func ExportProject(c *gin.Context) {
	req := &apipb.QueryProjectRequest{}
	resp := &apipb.QueryProjectResponse{Code: apipb.Code_Success}
	if err := c.BindQuery(req); err != nil {
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
	g.POST("add", AutoHandler(AddProject))
	g.PUT("update", AutoHandler(UpdateProject))
	g.GET("query", AutoQueryHandler(QueryProject))
	g.DELETE("delete", AutoHandler(DeleteProject))
	g.GET("detail", GetProjectDetail)
	g.GET("all", GetAllProject)
	g.GET("export", ExportProject)
	g.POST("import", ImportProject)
}
