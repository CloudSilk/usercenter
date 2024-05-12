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

// AddMenu
// @Summary 新增菜单
// @Description 新增菜单
// @Tags 菜单管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.MenuInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/add [post]
func AddMenu(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.MenuInfo{}
	resp := &model.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建Menu请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	tenantID := ucm.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	err = ucmodel.AddMenu(ucmodel.PBToMenu(req))
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateMenu
// @Summary 更新菜单
// @Description 更新菜单
// @Tags 菜单管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.MenuInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/update [put]
func UpdateMenu(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.MenuInfo{}
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建Menu请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.UpdateMenu(ucmodel.PBToMenu(req))
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteMenu
// @Summary 删除菜单
// @Description 软删除菜单
// @Tags 菜单管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/delete [delete]
func DeleteMenu(c *gin.Context) {
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
		log.Warnf(context.Background(), "TransID:%s,新建Menu请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.DeleteMenu(req.Id)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryMenu
// @Summary 分页查询
// @Description 分页查询
// @Tags 菜单管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param path query string false "路径"
// @Param name query string false "名称"
// @Param title query string false "显示名称"
// @Param parentID query string false "父ID"
// @Param level query int false "层级"
// @Success 200 {object} apipb.QueryMenuResponse
// @Router /api/core/auth/menu/query [get]
func QueryMenu(c *gin.Context) {
	req := &apipb.QueryMenuRequest{}
	resp := &apipb.QueryMenuResponse{
		Code: model.Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	tenantID := ucm.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	ucmodel.QueryMenu(req, resp, false)

	c.JSON(http.StatusOK, resp)
}

// GetMenuDetail
// @Summary 查询明细
// @Description 查询明细
// @Tags 菜单管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetMenuDetailResponse
// @Router /api/core/auth/menu/detail [get]
func GetMenuDetail(c *gin.Context) {
	resp := &apipb.GetMenuDetailResponse{
		Code: model.Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = model.BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := ucmodel.GetMenuByID(idStr)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = ucmodel.MenuToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// GetMenuTree
// @Summary 查询所有菜单（Tree）
// @Description 查询所有菜单（Tree）
// @Tags 菜单管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.QueryMenuResponse
// @Router /api/core/auth/menu/tree [get]
func GetMenuTree(c *gin.Context) {
	resp := &apipb.QueryMenuResponse{
		Code: model.Success,
	}
	var err error
	reqTenantID := c.Query("tenantID")
	tenantID := middleware.GetTenantID(c)
	if tenantID == constants.PlatformTenantID {
		tenantID = reqTenantID
	}
	data, records, err := ucmodel.GetAuthorizedMenuTree(tenantID)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = ucmodel.MenusToPB(data)
		resp.Records = records
	}
	c.JSON(http.StatusOK, resp)
}

// ExportMenu godoc
// @Summary 导出
// @Description 导出
// @Tags 菜单管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param ids query []string false "IDs"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/export [get]
func ExportMenu(c *gin.Context) {
	req := &apipb.QueryMenuRequest{}
	resp := &apipb.QueryMenuResponse{
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
	ucmodel.QueryMenu(req, resp, true)
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

// ImportMenu
// @Summary 导入
// @Description 导入
// @Tags 菜单管理
// @Accept  mpfd
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/menu/import [post]
func ImportMenu(c *gin.Context) {
	resp := &apipb.QueryMenuResponse{
		Code: apipb.Code_Success,
	}
	//从菜单中读取文件
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

	var list []*apipb.MenuInfo
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
		err = ucmodel.UpdateMenu(ucmodel.PBToMenu(f))
		if err == gorm.ErrRecordNotFound {
			err = ucmodel.AddMenu(ucmodel.PBToMenu(f))
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

func RegisterMenuRouter(r *gin.Engine) {
	menuGroup := r.Group("/api/core/auth/menu")
	menuGroup.POST("add", AddMenu)
	menuGroup.PUT("update", UpdateMenu)
	menuGroup.GET("query", QueryMenu)
	menuGroup.DELETE("delete", DeleteMenu)
	menuGroup.GET("detail", GetMenuDetail)
	menuGroup.GET("tree", GetMenuTree)
	menuGroup.GET("export", ExportMenu)
	menuGroup.POST("import", ImportMenu)
}
