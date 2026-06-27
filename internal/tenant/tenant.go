package tenant

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/usercenter/internal/alert"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Tenant struct {
	commonmodel.Model
	Name          string `json:"name" validate:"required" gorm:"index;size:200"`
	Contact       string `json:"contact" gorm:"size:100"`
	CellPhone     string `json:"cellPhone" gorm:"size:50"`
	Address       string `json:"address" gorm:"size:200"`
	BusinessScope string `json:"businessScope" gorm:"size:200"`
	AreaCovered   string `json:"areaCovered" gorm:"size:100"`
	StaffSize     int32  `json:"staffSize"`
	Enable        bool   `json:"enable" gorm:"index"`
	Province      string `gorm:"index;size:12"`
	City          string `gorm:"index;size:12"`
	Area          string `gorm:"index;size:12"`
	Town          string `gorm:"index;size:12"`
	UserCount     int32
	RoleCount     int32
	ProjectCount  int32
	Expired       time.Time
	TenantMenus   []*TenantMenu
	Certificate   *TenantCertificate
	IsMust        bool `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

func (t *Tenant) GetAuthorizedMenu() map[string]*TenantMenu {
	authiruzedMenus := make(map[string]*TenantMenu)
	for _, m := range t.TenantMenus {
		oldMenu, ok := authiruzedMenus[m.MenuID]
		if ok {
			if oldMenu.Funcs == "" {
				oldMenu.Funcs = m.Funcs
			} else if m.Funcs != "" {
				oldMenu.Funcs += "," + m.Funcs
			}
			continue
		}
		authiruzedMenus[m.MenuID] = m
	}
	return authiruzedMenus
}

type TenantMenu struct {
	commonmodel.Model
	TenantID string `json:"tenantID" gorm:"index;comment:租户ID"`
	MenuID   string `json:"menuID" gorm:"index;comment:菜单ID"`
	Funcs    string `json:"funcs" gorm:"size:500;comment:功能名称,多个以逗号隔开"`
	Menu     *permission.Menu `json:"menu"`
}

func (r *TenantMenu) GetMenuID() string { return r.MenuID }
func (r *TenantMenu) GetFuncs() []string { return strings.Split(r.Funcs, ",") }
func (r *TenantMenu) GetShow() bool      { return true }

type TenantCertificate struct {
	commonmodel.Model
	TenantID   string `json:"tenantID" gorm:"index;comment:租户ID"`
	PrivateKey string `gorm:"size:1000"`
	PublicKey  string `gorm:"size:1000"`
}

func CreateTenant(m *Tenant) error {
	duplication, err := store.Client().CreateWithCheckDuplication(m, " name =? ", m.Name)
	if err != nil {
		return err
	}
	if duplication {
		return errors.New("存在相同租户")
	}
	alert.FireEvent("tenant.created", map[string]interface{}{
		"id": m.ID, "name": m.Name,
	})
	return nil
}

func UpdateTenant(newTenant *Tenant) error {
	return store.DB().Transaction(func(tx *gorm.DB) error {
		oldTenant := &Tenant{}
		err := tx.Preload("TenantMenus").Preload(clause.Associations).Where("id = ?", newTenant.ID).First(oldTenant).Error
		if err != nil {
			return err
		}
		var deleteTenantMenu []string
		for _, oldTM := range oldTenant.TenantMenus {
			flag := false
			for _, newTM := range newTenant.TenantMenus {
				if newTM.ID == oldTM.ID {
					flag = true
				}
			}
			if !flag {
				deleteTenantMenu = append(deleteTenantMenu, oldTM.ID)
			}
		}
		if len(deleteTenantMenu) > 0 {
			err = tx.Unscoped().Delete(&TenantMenu{}, "id in ?", deleteTenantMenu).Error
			if err != nil {
				return err
			}
		}
		duplication, err := store.Client().UpdateWithCheckDuplicationAndOmit(tx, newTenant, true, []string{"created_at"}, "id != ?  and  name =? ", newTenant.ID, newTenant.Name)
		if err != nil {
			return err
		}
		if duplication {
			return errors.New("存在相同租户")
		}
		return nil
	})
}

func QueryTenant(req *apipb.QueryTenantRequest, resp *apipb.QueryTenantResponse) {
	db := store.DB().Model(&Tenant{})
	if req.Name != "" {
		db = db.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Province != "" {
		db = db.Where("province = ?", req.Province)
	}
	if req.City != "" {
		db = db.Where("city = ?", req.City)
	}
	if req.Area != "" {
		db = db.Where("area = ?", req.Area)
	}
	if req.Town != "" {
		db = db.Where("town = ?", req.Town)
	}
	if len(req.Ids) > 0 {
		db = db.Where("id in ?", req.Ids)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`name`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}
	var tenants []*Tenant
	resp.Records, resp.Pages, err = store.Client().PageQuery(db, req.PageSize, req.PageIndex, orderStr, &tenants, nil)
	if err != nil {
		resp.Code = commonmodel.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = TenantsToPB(tenants)
	}
	resp.Total = resp.Records
}

func GetAllTenant() (list []*Tenant, err error) {
	err = store.DB().Find(&list).Error
	return
}

func GetTenantByID(id string) (*Tenant, error) {
	m := &Tenant{}
	err := store.DB().Preload("TenantMenus").Preload("Certificate").Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

func ExportAllTenants(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := store.DB().Model(&Tenant{}).Preload("TenantMenus").Preload("Certificate").Preload(clause.Associations)
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	var list []*Tenant
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

// DeleteTenant 删除租户。userCountFn/roleCountFn 通过回调注入,
// 解除 tenant 对 user/role 的直接依赖(循环依赖接口化)。
type UserCountFn func(t int, tenantID, group string) (int64, error)
type RoleCountFn func(tenantID string) (int64, error)

func DeleteTenant(id string, userCountFn UserCountFn, roleCountFn RoleCountFn) (err error) {
	userCount, err := userCountFn(0, id, "")
	if err != nil {
		return err
	}
	if userCount > 0 {
		return errors.New("请先删除关联的用户")
	}
	roleCount, err := roleCountFn(id)
	if err != nil {
		return err
	}
	if roleCount > 0 {
		return errors.New("请先删除关联的角色")
	}
	return store.DB().Delete(&Tenant{}, "id=?", id).Error
}

func CopyTenant(id string) error {
	from, err := GetTenantByID(id)
	if err != nil {
		return err
	}
	to := &Tenant{}
	// 简单复制字段(弃用 copier 反射库,显式映射)
	to.Name = from.Name + " Copy"
	to.Contact = from.Contact
	to.CellPhone = from.CellPhone
	to.Address = from.Address
	to.BusinessScope = from.BusinessScope
	to.AreaCovered = from.AreaCovered
	to.StaffSize = from.StaffSize
	to.Enable = from.Enable
	to.Province = from.Province
	to.City = from.City
	to.Area = from.Area
	to.Town = from.Town
	to.UserCount = from.UserCount
	to.RoleCount = from.RoleCount
	to.ProjectCount = from.ProjectCount
	to.Expired = from.Expired
	return CreateTenant(to)
}

func EnableTenant(id string, enable bool) error {
	return store.DB().Model(&Tenant{}).Where("id=?", id).Update("enable", enable).Error
}

func StatisticTenantCount() (int64, error) {
	var count int64
	err := store.DB().Model(&Tenant{}).Count(&count).Error
	return count, err
}

func GetTenantUserCount(tenantID string) (bool, int32, error) {
	m := &Tenant{}
	err := store.DB().Where("id = ?", tenantID).First(m).Error
	if err != nil {
		return true, 0, err
	}
	return !m.Expired.After(time.Now()), m.UserCount, nil
}

// --- PB 转换 ---

func PBToTenantMenus(tenantMenus []*apipb.TenantMenu) []*TenantMenu {
	var list []*TenantMenu
	for _, tm := range tenantMenus {
		list = append(list, &TenantMenu{
			Model:    commonmodel.Model{ID: tm.Id},
			TenantID: tm.TenantID,
			MenuID:   tm.MenuID,
			Funcs:    tm.Funcs,
			Menu:     permission.PBToMenu(tm.Menu),
		})
	}
	return list
}

func TenantMenusToPB(tenantMenus []*TenantMenu) []*apipb.TenantMenu {
	var list []*apipb.TenantMenu
	for _, tm := range tenantMenus {
		list = append(list, &apipb.TenantMenu{
			Id:       tm.ID,
			TenantID: tm.TenantID,
			MenuID:   tm.MenuID,
			Funcs:    tm.Funcs,
			Menu:     permission.MenuToPB(tm.Menu),
		})
	}
	return list
}

func PBToTenantCertificate(in *apipb.TenantCertificate) *TenantCertificate {
	if in == nil {
		return nil
	}
	return &TenantCertificate{
		Model:      commonmodel.Model{ID: in.Id},
		TenantID:   in.TenantID,
		PrivateKey: in.PrivateKey,
		PublicKey:  in.PublicKey,
	}
}

func TenantCertificateToPB(in *TenantCertificate) *apipb.TenantCertificate {
	if in == nil {
		return nil
	}
	return &apipb.TenantCertificate{
		Id:         in.ID,
		TenantID:   in.TenantID,
		PrivateKey: in.PrivateKey,
		PublicKey:  in.PublicKey,
	}
}

func PBToTenant(in *apipb.TenantInfo) *Tenant {
	if in == nil {
		return nil
	}
	if len(in.Expired) == 10 {
		in.Expired = in.Expired + " 15:59:59"
	}
	return &Tenant{
		Model:          commonmodel.Model{ID: in.Id},
		Name:           in.Name,
		Contact:        in.Contact,
		CellPhone:      in.CellPhone,
		Address:        in.Address,
		BusinessScope:  in.BusinessScope,
		AreaCovered:    in.AreaCovered,
		StaffSize:      in.StaffSize,
		Enable:         in.Enable,
		Province:       in.Province,
		City:           in.City,
		Area:           in.Area,
		Town:           in.Town,
		UserCount:      in.UserCount,
		RoleCount:      in.RoleCount,
		ProjectCount:   in.ProjectCount,
		Expired:        utils.ParseTime(in.Expired),
		TenantMenus:    PBToTenantMenus(in.TenantMenus),
		Certificate:    PBToTenantCertificate(in.Certificate),
		IsMust:         in.IsMust,
	}
}

func TenantToPB(in *Tenant) *apipb.TenantInfo {
	if in == nil {
		return nil
	}
	return &apipb.TenantInfo{
		Id:            in.ID,
		Name:          in.Name,
		Contact:       in.Contact,
		CellPhone:     in.CellPhone,
		Address:       in.Address,
		BusinessScope: in.BusinessScope,
		AreaCovered:   in.AreaCovered,
		StaffSize:     in.StaffSize,
		Enable:        in.Enable,
		Province:      in.Province,
		City:          in.City,
		Area:          in.Area,
		Town:          in.Town,
		UserCount:     in.UserCount,
		RoleCount:     in.RoleCount,
		ProjectCount:  in.ProjectCount,
		Expired:       utils.FormatTime(in.Expired),
		TenantMenus:   TenantMenusToPB(in.TenantMenus),
		Certificate:   TenantCertificateToPB(in.Certificate),
		IsMust:        in.IsMust,
	}
}

func TenantsToPB(in []*Tenant) []*apipb.TenantInfo {
	var list []*apipb.TenantInfo
	for _, t := range in {
		list = append(list, TenantToPB(t))
	}
	return list
}
