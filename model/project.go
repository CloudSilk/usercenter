package model

import (
	"github.com/CloudSilk/usercenter/internal/project"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type Project = project.Project
type ProjectFormComponent = project.ProjectFormComponent

func CreateProject(m *Project) (string, error)   { return project.CreateProject(m) }
func UpdateProject(m *Project) error              { return project.UpdateProject(m) }
func DeleteProject(id string) error               { return project.DeleteProject(id) }
func QueryProject(req *apipb.QueryProjectRequest, resp *apipb.QueryProjectResponse, preload bool) {
	project.QueryProject(req, resp, preload)
}
func GetProjectByID(id string) (*Project, error) { return project.GetProjectByID(id) }
func ExportAllProjects(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	project.ExportAllProjects(req, resp)
}
func UpdateProjectAll(m *Project) error            { return project.UpdateProjectAll(m) }
func GetAllProjects() (list []*Project, err error) { return project.GetAllProjects() }
func PBToProjects(in []*apipb.ProjectInfo) []*Project { return project.PBToProjects(in) }
func PBToProject(in *apipb.ProjectInfo) *Project       { return project.PBToProject(in) }
func ProjectsToPB(in []*Project) []*apipb.ProjectInfo  { return project.ProjectsToPB(in) }
func ProjectToPB(in *Project) *apipb.ProjectInfo       { return project.ProjectToPB(in) }
func PBToProjectFormComponents(in []*apipb.ProjectFormComponent) []*ProjectFormComponent {
	return project.PBToProjectFormComponents(in)
}
func PBToProjectFormComponent(in *apipb.ProjectFormComponent) *ProjectFormComponent {
	return project.PBToProjectFormComponent(in)
}
func ProjectFormComponentsToPB(in []*ProjectFormComponent) []*apipb.ProjectFormComponent {
	return project.ProjectFormComponentsToPB(in)
}
func ProjectFormComponentToPB(in *ProjectFormComponent) *apipb.ProjectFormComponent {
	return project.ProjectFormComponentToPB(in)
}
