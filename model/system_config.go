package model

import (
	"github.com/CloudSilk/usercenter/internal/systemconfig"
	apipb "github.com/CloudSilk/usercenter/proto"
)

// SystemConfig 系统配置(定义已迁至 internal/systemconfig,此处为兼容别名)。
// model 历史调用方(http/system_config.go、init.go AutoMigrate、其他按 key 读写)
// 继续使用 model.SystemConfig / model.GetSystemConfigByKey 等,无需改动。
type SystemConfig = systemconfig.SystemConfig

// 以下函数委托 internal/systemconfig(REDESIGN §4 阶段0),向后兼容。

func CreateSystemConfig(m *SystemConfig) (string, error) { return systemconfig.CreateSystemConfig(m) }
func UpdateSystemConfig(m *SystemConfig) error           { return systemconfig.UpdateSystemConfig(m) }
func DeleteSystemConfig(id string) error                 { return systemconfig.DeleteSystemConfig(id) }
func QuerySystemConfig(req *apipb.QuerySystemConfigRequest, resp *apipb.QuerySystemConfigResponse, preload bool) {
	systemconfig.QuerySystemConfig(req, resp, preload)
}
func GetSystemConfigByID(id string) (*SystemConfig, error)       { return systemconfig.GetSystemConfigByID(id) }
func GetSystemConfigByIDs(ids []string) ([]*SystemConfig, error) { return systemconfig.GetSystemConfigByIDs(ids) }
func GetAllSystemConfigs() (list []*SystemConfig, err error)     { return systemconfig.GetAllSystemConfigs() }
func ExportAllSystemConfigs(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	systemconfig.ExportAllSystemConfigs(req, resp)
}
func GetSystemConfigByKey(key string) (*SystemConfig, error) { return systemconfig.GetSystemConfigByKey(key) }
func GetSystemConfigsByKeys(keys []string) ([]*SystemConfig, error) {
	return systemconfig.GetSystemConfigsByKeys(keys)
}
func GetSystemConfigMapByKeys(keys []string) (map[string]string, error) {
	return systemconfig.GetSystemConfigMapByKeys(keys)
}
func UpsertSystemConfigByKey(key, value string) error {
	return systemconfig.UpsertSystemConfigByKey(key, value)
}
func PBToSystemConfigs(in []*apipb.SystemConfigInfo) []*SystemConfig {
	return systemconfig.PBToSystemConfigs(in)
}
func PBToSystemConfig(in *apipb.SystemConfigInfo) *SystemConfig {
	return systemconfig.PBToSystemConfig(in)
}
func SystemConfigsToPB(in []*SystemConfig) []*apipb.SystemConfigInfo {
	return systemconfig.SystemConfigsToPB(in)
}
func SystemConfigToPB(in *SystemConfig) *apipb.SystemConfigInfo {
	return systemconfig.SystemConfigToPB(in)
}
