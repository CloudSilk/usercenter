package model

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/CloudSilk/pkg/model"
	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Tenant struct {
	model.Model
	// 租户名称
	// required: true
	// @inject_tag: validate:"required"
	Name string `json:"name" validate:"required" gorm:"index;size:200"`
	// 联系人
	Contact string `json:"contact" gorm:"size:100"`
	// 联系人电话
	CellPhone string `json:"cellPhone" gorm:"size:50"`
	// 地址
	Address string `json:"address" gorm:"size:200"`
	// 业务范围
	BusinessScope string `json:"businessScope" gorm:"size:200"`
	// 占地面积
	AreaCovered string `json:"areaCovered" gorm:"size:100"`
	// 人员规模
	StaffSize int32 `json:"staffSize"`
	Enable    bool  `json:"enable" gorm:"index"`
	//省份
	Province string `gorm:"index;size:12"`
	//城市
	City string `gorm:"index;size:12"`
	//区/县
	Area string `gorm:"index;size:12"`
	//街道/镇
	Town         string `gorm:"index;size:12"`
	UserCount    int32
	RoleCount    int32
	ProjectCount int32
	Expired      time.Time
	TenantMenus  []*TenantMenu
	Certificate  *TenantCertificate
	IsMust       bool `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

func (t *Tenant) getAuthorizedMenu() map[string]*TenantMenu {
	authiruzedMenus := make(map[string]*TenantMenu)
	for _, menu := range t.TenantMenus {
		oldMenu, ok := authiruzedMenus[menu.MenuID]
		if ok {
			if oldMenu.Funcs == "" {
				oldMenu.Funcs = menu.Funcs
			} else if menu.Funcs != "" {
				oldMenu.Funcs += "," + menu.Funcs
			}
			continue
		}
		authiruzedMenus[menu.MenuID] = menu
	}
	return authiruzedMenus
}

type TenantMenu struct {
	model.Model
	TenantID string `json:"tenantID" gorm:"index;comment:租户ID"`
	MenuID   string `json:"menuID" gorm:"index;comment:菜单ID"`
	Funcs    string `json:"funcs" gorm:"size:500;comment:功能名称,多个以逗号隔开"`
	Menu     *Menu  `json:"menu"`
}

func (r *TenantMenu) GetMenuID() string {
	return r.MenuID
}
func (r *TenantMenu) GetFuncs() []string {
	return strings.Split(r.Funcs, ",")
}
func (r *TenantMenu) GetShow() bool {
	return true
}

type TenantCertificate struct {
	model.Model
	TenantID   string `json:"tenantID" gorm:"index;comment:租户ID"`
	PrivateKey string `gorm:"size:1000"`
	PublicKey  string `gorm:"size:1000"`
}

func CreateTenant(m *Tenant) error {
	duplication, err := dbClient.CreateWithCheckDuplication(m, " name =? ", m.Name)
	if err != nil {
		return err
	}
	if duplication {
		return errors.New("存在相同租户")
	}
	return nil
}

func UpdateTenant(newTenant *Tenant) error {
	return dbClient.DB().Transaction(func(tx *gorm.DB) error {
		oldTenant := &Tenant{}
		err := tx.Preload("TenantMenus").Preload(clause.Associations).Where("id = ?", newTenant.ID).First(oldTenant).Error
		if err != nil {
			return err
		}

		var deleteTenantMenu []string
		for _, oldTenantMenu := range oldTenant.TenantMenus {
			flag := false
			for _, newTenantMenu := range newTenant.TenantMenus {
				if newTenantMenu.ID == oldTenantMenu.ID {
					flag = true
				}
			}
			if !flag {
				deleteTenantMenu = append(deleteTenantMenu, oldTenantMenu.ID)
			}
		}
		if len(deleteTenantMenu) > 0 {
			err = tx.Unscoped().Delete(&TenantMenu{}, "id in ?", deleteTenantMenu).Error
			if err != nil {
				return err
			}
		}

		duplication, err := dbClient.UpdateWithCheckDuplicationAndOmit(tx, newTenant, true, []string{"created_at"}, "id != ?  and  name =? ", newTenant.ID, newTenant.Name)
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
	db := dbClient.DB().Model(&Tenant{})
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
	resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &tenants, nil)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = TenantsToPB(tenants)
	}
	resp.Total = resp.Records
}

func GetAllTenant() (list []*Tenant, err error) {
	err = dbClient.DB().Find(&list).Error
	return
}

func GetTenantByID(id string) (*Tenant, error) {
	m := &Tenant{}
	err := dbClient.DB().Preload("TenantMenus").Preload("Certificate").Preload(clause.Associations).Where("id = ?", id).First(m).Error
	return m, err
}

func ExportAllTenants(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := dbClient.DB().Model(&Tenant{}).Preload("TenantMenus").Preload("Certificate").Preload(clause.Associations)

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

func DeleteTenant(id string) (err error) {
	//判断是否还存在关联的用户和角色未删除
	userCount, err := StatisticUserCount(0, id, "")
	if err != nil {
		return err
	}
	if userCount > 0 {
		return errors.New("请先删除关联的用户")
	}

	roleCount, err := StatisticRoleCount(id)
	if err != nil {
		return err
	}
	if roleCount > 0 {
		return errors.New("请先删除关联的角色")
	}
	return dbClient.DB().Delete(&Tenant{}, "id=?", id).Error
}

func CopyTenant(id string) error {
	from, err := GetTenantByID(id)
	if err != nil {
		return err
	}
	to := &Tenant{}
	err = copier.CopyWithOption(to, from, copier.Option{IgnoreEmpty: true, DeepCopy: true})
	if err != nil {
		return err
	}
	to.Name += " Copy"
	return CreateTenant(to)
}

func EnableTenant(id string, enable bool) error {
	err := dbClient.DB().Model(&Tenant{}).Where("id=?", id).Update("enable", enable).Error
	if err != nil {
		return err
	}
	return nil
}

func StatisticTenantCount() (int64, error) {
	db := dbClient.DB().Model(&Tenant{})
	var count int64
	err := db.Count(&count).Error
	return count, err
}

func getTenantUserCount(db *gorm.DB, tenantID string) (bool, int32, error) {
	m := &Tenant{}
	err := db.Where("id = ?", tenantID).First(m).Error
	if err != nil {
		return true, 0, err
	}
	return !m.Expired.After(time.Now()), m.UserCount, nil
}

func getTenantRoleCount(db *gorm.DB, tenantID string) (bool, int32, error) {
	m := &Tenant{}
	err := db.Where("id = ?", tenantID).First(m).Error
	if err != nil {
		return true, 0, err
	}
	return !m.Expired.After(time.Now()), m.UserCount, nil
}

func getTenantProjectCount(db *gorm.DB, tenantID string) (bool, int32, error) {
	m := &Tenant{}
	err := db.Where("id = ?", tenantID).First(m).Error
	if err != nil {
		return true, 0, err
	}
	return !m.Expired.After(time.Now()), m.UserCount, nil
}

// true-过期
// false-未过期
func tenantExpired(db *gorm.DB, tenantID string) (bool, error) {
	m := &Tenant{}
	err := db.Where("id = ?", tenantID).First(m).Error
	if err != nil {
		return false, err
	}

	return !m.Expired.After(time.Now()), nil
}

func PBToTenantMenus(tenantMenus []*apipb.TenantMenu) []*TenantMenu {
	var list []*TenantMenu
	for _, tenantMenu := range tenantMenus {
		list = append(list, &TenantMenu{
			Model: commonmodel.Model{
				ID: tenantMenu.Id,
			},
			TenantID: tenantMenu.TenantID,
			MenuID:   tenantMenu.MenuID,
			Funcs:    tenantMenu.Funcs,
			Menu:     PBToMenu(tenantMenu.Menu),
		})
	}
	return list
}

func TenantMenusToPB(tenantMenus []*TenantMenu) []*apipb.TenantMenu {
	var list []*apipb.TenantMenu
	for _, tenantMenu := range tenantMenus {
		list = append(list, &apipb.TenantMenu{
			Id:       tenantMenu.ID,
			TenantID: tenantMenu.TenantID,
			MenuID:   tenantMenu.MenuID,
			Funcs:    tenantMenu.Funcs,
			Menu:     MenuToPB(tenantMenu.Menu),
		})
	}
	return list
}

func PBToTenantCertificate(in *apipb.TenantCertificate) *TenantCertificate {
	if in == nil {
		return nil
	}
	return &TenantCertificate{
		Model: commonmodel.Model{
			ID: in.Id,
		},
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
		Model: commonmodel.Model{
			ID: in.Id,
		},
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
		Expired:       utils.ParseTime(in.Expired),
		TenantMenus:   PBToTenantMenus(in.TenantMenus),
		Certificate:   PBToTenantCertificate(in.Certificate),
		IsMust:        in.IsMust,
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
	for _, api := range in {
		list = append(list, TenantToPB(api))
	}
	return list
}
