package website

import (
	"encoding/json"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm/clause"
)

type WebSite struct {
	commonmodel.Model
	Name      string `json:"name" gorm:"size:200;index"`
	Code      string `json:"code" gorm:"size:100;uniqueindex:WebSite_uidx1"`
	Logo      string `json:"logo" gorm:"size:36"`
	ProjectID string `json:"projectID" gorm:"size:36"`
	TenantID  string `json:"tenantID" gorm:"size:36;index"`
	IsMust    bool   `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

func CreateWebSite(m *WebSite) (string, error) {
	err := store.DB().Create(m).Error
	return m.ID, err
}

func UpdateWebSite(m *WebSite) error {
	return store.DB().Omit("created_at").Save(m).Error
}

func DeleteWebSite(id string) (err error) {
	return store.DB().Delete(&WebSite{}, "id=?", id).Error
}

func QueryWebSite(req *apipb.QueryWebSiteRequest, resp *apipb.QueryWebSiteResponse, preload bool) {
	db := store.DB().Model(&WebSite{})
	if req.Name != "" {
		db = db.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Code != "" {
		db = db.Where("code = ?", req.Code)
	}
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
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

	var list []*WebSite
	resp.Records, resp.Pages, err = store.Client().PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = WebSitesToPB(list)
	}
	resp.Total = resp.Records
}

func GetWebSiteByID(id string) (*WebSite, error) {
	m := &WebSite{}
	err := store.DB().Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

func GetWebSiteByIDs(ids []string) ([]*WebSite, error) {
	var m []*WebSite
	err := store.DB().Preload(clause.Associations).Where("id in (?)", ids).Find(&m).Error
	return m, err
}

func GetAllWebSites() (list []*WebSite, err error) {
	err = store.DB().Find(&list).Error
	return
}

func ExportAllWebSites(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := store.DB().Model(&WebSite{})
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	var list []*WebSite
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

func PBToWebSites(in []*apipb.WebSiteInfo) []*WebSite {
	var result []*WebSite
	for _, c := range in {
		result = append(result, PBToWebSite(c))
	}
	return result
}

func PBToWebSite(in *apipb.WebSiteInfo) *WebSite {
	if in == nil {
		return nil
	}
	return &WebSite{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		Name:      in.Name,
		Code:      in.Code,
		Logo:      in.Logo,
		ProjectID: in.ProjectID,
		TenantID:  in.TenantID,
		IsMust:    in.IsMust,
	}
}

func WebSitesToPB(in []*WebSite) []*apipb.WebSiteInfo {
	var list []*apipb.WebSiteInfo
	for _, f := range in {
		list = append(list, WebSiteToPB(f))
	}
	return list
}

func WebSiteToPB(in *WebSite) *apipb.WebSiteInfo {
	if in == nil {
		return nil
	}
	return &apipb.WebSiteInfo{
		Id:        in.ID,
		Name:      in.Name,
		Code:      in.Code,
		Logo:      in.Logo,
		ProjectID: in.ProjectID,
		TenantID:  in.TenantID,
		IsMust:    in.IsMust,
	}
}
