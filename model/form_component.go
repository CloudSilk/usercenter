package model

import (
	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func CreateFormComponent(m *FormComponent) (string, error) {
	err := dbClient.DB().Create(m).Error
	return m.ID, err
}

func UpdateFormComponent(m *FormComponent) error {
	return dbClient.DB().Session(&gorm.Session{FullSaveAssociations: true}).Omit("created_at").Save(m).Error
}

func DeleteFormComponent(id string) (err error) {
	return dbClient.DB().Delete(&FormComponent{}, "id=?", id).Error
}

func QueryFormComponent(req *apipb.QueryFormComponentRequest, resp *apipb.QueryFormComponentResponse, preload bool) {
	db := dbClient.DB().Model(&FormComponent{})
	if req.Name != "" {
		db = db.Where("name LIKE ?", "%"+req.Name+"%")
	}

	if req.Group != "" {
		db = db.Where("`group` = ?", req.Group)
	}

	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`index`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}

	var list []*FormComponent
	resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = FormComponentsToPB(list)
	}
	resp.Total = resp.Records
}

func GetFormComponentByID(id string) (*FormComponent, error) {
	m := &FormComponent{}
	err := dbClient.DB().Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

type FormComponent struct {
	commonmodel.Model
	Name            string `json:"name" gorm:"size:200;index"`
	Title           string `gorm:"size:200"`
	Group           string `gorm:"size:100;index"`
	Index           int32
	Description     string `gorm:"size:1000"`
	Extends         string `gorm:"size:200"`
	Selector        string `gorm:"size:200"`
	DesignerProps   string
	DesignerLocales string
	Resource        *FormComponentResource
	Byo             bool `gorm:"comment:formily自带"`
}

type FormComponentResource struct {
	commonmodel.Model
	FormComponentID string `gorm:"size:36;index"`
	Icon            string `gorm:"size:500"`
	Thumb           string `gorm:"size:500"`
	Title           string `gorm:"size:200"`
	Description     string `gorm:"size:1000"`
	Span            int32
	Elements        string
}

func PBToFormComponents(in []*apipb.FormComponentInfo) []*FormComponent {
	var result []*FormComponent
	for _, c := range in {
		result = append(result, PBToFormComponent(c))
	}
	return result
}

func PBToFormComponent(in *apipb.FormComponentInfo) *FormComponent {
	return &FormComponent{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		Name:            in.Name,
		Title:           in.Title,
		Group:           in.Group,
		Index:           in.Index,
		Description:     in.Description,
		Extends:         in.Extends,
		Selector:        in.Selector,
		DesignerProps:   in.DesignerProps,
		DesignerLocales: in.DesignerLocales,
		Byo:             in.Byo,
		Resource:        PBToFormComponentResource(in.Resource),
	}
}

func FormComponentsToPB(in []*FormComponent) []*apipb.FormComponentInfo {
	var list []*apipb.FormComponentInfo
	for _, f := range in {
		list = append(list, FormComponentToPB(f))
	}
	return list
}

func FormComponentToPB(in *FormComponent) *apipb.FormComponentInfo {
	return &apipb.FormComponentInfo{
		Id:              in.ID,
		Name:            in.Name,
		Title:           in.Title,
		Group:           in.Group,
		Index:           in.Index,
		Description:     in.Description,
		Extends:         in.Extends,
		Selector:        in.Selector,
		DesignerProps:   in.DesignerProps,
		DesignerLocales: in.DesignerLocales,
		Byo:             in.Byo,
		Resource:        FormComponentResourceToPB(in.Resource),
	}
}

func PBToFormComponentResource(in *apipb.FormComponentResource) *FormComponentResource {
	if in == nil {
		return nil
	}
	return &FormComponentResource{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		FormComponentID: in.FormComponentID,
		Icon:            in.Icon,
		Thumb:           in.Thumb,
		Title:           in.Title,
		Description:     in.Description,
		Span:            in.Span,
		Elements:        in.Elements,
	}
}

func FormComponentResourceToPB(in *FormComponentResource) *apipb.FormComponentResource {
	if in == nil {
		return nil
	}
	return &apipb.FormComponentResource{
		Id:              in.ID,
		FormComponentID: in.FormComponentID,
		Icon:            in.Icon,
		Thumb:           in.Thumb,
		Title:           in.Title,
		Description:     in.Description,
		Span:            in.Span,
		Elements:        in.Elements,
	}
}
