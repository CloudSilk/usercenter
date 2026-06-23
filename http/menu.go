package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AddMenu godoc
// @Summary 新增菜单
// @Tags 菜单管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.MenuInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/add [post]
func AddMenu(c *gin.Context, req *apipb.MenuInfo) (*model.CommonResponse, error) {
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	if err := permission.AddMenu(permission.PBToMenu(req)); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

// UpdateMenu godoc
// @Summary 更新菜单
// @Tags 菜单管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.MenuInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/update [put]
func UpdateMenu(c *gin.Context, req *apipb.MenuInfo) (*model.CommonResponse, error) {
	if err := permission.UpdateMenu(permission.PBToMenu(req)); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

// DeleteMenu godoc
// @Summary 删除菜单
// @Tags 菜单管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/delete [delete]
func DeleteMenu(c *gin.Context, req *apipb.DelRequest) (*model.CommonResponse, error) {
	if err := permission.DeleteMenu(req.Id); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

// QueryMenu godoc
// @Summary 分页查询
// @Tags 菜单管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QueryMenuResponse
// @Router /api/core/auth/menu/query [get]
func QueryMenu(c *gin.Context, req *apipb.QueryMenuRequest) (*apipb.QueryMenuResponse, error) {
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	resp := &apipb.QueryMenuResponse{Code: apipb.Code_Success}
	permission.QueryMenu(req, resp, false)
	return resp, nil
}

// GetMenuDetail godoc
// @Summary 查询明细
// @Tags 菜单管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetMenuDetailResponse
// @Router /api/core/auth/menu/detail [get]
func GetMenuDetail(c *gin.Context) {
	resp := &apipb.GetMenuDetailResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	data, err := permission.GetMenuByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = permission.MenuToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// GetMenuTree godoc
// @Summary 查询所有菜单（Tree）
// @Tags 菜单管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.QueryMenuResponse
// @Router /api/core/auth/menu/tree [get]
func GetMenuTree(c *gin.Context) {
	resp := &apipb.QueryMenuResponse{Code: apipb.Code_Success}
	tenantID := ucm.GetTenantID(c)
	if tenantID == constants.PlatformTenantID {
		tenantID = c.Query("tenantID")
	}
	data, records, err := getAuthorizedMenuTree(tenantID)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = permission.MenusToPB(data)
		resp.Records = records
	}
	c.JSON(http.StatusOK, resp)
}

// ExportMenu godoc
// @Summary 导出
// @Tags 菜单管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/export [get]
func ExportMenu(c *gin.Context) {
	req := &apipb.QueryMenuRequest{}
	resp := &apipb.QueryMenuResponse{Code: apipb.Code_Success}
	if err := c.BindQuery(req); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	req.PageIndex = 1
	req.PageSize = 1000
	permission.QueryMenu(req, resp, true)
	if resp.Code != apipb.Code_Success {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment;filename=Menu.json")
	c.Header("Content-Transfer-Encoding", "binary")
	buf, _ := json.Marshal(resp.Data)
	c.Writer.Write(buf)
}

// ImportMenu godoc
// @Summary 导入
// @Tags 菜单管理
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/import [post]
func ImportMenu(c *gin.Context) {
	resp := &apipb.QueryMenuResponse{Code: apipb.Code_Success}
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
	var list []*apipb.MenuInfo
	if err := json.Unmarshal(buf, &list); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	successCount, failCount := 0, 0
	for _, f := range list {
		if err := permission.UpdateMenu(permission.PBToMenu(f)); err != nil {
			if err == gorm.ErrRecordNotFound {
				err = permission.AddMenu(permission.PBToMenu(f))
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

// getAuthorizedMenuTree 返回授权菜单树（从 model/role.go 中迁移）
func getAuthorizedMenuTree(tenantID string) ([]*permission.Menu, int64, error) {
	if tenantID != "" {
		t, err := tenant.GetTenantByID(tenantID)
		if err != nil {
			return nil, 0, err
		}
		authMenus := t.GetAuthorizedMenu()
		result, err := permission.GetAuthorizedMenu(store.DB(), authMenus, false)
		return result, 0, err
	}
	menus, err := permission.GetBaseMenuTree()
	return menus, 0, err
}

func RegisterMenuRouter(r *gin.Engine) {
	menuGroup := r.Group("/api/core/auth/menu")
	menuGroup.POST("add", AutoHandler(AddMenu))
	menuGroup.PUT("update", AutoHandler(UpdateMenu))
	menuGroup.GET("query", AutoQueryHandler(QueryMenu))
	menuGroup.DELETE("delete", AutoHandler(DeleteMenu))
	menuGroup.GET("detail", GetMenuDetail)
	menuGroup.GET("tree", GetMenuTree)
	menuGroup.GET("export", ExportMenu)
	menuGroup.POST("import", ImportMenu)
}
