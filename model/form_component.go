package model

import (
	"github.com/CloudSilk/usercenter/internal/formcomponent"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type FormComponent = formcomponent.FormComponent
type FormComponentResource = formcomponent.FormComponentResource

func CreateFormComponent(m *FormComponent) (string, error) { return formcomponent.CreateFormComponent(m) }
func UpdateFormComponent(m *FormComponent) error           { return formcomponent.UpdateFormComponent(m) }
func DeleteFormComponent(id string) error                  { return formcomponent.DeleteFormComponent(id) }
func QueryFormComponent(req *apipb.QueryFormComponentRequest, resp *apipb.QueryFormComponentResponse, preload bool) {
	formcomponent.QueryFormComponent(req, resp, preload)
}
func GetFormComponentByID(id string) (*FormComponent, error) { return formcomponent.GetFormComponentByID(id) }
func PBToFormComponents(in []*apipb.FormComponentInfo) []*FormComponent {
	return formcomponent.PBToFormComponents(in)
}
func PBToFormComponent(in *apipb.FormComponentInfo) *FormComponent {
	return formcomponent.PBToFormComponent(in)
}
func FormComponentsToPB(in []*FormComponent) []*apipb.FormComponentInfo {
	return formcomponent.FormComponentsToPB(in)
}
func FormComponentToPB(in *FormComponent) *apipb.FormComponentInfo {
	return formcomponent.FormComponentToPB(in)
}
func PBToFormComponentResource(in *apipb.FormComponentResource) *FormComponentResource {
	return formcomponent.PBToFormComponentResource(in)
}
func FormComponentResourceToPB(in *FormComponentResource) *apipb.FormComponentResource {
	return formcomponent.FormComponentResourceToPB(in)
}
