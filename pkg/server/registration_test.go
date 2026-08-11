package server

import (
	"errors"
	"testing"

	pkgdb "github.com/CloudSilk/pkg/db"
	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func registrationFacadeTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&tenant.Tenant{}, &permission.Role{}, &user.User{}, &user.UserRole{}); err != nil {
		t.Fatalf("migrate registration tables: %v", err)
	}
	SetDB(pkgdb.NewDBClient(database, false))
	if err := database.Create(&permission.Role{Model: commonmodel.Model{ID: "registration-member"}, Name: "member", Public: true, Enable: true}).Error; err != nil {
		t.Fatalf("seed public role: %v", err)
	}
	if err := database.Create(&permission.Role{Model: commonmodel.Model{ID: "private-role"}, TenantID: "another-tenant", Name: "private", Enable: true}).Error; err != nil {
		t.Fatalf("seed private role: %v", err)
	}
	return database
}

func TestRegisterTenantAdminInTransactionIsAtomic(t *testing.T) {
	database := registrationFacadeTestDB(t, "server-registration-atomic")
	request := TenantAdminRegistration{
		TenantID: "tenant-registration", TenantName: "注册企业", Contact: "张三",
		AdminUserName: "Owner", AdminPassword: "Registration123!", AdminNickname: "企业管理员",
		RoleID: "registration-member", ProjectID: "labelnexus",
	}
	result, err := func() (TenantAdminRegistrationResult, error) {
		var value TenantAdminRegistrationResult
		err := database.Transaction(func(tx *gorm.DB) error {
			var registerErr error
			value, registerErr = RegisterTenantAdminInTransaction(tx, request)
			return registerErr
		})
		return value, err
	}()
	if err != nil {
		t.Fatalf("register tenant administrator: %v", err)
	}
	if result.TenantID != request.TenantID || result.UserID == "" || result.RoleID != request.RoleID {
		t.Fatalf("unexpected registration result: %#v", result)
	}
	var administrator user.User
	if err := database.Where("id = ?", result.UserID).First(&administrator).Error; err != nil {
		t.Fatalf("load administrator: %v", err)
	}
	if administrator.UserName != "owner" || administrator.Password == request.AdminPassword || administrator.TenantID != request.TenantID {
		t.Fatalf("administrator was not normalized or hashed: %#v", administrator)
	}
	var assignment user.UserRole
	if err := database.Where("user_id = ? AND role_id = ?", result.UserID, request.RoleID).First(&assignment).Error; err != nil {
		t.Fatalf("load administrator role: %v", err)
	}

	rollbackErr := database.Transaction(func(tx *gorm.DB) error {
		_, registerErr := RegisterTenantAdminInTransaction(tx, TenantAdminRegistration{
			TenantID: "tenant-rollback", TenantName: "回滚企业", AdminUserName: "rollback-owner",
			AdminPassword: "Registration123!", RoleID: "registration-member",
		})
		if registerErr != nil {
			return registerErr
		}
		return errors.New("host onboarding failed")
	})
	if rollbackErr == nil {
		t.Fatal("host failure did not roll back registration")
	}
	var rollbackCount int64
	if err := database.Model(&tenant.Tenant{}).Where("id = ?", "tenant-rollback").Count(&rollbackCount).Error; err != nil || rollbackCount != 0 {
		t.Fatalf("rolled-back tenant remained: count=%d err=%v", rollbackCount, err)
	}
}

func TestRegisterTenantAdminInTransactionRejectsDuplicateAndForeignRole(t *testing.T) {
	database := registrationFacadeTestDB(t, "server-registration-validation")
	base := TenantAdminRegistration{
		TenantID: "tenant-a", TenantName: "企业 A", AdminUserName: "tenant-owner",
		AdminPassword: "Registration123!", RoleID: "registration-member",
	}
	if err := database.Transaction(func(tx *gorm.DB) error {
		_, err := RegisterTenantAdminInTransaction(tx, base)
		return err
	}); err != nil {
		t.Fatalf("seed registration: %v", err)
	}
	duplicate := base
	duplicate.TenantID = "tenant-b"
	duplicate.TenantName = "企业 B"
	if err := database.Transaction(func(tx *gorm.DB) error {
		_, err := RegisterTenantAdminInTransaction(tx, duplicate)
		return err
	}); err == nil {
		t.Fatal("duplicate administrator username was accepted")
	}
	foreignRole := base
	foreignRole.TenantID = "tenant-c"
	foreignRole.TenantName = "企业 C"
	foreignRole.AdminUserName = "tenant-c-owner"
	foreignRole.RoleID = "private-role"
	if err := database.Transaction(func(tx *gorm.DB) error {
		_, err := RegisterTenantAdminInTransaction(tx, foreignRole)
		return err
	}); err == nil {
		t.Fatal("foreign private role was accepted")
	}
}
