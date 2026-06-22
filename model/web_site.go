package model

import (
	"github.com/CloudSilk/usercenter/internal/website"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type WebSite = website.WebSite

func CreateWebSite(m *WebSite) (string, error)   { return website.CreateWebSite(m) }
func UpdateWebSite(m *WebSite) error              { return website.UpdateWebSite(m) }
func DeleteWebSite(id string) error               { return website.DeleteWebSite(id) }
func QueryWebSite(req *apipb.QueryWebSiteRequest, resp *apipb.QueryWebSiteResponse, preload bool) {
	website.QueryWebSite(req, resp, preload)
}
func GetWebSiteByID(id string) (*WebSite, error)       { return website.GetWebSiteByID(id) }
func GetWebSiteByIDs(ids []string) ([]*WebSite, error) { return website.GetWebSiteByIDs(ids) }
func GetAllWebSites() (list []*WebSite, err error)     { return website.GetAllWebSites() }
func ExportAllWebSites(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	website.ExportAllWebSites(req, resp)
}
func PBToWebSites(in []*apipb.WebSiteInfo) []*WebSite { return website.PBToWebSites(in) }
func PBToWebSite(in *apipb.WebSiteInfo) *WebSite       { return website.PBToWebSite(in) }
func WebSitesToPB(in []*WebSite) []*apipb.WebSiteInfo  { return website.WebSitesToPB(in) }
func WebSiteToPB(in *WebSite) *apipb.WebSiteInfo       { return website.WebSiteToPB(in) }
