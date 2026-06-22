package model

import (
	"github.com/CloudSilk/usercenter/internal/tenant"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type Tenant = tenant.Tenant
type TenantMenu = tenant.TenantMenu
type TenantCertificate = tenant.TenantCertificate

func CreateTenant(m *Tenant) error { return tenant.CreateTenant(m) }
func UpdateTenant(m *Tenant) error { return tenant.UpdateTenant(m) }
func QueryTenant(req *apipb.QueryTenantRequest, resp *apipb.QueryTenantResponse) {
	tenant.QueryTenant(req, resp)
}
func GetAllTenant() ([]*Tenant, error) { return tenant.GetAllTenant() }
func GetTenantByID(id string) (*Tenant, error) { return tenant.GetTenantByID(id) }
func ExportAllTenants(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	tenant.ExportAllTenants(req, resp)
}
// DeleteTenant 注入 statistic 回调,解耦 user/role 循环依赖
func DeleteTenant(id string) error {
	return tenant.DeleteTenant(id, StatisticUserCount, StatisticRoleCount)
}
func CopyTenant(id string) error { return tenant.CopyTenant(id) }
func EnableTenant(id string, enable bool) error { return tenant.EnableTenant(id, enable) }
func StatisticTenantCount() (int64, error) { return tenant.StatisticTenantCount() }

// getTenantUserCount 内部委托(供 user.go CreateUser 调用)
func getTenantUserCount(tenantID string) (bool, int32, error) { return tenant.GetTenantUserCount(tenantID) }

// getTenantRoleCount 内部委托(供 role.go CreateRole 调用)
func getTenantRoleCount(tenantID string) (bool, int32, error) { return tenant.GetTenantUserCount(tenantID) }

func PBToTenantMenus(in []*apipb.TenantMenu) []*TenantMenu { return tenant.PBToTenantMenus(in) }
func TenantMenusToPB(in []*TenantMenu) []*apipb.TenantMenu { return tenant.TenantMenusToPB(in) }
func PBToTenantCertificate(in *apipb.TenantCertificate) *TenantCertificate {
	return tenant.PBToTenantCertificate(in)
}
func TenantCertificateToPB(in *TenantCertificate) *apipb.TenantCertificate {
	return tenant.TenantCertificateToPB(in)
}
func PBToTenant(in *apipb.TenantInfo) *Tenant       { return tenant.PBToTenant(in) }
func TenantToPB(in *Tenant) *apipb.TenantInfo       { return tenant.TenantToPB(in) }
func TenantsToPB(in []*Tenant) []*apipb.TenantInfo  { return tenant.TenantsToPB(in) }
