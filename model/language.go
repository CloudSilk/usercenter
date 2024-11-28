package model

import (
	"encoding/json"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm/clause"
)

func CreateLanguage(m *Language) (string, error) {
	err := dbClient.DB().Create(m).Error
	return m.ID, err
}

func UpdateLanguage(m *Language) error {
	return dbClient.DB().Omit("created_at").Save(m).Error
}

func DeleteLanguage(id string) (err error) {
	return dbClient.DB().Delete(&Language{}, "id=?", id).Error
}

func QueryLanguage(req *apipb.QueryLanguageRequest, resp *apipb.QueryLanguageResponse, preload bool) {
	db := dbClient.DB().Model(&Language{})
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
	resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
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
	err := dbClient.DB().Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

func GetLanguageByIDs(ids []string) ([]*Language, error) {
	var m []*Language
	err := dbClient.DB().Preload(clause.Associations).Where("id in (?)", ids).Find(&m).Error
	return m, err
}

func GetAllLanguages() (list []*Language, err error) {
	err = dbClient.DB().Find(&list).Error
	return
}

func ExportAllLanguages(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := dbClient.DB().Model(&Language{})

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

type Language struct {
	commonmodel.Model
	//Chinese
	Chinese string `json:"chinese" gorm:"size:200" `
	//Description
	Description string `json:"description" gorm:"size:400" `
	//English
	English string `json:"english" gorm:"size:200" `
	//Group
	Group string `json:"group" gorm:"size:100" `
	//Japan
	Japan string `json:"japan" gorm:"size:200" `
	//Name
	Name string `json:"name" gorm:"size:100" `
	//System Admin-管理后台 APP
	System string `json:"system" gorm:"size:50;comment:Admin-管理后台 APP" `
	//所属站点
	WebSiteID string `json:"webSiteID" gorm:"size:36;index" `
	TenantID  string `json:"tenantID" gorm:"size:36;index" `
	ProjectID string `json:"projectID" gorm:"index;size:36"`
	IsMust    bool   `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
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
