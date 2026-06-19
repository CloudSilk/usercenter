package dictionaries

import (
	"encoding/json"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm/clause"
)

// Dictionaries 字典。从 model/dictionaries.go 迁入(REDESIGN §4 阶段0),通过 store 访问 DB。
type Dictionaries struct {
	commonmodel.Model
	Description string `json:"description" gorm:"size:200"`
	Name        string `json:"name" gorm:"size:100"`
	Type        string `json:"type" gorm:"size:100"`
	Value       string `json:"value" gorm:""`
	TenantID    string `json:"tenantID" gorm:"size:36;index"`
	ProjectID   string `json:"projectID" gorm:"index;size:36"`
	IsMust      bool   `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

func CreateDictionaries(m *Dictionaries) (string, error) {
	err := store.DB().Create(m).Error
	return m.ID, err
}

func UpdateDictionaries(m *Dictionaries) error {
	return store.DB().Omit("created_at").Save(m).Error
}

func DeleteDictionaries(id string) (err error) {
	return store.DB().Delete(&Dictionaries{}, "id=?", id).Error
}

func QueryDictionaries(req *apipb.QueryDictionariesRequest, resp *apipb.QueryDictionariesResponse, preload bool) {
	db := store.DB().Model(&Dictionaries{})
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
	resp.Records, resp.Pages, err = store.Client().PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
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
	err := store.DB().Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

func GetDictionariesByIDs(ids []string) ([]*Dictionaries, error) {
	var m []*Dictionaries
	err := store.DB().Preload(clause.Associations).Where("id in (?)", ids).Find(&m).Error
	return m, err
}

func GetAllDictionaries() (list []*Dictionaries, err error) {
	err = store.DB().Find(&list).Error
	return
}

func ExportAllDictionaries(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := store.DB().Model(&Dictionaries{})
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
