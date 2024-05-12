package model

import (
	"encoding/json"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm/clause"
)

func CreateWechatConfig(m *WechatConfig) (string, error) {
	err := dbClient.DB().Create(m).Error
	return m.ID, err
}

func UpdateWechatConfig(m *WechatConfig) error {
	return dbClient.DB().Omit("created_at").Save(m).Error
}

func DeleteWechatConfig(id string) (err error) {
	return dbClient.DB().Delete(&WechatConfig{}, "id=?", id).Error
}

func QueryWechatConfig(req *apipb.QueryWechatConfigRequest, resp *apipb.QueryWechatConfigResponse, preload bool) {
	db := dbClient.DB().Model(&WechatConfig{})
	if req.AppName != "" {
		db = db.Where("app_name LIKE ?", "%"+req.AppName+"%")
	}

	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}

	if req.AppType > 0 {
		db = db.Where("app_type = ?", req.AppType)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}

	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`display_name`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}

	var list []*WechatConfig
	resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = WechatConfigsToPB(list)
	}
	resp.Total = resp.Records
}

func GetWechatConfigByID(id string) (*WechatConfig, error) {
	m := &WechatConfig{}
	err := dbClient.DB().Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

func GetWechatConfigByIDs(ids []string) ([]*WechatConfig, error) {
	var m []*WechatConfig
	err := dbClient.DB().Preload(clause.Associations).Where("id in (?)", ids).Find(&m).Error
	return m, err
}

func GetAllWechatConfigs() (list []*WechatConfig, err error) {
	err = dbClient.DB().Find(&list).Error
	return
}

func ExportAllWechatConfigs(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := dbClient.DB().Model(&WechatConfig{})

	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}

	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}

	var list []*WechatConfig
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

type WechatConfig struct {
	commonmodel.Model
	//APP ID
	AppID string `json:"appID" gorm:"size:36" `
	//APP名称
	AppName string `json:"appName" gorm:"size:100;uniqueindex:wechatconfig_uidx1" `
	//显示名称
	DisplayName string `json:"displayName" gorm:"size:100" `
	//秘钥
	Secret string `json:"secret" gorm:"size:512" `
	//租户ID
	TenantID string `json:"tenantID" gorm:"size:36;index" `
	//类型 1-微信小程序 2-微信公众号 3-微信APP应用 4-微信网站应用
	AppType int32 `json:"appType" gorm:"index;comment:1-微信小程序 2-微信公众号 3-微信APP应用 4-微信网站应用" `
	//默认角色 用户通过微信注册时赋予默认角色
	DefaultRoleID  string `json:"defaultRoleID" gorm:"size:36;comment:用户通过微信注册时赋予默认角色" `
	RedirectUrl    string
	Token          string `json:"token" gorm:"size:32;"`
	EncodingAESKey string `json:"encodingAESKey" gorm:"size:43;"`
	EncodingMethod int32  `json:"encodingMethod" gorm:"comment:1-明文模式 2-兼容模式 3-安全模式" `
	ProjectID      string `json:"projectID" gorm:"index;size:36"`
	IsMust         bool   `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

func PBToWechatConfigs(in []*apipb.WechatConfigInfo) []*WechatConfig {
	var result []*WechatConfig
	for _, c := range in {
		result = append(result, PBToWechatConfig(c))
	}
	return result
}

func PBToWechatConfig(in *apipb.WechatConfigInfo) *WechatConfig {
	if in == nil {
		return nil
	}
	return &WechatConfig{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		AppID:          in.AppID,
		AppName:        in.AppName,
		Secret:         in.Secret,
		TenantID:       in.TenantID,
		AppType:        in.AppType,
		DefaultRoleID:  in.DefaultRoleID,
		RedirectUrl:    in.RedirectUrl,
		Token:          in.Token,
		EncodingAESKey: in.EncodingAESKey,
		EncodingMethod: in.EncodingMethod,
		DisplayName:    in.DisplayName,
		ProjectID:      in.ProjectID,
		IsMust:         in.IsMust,
	}
}

func WechatConfigsToPB(in []*WechatConfig) []*apipb.WechatConfigInfo {
	var list []*apipb.WechatConfigInfo
	for _, f := range in {
		list = append(list, WechatConfigToPB(f))
	}
	return list
}

func WechatConfigToPB(in *WechatConfig) *apipb.WechatConfigInfo {
	if in == nil {
		return nil
	}
	return &apipb.WechatConfigInfo{
		Id:             in.ID,
		AppID:          in.AppID,
		AppName:        in.AppName,
		Secret:         in.Secret,
		TenantID:       in.TenantID,
		AppType:        in.AppType,
		DefaultRoleID:  in.DefaultRoleID,
		RedirectUrl:    in.RedirectUrl,
		Token:          in.Token,
		EncodingAESKey: in.EncodingAESKey,
		EncodingMethod: in.EncodingMethod,
		DisplayName:    in.DisplayName,
		ProjectID:      in.ProjectID,
		IsMust:         in.IsMust,
	}
}
