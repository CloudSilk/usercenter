package model

import (
	"github.com/CloudSilk/usercenter/internal/wechatconfig"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type WechatConfig = wechatconfig.WechatConfig

func CreateWechatConfig(m *WechatConfig) (string, error) { return wechatconfig.CreateWechatConfig(m) }
func UpdateWechatConfig(m *WechatConfig) error           { return wechatconfig.UpdateWechatConfig(m) }
func DeleteWechatConfig(id string) error                 { return wechatconfig.DeleteWechatConfig(id) }
func QueryWechatConfig(req *apipb.QueryWechatConfigRequest, resp *apipb.QueryWechatConfigResponse, preload bool) {
	wechatconfig.QueryWechatConfig(req, resp, preload)
}
func GetWechatConfigByID(id string) (*WechatConfig, error) { return wechatconfig.GetWechatConfigByID(id) }
func GetWechatConfigByIDs(ids []string) ([]*WechatConfig, error) {
	return wechatconfig.GetWechatConfigByIDs(ids)
}
func GetAllWechatConfigs() (list []*WechatConfig, err error) { return wechatconfig.GetAllWechatConfigs() }
func ExportAllWechatConfigs(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	wechatconfig.ExportAllWechatConfigs(req, resp)
}
func PBToWechatConfigs(in []*apipb.WechatConfigInfo) []*WechatConfig {
	return wechatconfig.PBToWechatConfigs(in)
}
func PBToWechatConfig(in *apipb.WechatConfigInfo) *WechatConfig {
	return wechatconfig.PBToWechatConfig(in)
}
func WechatConfigsToPB(in []*WechatConfig) []*apipb.WechatConfigInfo {
	return wechatconfig.WechatConfigsToPB(in)
}
func WechatConfigToPB(in *WechatConfig) *apipb.WechatConfigInfo {
	return wechatconfig.WechatConfigToPB(in)
}
