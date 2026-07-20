package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func menuMutationResponse(err error) *apipb.CommonResponse {
	if err == nil {
		return &apipb.CommonResponse{Code: apipb.Code_Success}
	}
	code := apipb.Code_InternalServerError
	if errors.Is(err, permission.ErrInvalidMenu) ||
		errors.Is(err, permission.ErrProtectedMenu) ||
		errors.Is(err, permission.ErrMenuHasChild) {
		code = apipb.Code_BadRequest
	}
	return &apipb.CommonResponse{Code: code, Message: err.Error()}
}

func canManageMenu(c *gin.Context, menuID string) bool {
	currentTenantID := ucm.GetTenantID(c)
	if currentTenantID == constants.PlatformTenantID {
		return true
	}
	menu, err := permission.GetMenuByID(strings.TrimSpace(menuID))
	return err == nil && menu.TenantID == currentTenantID
}

func noMenuPermissionResponse() *apipb.CommonResponse {
	return &apipb.CommonResponse{Code: apipb.Code_NoPermission, Message: "no permission to manage this menu"}
}

func menuFunctionBindingError(c *gin.Context, err error) {
	if errors.Is(err, permission.ErrMenuFunctionRevisionConflict) {
		c.JSON(http.StatusOK, gin.H{
			"code":    apipb.Code_BadRequest,
			"message": err.Error(),
			"data": gin.H{
				"conflict": true,
			},
		})
		return
	}
	if errors.Is(err, permission.ErrInvalidMenu) || errors.Is(err, permission.ErrProtectedMenu) {
		writeBadRequest(c, err)
		return
	}
	writeErr(c, err)
}

type menuReorderRequest struct {
	ID        string `json:"id" validate:"required"`
	Direction string `json:"direction" validate:"required"`
}

// AddMenu godoc
// @Summary 新增菜单
// @Tags 菜单管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.MenuInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/add [post]
func AddMenu(c *gin.Context, req *apipb.MenuInfo) (*apipb.CommonResponse, error) {
	req.IsMust = false
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	} else if strings.TrimSpace(req.TenantID) == "" {
		req.TenantID = constants.PlatformTenantID
	}
	menu := permission.PBToMenu(req)
	if err := permission.AddMenu(menu); err != nil {
		return menuMutationResponse(err), nil
	}
	recordAudit(c, "create_menu", menu.ID, fmt.Sprintf("%s %s", menu.Name, menu.Path))
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// UpdateMenu godoc
// @Summary 更新菜单
// @Tags 菜单管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.MenuInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/update [put]
func UpdateMenu(c *gin.Context, req *apipb.MenuInfo) (*apipb.CommonResponse, error) {
	if !canManageMenu(c, req.Id) {
		return noMenuPermissionResponse(), nil
	}
	current, err := permission.GetMenuByID(req.Id)
	if err != nil {
		return menuMutationResponse(err), nil
	}
	req.TenantID = current.TenantID
	req.ProjectID = current.ProjectID
	req.IsMust = current.IsMust
	if err := permission.UpdateMenu(permission.PBToMenu(req)); err != nil {
		return menuMutationResponse(err), nil
	}
	recordAudit(c, "update_menu", req.Id, fmt.Sprintf("%s %s", req.Name, req.Path))
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteMenu godoc
// @Summary 删除菜单
// @Tags 菜单管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/delete [delete]
func DeleteMenu(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	if !canManageMenu(c, req.Id) {
		return noMenuPermissionResponse(), nil
	}
	if err := permission.DeleteMenu(req.Id); err != nil {
		return menuMutationResponse(err), nil
	}
	recordAudit(c, "delete_menu", req.Id, "")
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

func ReorderMenu(c *gin.Context, req *menuReorderRequest) (*apipb.CommonResponse, error) {
	if !canManageMenu(c, req.ID) {
		return noMenuPermissionResponse(), nil
	}
	if err := permission.ReorderMenu(req.ID, req.Direction); err != nil {
		return menuMutationResponse(err), nil
	}
	recordAudit(c, "reorder_menu", req.ID, req.Direction)
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
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
	var hidden *bool
	switch strings.TrimSpace(c.Query("visibility")) {
	case "", "all":
	case "visible":
		value := false
		hidden = &value
	case "hidden":
		value := true
		hidden = &value
	default:
		resp.Code = apipb.Code_BadRequest
		resp.Message = "visibility must be all, visible or hidden"
		return resp, nil
	}
	permission.QueryMenus(req, resp, false, c.Query("keyword"), hidden)
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
	if !canManageMenu(c, idStr) {
		resp.Code = apipb.Code_NoPermission
		resp.Message = "no permission to view this menu"
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

func GetMenuImpact(c *gin.Context) {
	idStr := strings.TrimSpace(c.Query("id"))
	if idStr == "" {
		writeBadRequest(c, errors.New("id required"))
		return
	}
	if !canManageMenu(c, idStr) {
		c.JSON(http.StatusOK, noMenuPermissionResponse())
		return
	}
	impact, err := permission.GetMenuImpact(idStr)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeOK(c, gin.H{"data": impact})
}

// GetMenuTree godoc
// @Summary 查询所有菜单（Tree）
// @Tags 菜单管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.QueryMenuResponse
// @Router /api/core/auth/menu/tree [get]
// GetMenuFunctionBindings returns the native menu action -> API -> role/Casbin
// projection without copying permission data into an application-specific model.
func GetMenuFunctionBindings(c *gin.Context) {
	menuID := strings.TrimSpace(c.Query("id"))
	if menuID == "" {
		writeBadRequest(c, errors.New("id required"))
		return
	}
	if !canManageMenu(c, menuID) {
		c.JSON(http.StatusOK, noMenuPermissionResponse())
		return
	}
	detail, err := permission.GetMenuFunctionBindings(menuID)
	if err != nil {
		menuFunctionBindingError(c, err)
		return
	}
	writeOK(c, gin.H{"data": detail})
}

// UpdateMenuFunctionBindings atomically replaces a menu's functions and API
// links, rebuilds every affected role policy, and revokes stale login contexts.
func UpdateMenuFunctionBindings(c *gin.Context) {
	req := &permission.MenuFunctionBindingUpdate{}
	if err := c.ShouldBindJSON(req); err != nil {
		menuFunctionBindingError(c, err)
		return
	}
	if !canManageMenu(c, req.MenuID) {
		c.JSON(http.StatusOK, noMenuPermissionResponse())
		return
	}
	result, err := permission.ReplaceMenuFunctionBindings(*req)
	if err != nil {
		menuFunctionBindingError(c, err)
		return
	}
	currentSessionRevoked := false
	currentUserID := ucm.GetUserID(c)
	for _, userID := range result.AffectedUserIDs {
		if userID == currentUserID {
			currentSessionRevoked = true
			break
		}
	}
	auditDetail, _ := json.Marshal(map[string]interface{}{
		"revision":        result.Revision,
		"summary":         result.Summary,
		"sessionsRevoked": result.SessionsRevoked,
	})
	recordAudit(c, "update_menu_function_bindings", req.MenuID, string(auditDetail))
	writeOK(c, gin.H{
		"data": gin.H{
			"revision":              result.Revision,
			"summary":               result.Summary,
			"sessionsRevoked":       result.SessionsRevoked,
			"currentSessionRevoked": currentSessionRevoked,
		},
	})
}

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
	currentTenantID := ucm.GetTenantID(c)
	for _, f := range list {
		var itemErr error
		existing, lookupErr := permission.GetMenuByID(f.Id)
		switch {
		case lookupErr == nil:
			if currentTenantID != constants.PlatformTenantID && existing.TenantID != currentTenantID {
				failCount++
				continue
			}
			f.TenantID = existing.TenantID
			f.ProjectID = existing.ProjectID
			f.IsMust = existing.IsMust
			itemErr = permission.UpdateMenu(permission.PBToMenu(f))
		case errors.Is(lookupErr, gorm.ErrRecordNotFound):
			f.IsMust = false
			if currentTenantID != constants.PlatformTenantID {
				f.TenantID = currentTenantID
			} else if strings.TrimSpace(f.TenantID) == "" {
				f.TenantID = constants.PlatformTenantID
			}
			itemErr = permission.AddMenu(permission.PBToMenu(f))
		default:
			itemErr = lookupErr
		}
		if itemErr != nil {
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
	menuGroup.PUT("reorder", AutoHandler(ReorderMenu))
	menuGroup.GET("query", AutoQueryHandler(QueryMenu))
	menuGroup.DELETE("delete", AutoHandler(DeleteMenu))
	menuGroup.GET("detail", GetMenuDetail)
	menuGroup.GET("impact", GetMenuImpact)
	menuGroup.GET("functions", GetMenuFunctionBindings)
	menuGroup.PUT("functions", UpdateMenuFunctionBindings)
	menuGroup.GET("tree", GetMenuTree)
	menuGroup.GET("export", ExportMenu)
	menuGroup.POST("import", ImportMenu)
}
