package model

import (
	"encoding/json"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm/clause"
)

func CreateSystemConfig(m *SystemConfig) (string, error) {
	err := dbClient.DB().Create(m).Error
	return m.ID, err
}

func UpdateSystemConfig(m *SystemConfig) error {
	return dbClient.DB().Omit("created_at").Save(m).Error
}

func DeleteSystemConfig(id string) (err error) {
	return dbClient.DB().Delete(&SystemConfig{}, "id=?", id).Error
}

func QuerySystemConfig(req *apipb.QuerySystemConfigRequest, resp *apipb.QuerySystemConfigResponse, preload bool) {
	db := dbClient.DB().Model(&SystemConfig{})
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}

	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`key`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}

	var list []*SystemConfig
	resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = SystemConfigsToPB(list)
	}
	resp.Total = resp.Records
}

func GetSystemConfigByID(id string) (*SystemConfig, error) {
	m := &SystemConfig{}
	err := dbClient.DB().Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

func GetSystemConfigByIDs(ids []string) ([]*SystemConfig, error) {
	var m []*SystemConfig
	err := dbClient.DB().Preload(clause.Associations).Where("id in (?)", ids).Find(&m).Error
	return m, err
}

func GetAllSystemConfigs() (list []*SystemConfig, err error) {
	err = dbClient.DB().Find(&list).Error
	return
}

func ExportAllSystemConfigs(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := dbClient.DB().Model(&SystemConfig{})

	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}

	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}

	var list []*SystemConfig
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

type SystemConfig struct {
	commonmodel.Model
	Key       string `json:"key" gorm:"size:100;uniqueIndex"`
	Value     string `json:"value" gorm:"type:text"`
	TenantID  string `json:"tenantID" gorm:"size:36;index"`
	ProjectID string `json:"projectID" gorm:"index;size:36"`
	IsMust    bool   `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

// GetSystemConfigByKey returns a single config by key.
func GetSystemConfigByKey(key string) (*SystemConfig, error) {
	m := &SystemConfig{}
	err := dbClient.DB().Where("`key` = ?", key).First(m).Error
	return m, err
}

// GetSystemConfigsByKeys returns configs matching the given keys.
func GetSystemConfigsByKeys(keys []string) ([]*SystemConfig, error) {
	var list []*SystemConfig
	err := dbClient.DB().Where("`key` IN ?", keys).Find(&list).Error
	return list, err
}

// GetSystemConfigMapByKeys returns a key→value map for the given keys.
func GetSystemConfigMapByKeys(keys []string) (map[string]string, error) {
	list, err := GetSystemConfigsByKeys(keys)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(list))
	for _, c := range list {
		out[c.Key] = c.Value
	}
	return out, nil
}

// UpsertSystemConfigByKey creates or updates a config row identified by key.
// Uses ON CONFLICT for atomic upsert — safe under concurrent writes.
func UpsertSystemConfigByKey(key, value string) error {
	if key == "" {
		return nil
	}
	return dbClient.DB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&SystemConfig{Key: key, Value: value}).Error
}

func PBToSystemConfigs(in []*apipb.SystemConfigInfo) []*SystemConfig {
	var result []*SystemConfig
	for _, c := range in {
		result = append(result, PBToSystemConfig(c))
	}
	return result
}

func PBToSystemConfig(in *apipb.SystemConfigInfo) *SystemConfig {
	if in == nil {
		return nil
	}
	return &SystemConfig{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		Key:       in.Key,
		Value:     in.Value,
		TenantID:  in.TenantID,
		ProjectID: in.ProjectID,
		IsMust:    in.IsMust,
	}
}

func SystemConfigsToPB(in []*SystemConfig) []*apipb.SystemConfigInfo {
	var list []*apipb.SystemConfigInfo
	for _, f := range in {
		list = append(list, SystemConfigToPB(f))
	}
	return list
}

func SystemConfigToPB(in *SystemConfig) *apipb.SystemConfigInfo {
	if in == nil {
		return nil
	}
	return &apipb.SystemConfigInfo{
		Id:        in.ID,
		Key:       in.Key,
		Value:     in.Value,
		ProjectID: in.ProjectID,
		TenantID:  in.TenantID,
		IsMust:    in.IsMust,
	}
}
