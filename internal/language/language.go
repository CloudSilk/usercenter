package language

import (
	"encoding/json"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm/clause"
)

type Language struct {
	commonmodel.Model
	Chinese     string `json:"chinese" gorm:"size:200"`
	Description string `json:"description" gorm:"size:400"`
	English     string `json:"english" gorm:"size:200"`
	Group       string `json:"group" gorm:"size:100"`
	Japan       string `json:"japan" gorm:"size:200"`
	Name        string `json:"name" gorm:"size:100"`
	System      string `json:"system" gorm:"size:50;comment:Admin-管理后台 APP"`
	WebSiteID   string `json:"webSiteID" gorm:"size:36;index"`
	TenantID    string `json:"tenantID" gorm:"size:36;index"`
	ProjectID   string `json:"projectID" gorm:"index;size:36"`
	IsMust      bool   `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

func CreateLanguage(m *Language) (string, error) {
	err := store.DB().Create(m).Error
	return m.ID, err
}

func UpdateLanguage(m *Language) error {
	return store.DB().Omit("created_at").Save(m).Error
}

func DeleteLanguage(id string) (err error) {
	return store.DB().Delete(&Language{}, "id=?", id).Error
}

func QueryLanguage(req *apipb.QueryLanguageRequest, resp *apipb.QueryLanguageResponse, preload bool) {
	db := store.DB().Model(&Language{})
	if req.WebSiteID != "" {
		db = db.Where("web_site_id = ?", req.WebSiteID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}

	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`name`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}

	var list []*Language
	resp.Records, resp.Pages, err = store.Client().PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = LanguagesToPB(list)
	}
	resp.Total = resp.Records
}

func GetLanguageByID(id string) (*Language, error) {
	m := &Language{}
	err := store.DB().Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

func GetLanguageByIDs(ids []string) ([]*Language, error) {
	var m []*Language
	err := store.DB().Preload(clause.Associations).Where("id in (?)", ids).Find(&m).Error
	return m, err
}

func GetAllLanguages() (list []*Language, err error) {
	err = store.DB().Find(&list).Error
	return
}

func ExportAllLanguages(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := store.DB().Model(&Language{})
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	var list []*Language
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

func PBToLanguages(in []*apipb.LanguageInfo) []*Language {
	var result []*Language
	for _, c := range in {
		result = append(result, PBToLanguage(c))
	}
	return result
}

func PBToLanguage(in *apipb.LanguageInfo) *Language {
	if in == nil {
		return nil
	}
	return &Language{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		Chinese:     in.Chinese,
		Description: in.Description,
		English:     in.English,
		Group:       in.Group,
		Japan:       in.Japan,
		Name:        in.Name,
		System:      in.System,
		WebSiteID:   in.WebSiteID,
		TenantID:    in.TenantID,
		ProjectID:   in.ProjectID,
		IsMust:      in.IsMust,
	}
}

func LanguagesToPB(in []*Language) []*apipb.LanguageInfo {
	var list []*apipb.LanguageInfo
	for _, f := range in {
		list = append(list, LanguageToPB(f))
	}
	return list
}

func LanguageToPB(in *Language) *apipb.LanguageInfo {
	if in == nil {
		return nil
	}
	return &apipb.LanguageInfo{
		Id:          in.ID,
		Chinese:     in.Chinese,
		Description: in.Description,
		English:     in.English,
		Group:       in.Group,
		Japan:       in.Japan,
		Name:        in.Name,
		System:      in.System,
		WebSiteID:   in.WebSiteID,
		TenantID:    in.TenantID,
		ProjectID:   in.ProjectID,
		IsMust:      in.IsMust,
	}
}
