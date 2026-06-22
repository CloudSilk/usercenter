package model

import (
	"github.com/CloudSilk/usercenter/internal/permission"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type API = permission.API
type QueryAPIRequest = permission.QueryAPIRequest
type QueryAPIResponse = permission.QueryAPIResponse

func CreateAPI(api *API) error  { return permission.CreateAPI(api) }
func DeleteApi(id string) error { return permission.DeleteApi(id) }
func QueryAPI(req *apipb.QueryAPIRequest, resp *apipb.QueryAPIResponse) {
	permission.QueryAPI(req, resp)
}
func GetAllAPIs(req *apipb.QueryAPIRequest) ([]API, error) { return permission.GetAllAPIs(req) }
func GetAPIById(id string) (API, error)                    { return permission.GetAPIById(id) }
func UpdateAPI(api *API) error                             { return permission.UpdateAPI(api) }
func EnableAPI(id string, enable bool) error               { return permission.EnableAPI(id, enable) }
func ExportAllApis(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	permission.ExportAllApis(req, resp)
}
func PBToAPI(in *apipb.APIInfo) *API     { return permission.PBToAPI(in) }
func APIToPB(in *API) *apipb.APIInfo     { return permission.APIToPB(in) }
func APIsToPB(in []API) []*apipb.APIInfo { return permission.APIsToPB(in) }
func updateNotCheckAuthRule()            { permission.UpdateNotCheckAuthRule() }
func updateNotCheckLoginRule()           { permission.UpdateNotCheckLoginRule() }
