package model

import (
	"github.com/CloudSilk/usercenter/internal/app"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type APP = app.APP
type APPProp = app.APPProp
type QueryAPPRequest = app.QueryAPPRequest
type QueryAPPResponse = app.QueryAPPResponse

func CreateAPP(md *APP) error                      { return app.CreateAPP(md) }
func DeleteAPP(id string) error                    { return app.DeleteAPP(id) }
func QueryAPP(req *QueryAPPRequest, resp *QueryAPPResponse) { app.QueryAPP(req, resp) }
func GetAllAPPs() ([]*APP, error)                  { return app.GetAllAPPs() }
func GetAPPById(id string) (APP, error)            { return app.GetAPPById(id) }
func UpdateAPP(md *APP) error                      { return app.UpdateAPP(md) }
func ExportAllAPPs(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	app.ExportAllAPPs(req, resp)
}
