package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/pkg/utils/log"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type API struct {
	commonmodel.Model
	TenantID    string `json:"tenantID" gorm:"index;size:36"`
	ProjectID   string `json:"projectID" gorm:"index;size:36"`
	Path        string `json:"path" gorm:"size:200;index;comment:路径"`
	Group       string `json:"group" gorm:"size:50;index;comment:分组"`
	Method      string `json:"method" gorm:"size:50;index;default:POST;comment:方法"`
	Description string `json:"description" gorm:"size:200;index;comment:中文描述"`
	Enable      bool   `json:"enable" gorm:"index;comment:是否启用API"`
	//如果不开启权限校验，那么在每个角色都加上casbin rule
	//1. 不需要登录就可以直接访问
	//2. 需要登录但不需要校验权限，也就是所有人都可以访问
	//3. 需要登录并且校验权限
	CheckAuth  bool `json:"checkAuth" gorm:"index;comment:是否校验权限"`
	CheckLogin bool `json:"checkLogin" gorm:"index;comment:是否校验登录"`
	IsMust     bool `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

func (API) TableName() string {
	return "api"
}

func CreateAPI(api *API) error {
	duplication, err := dbClient.CreateWithCheckDuplication(api, "path = ? AND method = ? and tenant_id =? and project_id = ?", api.Path, api.Method, api.TenantID, api.ProjectID)
	if err != nil {
		return err
	}
	if duplication {
		return errors.New("存在相同api")
	}
	updateNotCheckAuthRule()
	updateNotCheckLoginRule()
	return nil
}

func DeleteApi(id string) (err error) {
	api := &API{}
	err = dbClient.DB().Where("id = ?", id).First(&api).Error
	if err != nil {
		return err
	}
	err = dbClient.DB().Delete(api).Error
	if err != nil {
		return err
	}
	ClearCasbin(1, api.Path, api.Method)
	return nil
}

type QueryAPIRequest struct {
	commonmodel.CommonRequest
	Path       string `json:"path" form:"path" path:"path"`
	Method     string `json:"method" form:"method" path:"method"`
	Group      string `json:"group" form:"group" path:"group"`
	CheckAuth  int    `json:"checkAuth" form:"checkAuth" path:"checkAuth"`
	CheckLogin int    `json:"checkLogin" form:"checkLogin" path:"checkLogin"`
}

type QueryAPIResponse struct {
	commonmodel.CommonResponse
	Data []API `json:"data"`
}

// @author: [guoxf](https://github.com/guoxf)
// @function: GetAPIInfoList
// @description: 分页查询API
// @param: api API, info PageInfo, order string, desc bool
// @return: list []*API, total int64 , err error
func QueryAPI(req *apipb.QueryAPIRequest, resp *apipb.QueryAPIResponse) {
	db := dbClient.DB().Model(&API{})

	if req.Path != "" {
		db = db.Where("path LIKE ?", "%"+req.Path+"%")
	}

	if req.Method != "" {
		db = db.Where("method = ?", req.Method)
	}

	if req.Group != "" {
		db = db.Where("`group` = ?", req.Group)
	}

	if req.CheckAuth > 0 {
		db = db.Where("check_auth = ?", req.CheckAuth == 1)
	}

	if req.CheckLogin > 0 {
		db = db.Where("check_login = ?", req.CheckLogin == 1)
	}

	if len(req.Ids) > 0 {
		db = db.Where("id in ?", req.Ids)
	}

	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}

	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`path`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}

	var apis []API
	resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &apis)
	if err != nil {
		resp.Code = commonmodel.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = APIsToPB(apis)
	}
	resp.Total = resp.Records
}

func GetAllAPIs(req *apipb.QueryAPIRequest) (apis []API, err error) {
	db := dbClient.DB()
	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}

	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	err = db.Find(&apis).Error
	return
}

func GetAPIById(id string) (api API, err error) {
	err = dbClient.DB().Where("id = ?", id).First(&api).Error
	return
}

func UpdateAPI(api *API) error {
	var oldA API
	err := dbClient.DB().Where("id = ?", api.ID).First(&oldA).Error
	if err != nil {
		return err
	}

	if oldA.Path != api.Path || oldA.Method != api.Method {
		err = UpdateCasbinApi(oldA.Path, api.Path, oldA.Method, api.Method)
		if err != nil {
			return err
		}
	}

	duplication, err := dbClient.UpdateWithCheckDuplicationAndOmit(dbClient.DB(), api, false, []string{"created_at"}, "id <> ? and path = ? AND method = ? and tenant_id =? and project_id = ?", api.ID, api.Path, api.Method, api.TenantID, api.ProjectID)
	if err != nil {
		return err
	}
	if duplication {
		return errors.New("存在相同api")
	}
	// updateNotCheckAuthRule()
	// updateNotCheckLoginRule()
	return nil
}

func EnableAPI(id string, enable bool) error {
	err := dbClient.DB().Table("api").Where("id=?", id).Update("enable", enable).Error
	if err != nil {
		return err
	}
	updateNotCheckAuthRule()
	updateNotCheckLoginRule()
	return nil
}

func updateNotCheckAuthRule() {
	resp := &apipb.QueryAPIResponse{
		Code: commonmodel.Success,
	}
	QueryAPI(&apipb.QueryAPIRequest{
		CheckAuth:  2,
		CheckLogin: 1,
		PageSize:   1000,
	}, resp)
	if resp.Code != commonmodel.Success {
		log.Errorf(context.Background(), "updateNotCheckAuthRule error:%s", resp.Message)
		return
	}
	fmt.Println("updateNotCheckAuthRule===>", resp.Data)
	var newRules []*CasbinRule
	for _, api := range resp.Data {
		newRules = append(newRules, &CasbinRule{
			Ptype:     "p",
			RoleID:    "0",
			Path:      api.Path,
			Method:    api.Method,
			CheckAuth: "false",
		})
	}
	_, err := ClearCasbin(0, "0")
	if err != nil {
		log.Errorf(context.Background(), "updateNotCheckAuthRule error:%v", err)
	}
	if len(newRules) > 0 {
		err = UpdateCasbin("0", newRules)
		if err != nil {
			log.Errorf(context.Background(), "updateNotCheckAuthRule error:%v", err)
		}
	}
}

func updateNotCheckLoginRule() {
	resp := &apipb.QueryAPIResponse{
		Code: commonmodel.Success,
	}
	QueryAPI(&apipb.QueryAPIRequest{
		CheckLogin: 2,
		PageSize:   1000,
	}, resp)
	if resp.Code != commonmodel.Success {
		log.Errorf(context.Background(), "updateNotCheckLoginRule error:%s", resp.Message)
		return
	}
	var newRules []*CasbinRule
	for _, api := range resp.Data {
		newRules = append(newRules, &CasbinRule{
			Ptype:     "p",
			RoleID:    "-1",
			Path:      api.Path,
			Method:    api.Method,
			CheckAuth: "false",
		})
	}
	_, err := ClearCasbin(0, "-1")
	if err != nil {
		log.Errorf(context.Background(), "updateNotCheckLoginRule error:%v", err)
	}
	if len(newRules) > 0 {
		err = UpdateCasbin("-1", newRules)
		if err != nil {
			log.Errorf(context.Background(), "updateNotCheckLoginRule error:%v", err)
		}
	}
}

func ExportAllApis(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := dbClient.DB().Model(&API{})

	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}

	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}

	var list []*API
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

func PBToAPI(in *apipb.APIInfo) *API {
	if in == nil {
		return nil
	}
	return &API{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		ProjectID:   in.ProjectID,
		TenantID:    in.TenantID,
		Path:        in.Path,
		Group:       in.Group,
		Method:      in.Method,
		Enable:      in.Enable,
		Description: in.Description,
		CheckAuth:   in.CheckAuth,
		CheckLogin:  in.CheckLogin,
		IsMust:      in.IsMust,
	}
}

func APIToPB(in *API) *apipb.APIInfo {
	if in == nil {
		return nil
	}
	return &apipb.APIInfo{
		Id:          in.ID,
		ProjectID:   in.ProjectID,
		TenantID:    in.TenantID,
		Path:        in.Path,
		Group:       in.Group,
		Method:      in.Method,
		Enable:      in.Enable,
		Description: in.Description,
		CheckAuth:   in.CheckAuth,
		CheckLogin:  in.CheckLogin,
		IsMust:      in.IsMust,
	}
}

func APIsToPB(in []API) []*apipb.APIInfo {
	var list []*apipb.APIInfo
	for _, api := range in {
		list = append(list, APIToPB(&api))
	}
	return list
}
