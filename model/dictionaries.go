package model

import (
	"encoding/json"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm/clause"
)

func CreateDictionaries(m *Dictionaries) (string, error) {
	err := dbClient.DB().Create(m).Error
	return m.ID, err
}

func UpdateDictionaries(m *Dictionaries) error {
	return dbClient.DB().Omit("created_at").Save(m).Error
}

func DeleteDictionaries(id string) (err error) {
	return dbClient.DB().Delete(&Dictionaries{}, "id=?", id).Error
}

func QueryDictionaries(req *apipb.QueryDictionariesRequest, resp *apipb.QueryDictionariesResponse, preload bool) {
	db := dbClient.DB().Model(&Dictionaries{})
	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}

	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`updated_at` desc")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}

	var list []*Dictionaries
	resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = DictionariesArrayToPB(list)
	}
	resp.Total = resp.Records
}

func GetDictionariesByID(id string) (*Dictionaries, error) {
	m := &Dictionaries{}
	err := dbClient.DB().Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

func GetDictionariesByIDs(ids []string) ([]*Dictionaries, error) {
	var m []*Dictionaries
	err := dbClient.DB().Preload(clause.Associations).Where("id in (?)", ids).Find(&m).Error
	return m, err
}

func GetAllDictionaries() (list []*Dictionaries, err error) {
	err = dbClient.DB().Find(&list).Error
	return
}

func ExportAllDictionaries(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := dbClient.DB().Model(&Dictionaries{})

	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}

	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}

	var list []*Dictionaries
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

type Dictionaries struct {
	commonmodel.Model
	//Description
	Description string `json:"description" gorm:"size:200" `
	//Name
	Name string `json:"name" gorm:"size:100" `
	//Type
	Type string `json:"type" gorm:"size:100" `
	//Value
	Value string `json:"value" gorm:"" `
	//租户ID
	TenantID  string `json:"tenantID" gorm:"size:36;index" `
	ProjectID string `json:"projectID" gorm:"index;size:36"`
	IsMust    bool   `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

func PBToDictionaries(in []*apipb.DictionariesInfo) []*Dictionaries {
	var result []*Dictionaries
	for _, c := range in {
		result = append(result, PBToDictionariesArray(c))
	}
	return result
}

func PBToDictionariesArray(in *apipb.DictionariesInfo) *Dictionaries {
	if in == nil {
		return nil
	}
	return &Dictionaries{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		Description: in.Description,
		Name:        in.Name,
		Type:        in.Type,
		Value:       in.Value,
		TenantID:    in.TenantID,
		ProjectID:   in.ProjectID,
		IsMust:      in.IsMust,
	}
}

func DictionariesArrayToPB(in []*Dictionaries) []*apipb.DictionariesInfo {
	var list []*apipb.DictionariesInfo
	for _, f := range in {
		list = append(list, DictionariesToPB(f))
	}
	return list
}

func DictionariesToPB(in *Dictionaries) *apipb.DictionariesInfo {
	if in == nil {
		return nil
	}
	return &apipb.DictionariesInfo{
		Id:          in.ID,
		Description: in.Description,
		Name:        in.Name,
		Type:        in.Type,
		Value:       in.Value,
		TenantID:    in.TenantID,
		ProjectID:   in.ProjectID,
		IsMust:      in.IsMust,
	}
}
