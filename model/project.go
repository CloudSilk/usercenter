package model

import (
	"encoding/json"
	"errors"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm/clause"
)

func CreateProject(m *Project) (string, error) {
	err := dbClient.DB().Create(m).Error
	return m.ID, err
}

func UpdateProject(m *Project) error {
	return dbClient.DB().Omit("created_at", "tenant_id").Save(m).Error
}

func DeleteProject(id string) (err error) {
	return dbClient.DB().Delete(&Project{}, "id=?", id).Error
}

func QueryProject(req *apipb.QueryProjectRequest, resp *apipb.QueryProjectResponse, preload bool) {
	db := dbClient.DB().Model(&Project{})
	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}

	if req.Name != "" {
		db = db.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	if len(req.Ids) > 0 {
		db = db.Where("id in ?", req.Ids)
	}
	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`name`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}

	var list []*Project
	resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = ProjectsToPB(list)
	}
	resp.Total = resp.Records
}

func GetProjectByID(id string) (*Project, error) {
	m := &Project{}
	err := dbClient.DB().Preload("FormComponents").Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

func ExportAllProjects(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := dbClient.DB().Model(&Language{}).Preload("FormComponents").Preload(clause.Associations)

	if req.ProjectID != "" {
		db = db.Where("id = ?", req.ProjectID)
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

func UpdateProjectAll(m *Project) error {
	duplication, err := dbClient.UpdateWithCheckDuplicationAndOmit(dbClient.DB(), m, true, []string{"created_at"}, "id != ? and  name=? ", m.ID, m.Name)
	if err != nil {
		return err
	}
	if duplication {
		return errors.New("存在相同项目管理")
	}

	return nil
}

func GetAllProjects() (list []*Project, err error) {
	err = dbClient.DB().Find(&list).Error
	return
}

type Project struct {
	commonmodel.Model
	TenantID       string                  `json:"tenantID" gorm:"size:36;index" `
	Name           string                  `json:"name" gorm:"size:200;index" `
	FormCount      int32                   `json:"formCount" gorm:"comment:可以创建的表单数量,-1为不限制" `
	PageCount      int32                   `json:"pageCount" gorm:"comment:可以创建的页面数量,-1为不限制" `
	Expired        time.Time               `json:"expired" gorm:"comment:项目继续维护表单和页面的过期时间" `
	Description    string                  `json:"description" gorm:"size:500" `
	CellCount      int32                   `json:"cellCount" gorm:"" `
	FormComponents []*ProjectFormComponent `json:"formComponents" gorm:"size:36" `
	IsMust         bool                    `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

type ProjectFormComponent struct {
	commonmodel.Model
	ProjectID string `json:"projectID" gorm:"size:36;index" `
	Name      string `json:"name" gorm:"size:200;index" `
}

func PBToProjects(in []*apipb.ProjectInfo) []*Project {
	var result []*Project
	for _, c := range in {
		result = append(result, PBToProject(c))
	}
	return result
}

func PBToProject(in *apipb.ProjectInfo) *Project {
	if len(in.Expired) == 10 {
		in.Expired = in.Expired + " 23:59:59"
	}
	return &Project{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		TenantID:       in.TenantID,
		Name:           in.Name,
		FormCount:      in.FormCount,
		PageCount:      in.PageCount,
		Expired:        utils.ParseTime(in.Expired),
		Description:    in.Description,
		CellCount:      in.CellCount,
		FormComponents: PBToProjectFormComponents(in.FormComponents),
		IsMust:         in.IsMust,
	}
}

func ProjectsToPB(in []*Project) []*apipb.ProjectInfo {
	var list []*apipb.ProjectInfo
	for _, f := range in {
		list = append(list, ProjectToPB(f))
	}
	return list
}

func ProjectToPB(in *Project) *apipb.ProjectInfo {
	return &apipb.ProjectInfo{
		Id:             in.ID,
		TenantID:       in.TenantID,
		Name:           in.Name,
		FormCount:      in.FormCount,
		PageCount:      in.PageCount,
		Expired:        utils.FormatTime(in.Expired),
		Description:    in.Description,
		CellCount:      in.CellCount,
		FormComponents: ProjectFormComponentsToPB(in.FormComponents),
		IsMust:         in.IsMust,
	}
}

func PBToProjectFormComponents(in []*apipb.ProjectFormComponent) []*ProjectFormComponent {
	var result []*ProjectFormComponent
	for _, c := range in {
		result = append(result, PBToProjectFormComponent(c))
	}
	return result
}

func PBToProjectFormComponent(in *apipb.ProjectFormComponent) *ProjectFormComponent {
	return &ProjectFormComponent{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		ProjectID: in.ProjectID,
		Name:      in.Name,
	}
}

func ProjectFormComponentsToPB(in []*ProjectFormComponent) []*apipb.ProjectFormComponent {
	var list []*apipb.ProjectFormComponent
	for _, f := range in {
		list = append(list, ProjectFormComponentToPB(f))
	}
	return list
}

func ProjectFormComponentToPB(in *ProjectFormComponent) *apipb.ProjectFormComponent {
	return &apipb.ProjectFormComponent{
		Id:        in.ID,
		ProjectID: in.ProjectID,
		Name:      in.Name,
	}
}
