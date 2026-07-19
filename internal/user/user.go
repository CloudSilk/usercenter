package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/alert"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	apipb "github.com/CloudSilk/usercenter/proto"
	scrypt "github.com/elithrar/simple-scrypt"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 全局配置(从 model/init.go 迁入)
var (
	DefaultPwd                 = ""
	loginLockMaxErrCount int32 = 5
	loginLockLockMinutes int   = 15
)

func SetDefaultPwd(pwd string) {
	if pwd != "" {
		DefaultPwd = pwd
	}
}

func SetLoginLock(maxErr int, lockMin int) {
	if maxErr > 0 {
		loginLockMaxErrCount = int32(maxErr)
	}
	if lockMin > 0 {
		loginLockLockMinutes = lockMin
	}
}

type User struct {
	commonmodel.TenantModel
	ProjectID         string                 `json:"projectID" gorm:"index;size:36"`
	UserName          string                 `json:"userName" validate:"required" gorm:"size:50;index;comment:用户登录名"`
	Password          string                 `json:"password" gorm:"size:200;comment:用户登录密码"`
	PasswordUpdatedAt int64                  `json:"passwordUpdatedAt" gorm:"default:0;comment:密码最后修改时间(unix)"`
	Nickname          string                 `json:"nickname" validate:"required" gorm:"size:100;index;default:未设置;comment:用户昵称"`
	UserRoles         []*UserRole            `json:"userRoles"`
	RoleIDs           []string               `json:"roleIDs" gorm:"-"`
	WechatUnionID     string                 `json:"wechatUnionID" gorm:"size:36;index;comment:微信UionID"`
	WechatOpenID      string                 `json:"wechatOpenID" gorm:"size:36;index;comment:微信OpenID"`
	WechatConfigID    string                 `json:"wechatConfigID" gorm:"size:36;index;comment:微信配置ID"`
	Type              int32                  `json:"type" gorm:"index"`
	Group             string                 `json:"group" gorm:"index;size:50"`
	Enable            bool                   `json:"enable" gorm:"index"`
	ErrNumber         int32                  `json:"errNumber"`
	LockedExpired     int64                  `json:"lockedExpired"`
	ForceChangePwd    bool                   `json:"forceChangePwd"`
	Expired           int64                  `json:"expired"`
	CanDel            bool                   `json:"canDel"`
	Email             string                 `json:"email" gorm:"size:100;"`
	Mobile            string                 `json:"mobile" gorm:"size:20;index;comment:手机号"`
	IDCard            string                 `json:"idCard" gorm:"size:18;index;comment:身份证号"`
	Avatar            string                 `json:"avatar" gorm:"size:200;comment:用户头像"`
	EID               string                 `json:"eid" gorm:"size:50;"`
	Title             string                 `json:"title" gorm:"size:100;comment:职位"`
	Description       string                 `json:"description" gorm:"size:200;"`
	RealName          string                 `json:"realName" gorm:"index;size:50;"`
	Gender            bool                   `json:"gender"`
	Age               int32                  `json:"age"`
	Height            float32                `json:"height"`
	Weight            float32                `json:"weight"`
	ChineseName       string                 `json:"chineseName" gorm:"size:50"`
	EnglishName       string                 `json:"englishName" gorm:"size:50"`
	StaffNo           string                 `json:"staffNo" gorm:"index;size:50"`
	Country           string                 `json:"country" gorm:"size:100;"`
	Province          string                 `json:"province" gorm:"size:100;"`
	City              string                 `json:"city" gorm:"size:100;"`
	County            string                 `json:"county" gorm:"size:100;"`
	Birthday          int64                  `json:"birthday"`
	IsVip             bool                   `json:"isVip"`
	VipExpired        *time.Time             `json:"vipExpired"`
	Tenant            *tenant.Tenant         `json:"tenant"`
	IsMust            bool                   `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
	WechatOpenIDMaps  []*UserWechatOpenIDMap `json:"wechatOpenIDMaps"`
}

type UserWechatOpenIDMap struct {
	commonmodel.Model
	UserID         string `json:"userID" gorm:"index"`
	UnionID        string `json:"unionID" gorm:"size:36;index;comment:微信UionID"`
	OpenID         string `json:"openID" gorm:"size:36;index;comment:微信OpenID"`
	WechatConfigID string `json:"wechatConfigID" gorm:"size:36;index;comment:微信配置ID"`
}

type UserRole struct {
	commonmodel.Model
	UserID string `json:"userID" gorm:"index"`
	RoleID string `json:"roleID" gorm:"index"`
	Role   *permission.Role
}

func (u User) GetRoleIDs() []string {
	var roleIDs []string
	for _, role := range u.UserRoles {
		roleIDs = append(roleIDs, role.RoleID)
	}
	return roleIDs
}

func CreateUser(user *User, isCreateFromWechat bool) error {
	if user.Password != "" {
		if !auth.ValidPasswdStrength(user.Password) {
			return errors.New("密码强度不够")
		}
		var err error
		user.Password, err = auth.EncryptedPassword(user.Password)
		if err != nil {
			return err
		}
		user.PasswordUpdatedAt = time.Now().Unix()
	}
	user.CanDel = true
	user.UserName = strings.ToLower(user.UserName)
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		count, err := statisticUserCount(tx, 0, user.TenantID, "")
		if err != nil {
			return err
		}
		expired, tenantUserCount, err := tenant.GetTenantUserCount(user.TenantID)
		if err != nil {
			return err
		}
		if expired {
			return fmt.Errorf("账号使用期限已过，你可以联系管理员!")
		}
		if tenantUserCount > 0 && tenantUserCount <= int32(count) {
			return fmt.Errorf("只能创建 %d 个用户", tenantUserCount)
		}
		var duplication bool
		if isCreateFromWechat {
			duplication, err = store.Client().CreateWithCheckDuplicationWithDB(tx, user, "wechat_union_id = ? and wechat_open_id = ?", user.WechatUnionID, user.WechatOpenID)
		} else {
			if strings.TrimSpace(user.Mobile) == "" {
				duplication, err = store.Client().CreateWithCheckDuplicationWithDB(tx, user, "user_name = ?", user.UserName)
			} else {
				duplication, err = store.Client().CreateWithCheckDuplicationWithDB(tx, user, "user_name = ? or mobile = ?", user.UserName, user.Mobile)
			}
		}
		if err != nil {
			return err
		}
		if duplication {
			return errors.New("存在相同用户名或者手机号")
		}
		return nil
	})
	if err == nil {
		alert.FireEvent("user.created", map[string]interface{}{
			"id": user.ID, "userName": user.UserName, "tenantID": user.TenantID,
		})
	}
	return err
}

func DeleteUser(id string) (err error) {
	var oldUser User
	_ = store.DB().Where("id = ?", id).First(&oldUser).Error
	txErr := store.DB().Transaction(func(tx *gorm.DB) error {
		var exists User
		if e := tx.Where("id = ?", id).First(&exists).Error; e != nil {
			return e
		}
		if !exists.CanDel {
			return errors.New("此用户不允许删除")
		}
		if e := store.DB().Unscoped().Delete(&UserRole{}, "user_id=?", id).Error; e != nil {
			return e
		}
		return store.DB().Delete(&User{}, "id=?", id).Error
	})
	if txErr == nil {
		alert.FireEvent("user.deleted", map[string]interface{}{
			"id": oldUser.ID, "userName": oldUser.UserName, "tenantID": oldUser.TenantID,
		})
	}
	return txErr
}

// QueryOptions contains HTTP-oriented filters that are not part of the legacy
// Triple QueryUserRequest contract.
type QueryOptions struct {
	Keyword string
	Enable  *bool
}

func QueryUser(req *apipb.QueryUserRequest, resp *apipb.QueryUserResponse, preload bool) {
	QueryUserWithOptions(req, resp, preload, QueryOptions{})
}

// QueryUserWithOptions keeps the existing QueryUser contract while allowing
// the native HTTP API to offer a single keyword search and an explicit status
// filter.
func QueryUserWithOptions(req *apipb.QueryUserRequest, resp *apipb.QueryUserResponse, preload bool, options QueryOptions) {
	db := store.DB().Model(&User{})
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where(
			"(user_name LIKE ? OR nickname LIKE ? OR mobile LIKE ? OR email LIKE ? OR real_name LIKE ?)",
			like, like, like, like, like,
		)
	}
	if options.Enable != nil {
		db = db.Where("enable = ?", *options.Enable)
	}
	if req.UserName != "" {
		db = db.Where("`user_name` LIKE ?", "%"+req.UserName+"%")
	} else if len(req.UserNames) > 0 {
		db = db.Where("`user_name` in ?", strings.Split(req.UserNames, ","))
	}
	if req.Nickname != "" {
		db = db.Where("`nickname` LIKE ?", "%"+req.Nickname+"%")
	}
	if req.IdCard != "" {
		db = db.Where("`id_card` = ?", req.IdCard)
	}
	if req.TenantID != "" {
		db = db.Where("`tenant_id` = ?", req.TenantID)
	}
	if req.Mobile != "" {
		db = db.Where("`mobile` LIKE ?", "%"+req.Mobile+"%")
	}
	if req.Title != "" {
		db = db.Where("`title` LIKE ?", "%"+req.Title+"%")
	}
	if req.Type > 0 {
		db = db.Where("`type` = ?", req.Type)
	}
	if req.Group != "" {
		db = db.Where("`group` = ?", req.Group)
	}
	if len(req.Ids) > 0 {
		db = db.Where("id in ?", req.Ids)
	}
	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`user_name`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}
	var list []*User
	if preload {
		resp.Records, resp.Pages, err = store.Client().PageQueryWithPreload(db, req.PageSize, req.PageIndex, orderStr, []string{"UserRoles", clause.Associations}, &list)
	} else {
		resp.Records, resp.Pages, err = store.Client().PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
	}
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = UsersToPB(list)
	}
	resp.Total = resp.Records
}

func GetAllUsers(req *apipb.GetAllUsersRequest) (users []*User, err error) {
	db := store.DB().Model(&User{})
	if req.TenantID != "" {
		db = db.Where("`tenant_id` = ?", req.TenantID)
	}
	if req.Type > 0 {
		db = db.Where("`type` = ?", req.Type)
	}
	if req.Group != "" {
		db = db.Where("`group` = ?", req.Group)
	}
	err = db.Find(&users).Error
	return
}

func GetUserById(id string) (user User, err error) {
	err = store.DB().Preload("UserRoles.Role").Preload(clause.Associations).Where("id = ?", id).First(&user).Error
	user.Password = ""
	user.WechatUnionID = ""
	user.WechatOpenID = ""
	user.RoleIDs = user.GetRoleIDs()
	return
}

func UpdateUser(user *User) error {
	user.UserName = strings.ToLower(user.UserName)
	return store.DB().Transaction(func(tx *gorm.DB) error {
		oldUser := &User{}
		err := tx.Preload("UserRoles").Preload(clause.Associations).Where("id = ?", user.ID).First(oldUser).Error
		if err != nil {
			return err
		}
		if user.UserRoles != nil {
			var deleteUserRole []string
			for _, oldUserRole := range oldUser.UserRoles {
				flag := false
				for _, newUserRole := range user.UserRoles {
					if newUserRole.ID == oldUserRole.ID {
						flag = true
					}
				}
				if !flag {
					deleteUserRole = append(deleteUserRole, oldUserRole.ID)
				}
			}
			if len(deleteUserRole) > 0 {
				err = tx.Unscoped().Delete(&UserRole{}, "id in ?", deleteUserRole).Error
				if err != nil {
					return err
				}
			}
		}
		query := "id <> ? and user_name = ?"
		params := []interface{}{user.ID, user.UserName}
		if strings.TrimSpace(user.Mobile) != "" {
			query = "id <> ? and (user_name = ? or mobile = ?)"
			params = append(params, user.Mobile)
		}
		duplication, err := store.Client().UpdateWithCheckDuplicationAndOmit(tx, user, true, []string{"password", "can_del", "created_at", "wechat_union_id", "wechat_open_id", "height", "age"}, query, params...)
		if err != nil {
			return err
		}
		if duplication {
			return errors.New("存在相同用户名或者手机号")
		}
		return nil
	})
}

func EnableUser(id string, enable bool) error {
	return store.DB().Model(&User{}).Where("id=?", id).Update("enable", enable).Error
}

func ResetPwd(id string, pwd string) error {
	if pwd == "" {
		pwd = auth.GeneratePasswd(16, auth.PwdStrengthAdvance)
	}
	password, err := auth.EncryptedPassword(pwd)
	if err != nil {
		return err
	}
	return store.DB().Model(&User{}).Where("id=?", id).UpdateColumns(map[string]interface{}{
		"password":            password,
		"force_change_pwd":    true,
		"password_updated_at": time.Now().Unix(),
		"err_number":          0,
		"locked_expired":      0,
	}).Error
}

func GetUserTenantID(id string) (string, error) {
	var u User
	err := store.DB().Select("tenant_id").Where("id = ?", id).Limit(1).First(&u).Error
	if err != nil {
		return "", err
	}
	return u.TenantID, nil
}

func recordLoginFailure(userID string, currentErrNumber int32) {
	updates := map[string]interface{}{
		"err_number": gorm.Expr("err_number + ?", 1),
	}
	if currentErrNumber+1 >= loginLockMaxErrCount {
		updates["locked_expired"] = time.Now().Unix() + int64(loginLockLockMinutes*60)
	}
	if err := store.DB().Model(&User{}).Where("id = ?", userID).UpdateColumns(updates).Error; err != nil {
		log.Error(context.Background(), err)
	}
}

func clearLoginFailure(userID string) {
	if err := store.DB().Model(&User{}).Where("id = ?", userID).UpdateColumns(map[string]interface{}{
		"err_number":     0,
		"locked_expired": 0,
	}).Error; err != nil {
		log.Error(context.Background(), err)
	}
}

func UpdatePwd(id string, oldPwd, newPwd string) error {
	if !auth.ValidPasswdStrength(newPwd) {
		return errors.New("密码强度不够")
	}
	var u User
	err := store.DB().Where("id = ?", id).First(&u).Error
	if err != nil {
		return err
	}
	err = scrypt.CompareHashAndPassword([]byte(u.Password), []byte(oldPwd))
	if err != nil {
		return err
	}
	return ResetPwd(id, newPwd)
}

func UpdateProfile(m *User, updateUserName bool) error {
	fields := []string{"gender", "country", "province", "city", "county", "birthday", "nickname", "description", "eid", "avatar", "mobile", "email", "real_name", "title", "id_card"}
	if updateUserName {
		fields = append(fields, "user_name")
	}
	return store.DB().Model(m).Select(fields).Where("id=?", m.ID).Updates(m).Error
}

func Login(req *apipb.LoginRequest, resp *apipb.LoginResponse) {
	req.UserName = strings.ToLower(req.UserName)
	u := &User{}
	err := store.DB().Model(u).Preload("UserRoles").First(u, "user_name=?", req.UserName).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	if err == gorm.ErrRecordNotFound {
		resp.Code = apipb.Code_UserIsNotExist
		resp.Message = "用户不存在!"
		return
	}
	if !u.Enable {
		resp.Code = commonmodel.UserDisabled
		resp.Message = "用户已禁用!"
		return
	}
	if u.LockedExpired > time.Now().Unix() {
		resp.Code = commonmodel.UserDisabled
		resp.Message = "账号已锁定，请稍后再试"
		return
	}
	password := []byte(u.Password)
	err = scrypt.CompareHashAndPassword(password, []byte(req.Password))
	if err != nil && err == scrypt.ErrMismatchedHashAndPassword {
		recordLoginFailure(u.ID, u.ErrNumber)
		resp.Code = commonmodel.UserNameOrPasswordIsWrong
		return
	} else if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	clearLoginFailure(u.ID)
	if auth.IsPwdExpired(u.PasswordUpdatedAt) {
		resp.Code = 41007
		resp.Message = "密码已过期，请修改密码"
		return
	}
	// MFA 二阶段：若用户绑定了启用的因子，签发一次性 challenge，要求二次验证（不签发 access_token）。
	if auth.HasEnabledMFA(u.ID) {
		challenge, err := auth.IssueMFAChallenge(u.ID)
		if err != nil {
			resp.Code = apipb.Code_InternalServerError
			resp.Message = err.Error()
			return
		}
		resp.Code = 41008 // MfaRequired
		resp.Message = "需要 MFA 二次验证"
		resp.Data = challenge
		return
	}
	currentUser := &apipb.CurrentUser{
		Id: u.ID, UserName: u.UserName, Gender: u.Gender,
		RoleIDs: u.GetRoleIDs(), TenantID: u.TenantID, Nickname: u.Nickname, Avatar: u.Avatar,
	}
	t, err := token.EncodeToken(currentUser)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	resp.Data = t
}

// CompleteMFALogin 完成 MFA 第二因素验证并签发 access_token。
// mfaToken 来自 Login 返回的 challenge（code=41008 时 resp.Data），code 为 6 位 TOTP。
func CompleteMFALogin(mfaToken, code string, resp *apipb.LoginResponse) {
	userID, ok := auth.ConsumeMFAChallenge(mfaToken)
	if !ok {
		resp.Code = 41009 // MfaChallengeInvalid
		resp.Message = "MFA 令牌无效或已过期，请重新登录"
		return
	}
	if !auth.VerifyMFACode(userID, code) {
		resp.Code = 41010 // MfaCodeInvalid
		resp.Message = "MFA 验证码不正确"
		return
	}
	u := &User{}
	if err := store.DB().Preload("UserRoles").First(u, "id = ?", userID).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	currentUser := &apipb.CurrentUser{
		Id: u.ID, UserName: u.UserName, Gender: u.Gender,
		RoleIDs: u.GetRoleIDs(), TenantID: u.TenantID, Nickname: u.Nickname, Avatar: u.Avatar,
	}
	t, err := token.EncodeToken(currentUser)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	resp.Data = t
}

func LoginByWechat(register bool, req *User, resp *apipb.LoginResponse) {
	if req.WechatOpenID == "" && req.WechatUnionID == "" {
		resp.Code = commonmodel.BadRequest
		resp.Message = "用户微信信息不完整!"
		return
	}
	u := &User{}
	whereSql := ""
	value := ""
	if req.WechatUnionID != "" {
		whereSql = "wechat_union_id=?"
		value = req.WechatUnionID
	} else {
		whereSql = "wechat_open_id=?"
		value = req.WechatOpenID
	}
	err := store.DB().Model(u).Preload("UserRoles").Where(whereSql, value).First(u).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	if err == gorm.ErrRecordNotFound {
		if register {
			req.Password = auth.GeneratePasswd(16, auth.PwdStrengthAdvance)
			err = CreateUser(req, true)
			if err != nil && err != gorm.ErrRecordNotFound {
				resp.Code = apipb.Code_InternalServerError
				resp.Message = err.Error()
				return
			}
			u = req
		} else {
			resp.Code = apipb.Code_UserIsNotExist
			return
		}
	}
	if !u.Enable {
		resp.Code = commonmodel.UserDisabled
		resp.Message = "用户已禁用!"
		return
	}
	// MFA 二阶段：微信登录同样强制（绑定 MFA 的用户需二次验证）
	if auth.HasEnabledMFA(u.ID) {
		challenge, err := auth.IssueMFAChallenge(u.ID)
		if err != nil {
			resp.Code = apipb.Code_InternalServerError
			resp.Message = err.Error()
			return
		}
		resp.Code = 41008
		resp.Message = "需要 MFA 二次验证"
		resp.Data = challenge
		return
	}
	currentUser := &apipb.CurrentUser{
		Id: u.ID, UserName: u.UserName, Gender: u.Gender,
		RoleIDs: u.GetRoleIDs(), TenantID: u.TenantID, Nickname: u.Nickname, Avatar: u.Avatar,
	}
	t, err := token.EncodeToken(currentUser)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	resp.Data = t
}

func CheckRegisterWithWechat(openID string) (string, error) {
	m := &UserWechatOpenIDMap{}
	err := store.DB().Model(m).First(m, "open_id=?", openID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return "", err
	}
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	return m.UserID, nil
}

func Logout(t string) error {
	currentUser, err := token.DecodeToken(t)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil
		}
	}
	if currentUser == nil {
		return nil
	}
	err = token.DefaultTokenCache.Del(fmt.Sprint(currentUser.Id), t)
	if err != nil {
		log.Error(context.Background(), err)
	}
	return nil
}

func GetUserProfile(id string, needMenu bool) (*apipb.UserProfile, error) {
	var u = &User{}
	err := store.DB().Preload("UserRoles.Role.RoleMenus").Preload(clause.Associations).Where("id = ?", id).First(&u).Error
	if err != nil {
		return nil, err
	}
	u.Password = ""
	u.WechatUnionID = ""
	u.WechatOpenID = ""
	userProfile := &apipb.UserProfile{
		Id: u.ID, TenantID: u.TenantID, UserName: u.UserName, Nickname: u.Nickname,
		Email: u.Email, Mobile: u.Mobile, IdCard: u.IDCard, Avatar: u.Avatar, RealName: u.RealName,
		Gender: u.Gender, Type: u.Type, Group: u.Group,
		Country: u.Country, Province: u.Province, City: u.City, County: u.County,
		Eid: u.EID, Description: u.Description, Birthday: u.Birthday,
		ChineseName: u.ChineseName, EnglishName: u.EnglishName, StaffNo: u.StaffNo,
	}
	if !needMenu {
		return userProfile, nil
	}
	haveMenu := make(map[string]*permission.RoleMenu)
	for _, userRole := range u.UserRoles {
		for _, roleMenu := range userRole.Role.RoleMenus {
			oldMenu, ok := haveMenu[roleMenu.MenuID]
			if ok {
				if oldMenu.Funcs == "" {
					oldMenu.Funcs = roleMenu.Funcs
				} else if roleMenu.Funcs != "" {
					oldMenu.Funcs += "," + roleMenu.Funcs
				}
				if oldMenu.Show || roleMenu.Show {
					oldMenu.Show = true
				}
				continue
			}
			haveMenu[roleMenu.MenuID] = roleMenu
		}
	}
	result, err := permission.GetAuthorizedMenu(store.DB(), haveMenu, true)
	if err != nil {
		return nil, err
	}
	userProfile.Menus = permission.MenusToPB(result)
	return userProfile, nil
}

func StatisticUserCount(t int, tenantID, group string) (int64, error) {
	return statisticUserCount(store.DB(), t, tenantID, group)
}

func statisticUserCount(db *gorm.DB, t int, tenantID, group string) (int64, error) {
	db = db.Model(&User{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if group != "" {
		db = db.Where("`group` = ?", group)
	}
	if t > 0 {
		db = db.Where("`type` = ?", t)
	}
	var count int64
	err := db.Count(&count).Error
	return count, err
}

func BindPhone(userID string, phoneNumber string) error {
	return store.DB().Model(&User{}).Where("id=?", userID).Update("mobile", phoneNumber).Error
}

func GetOpenIDByUserIDAndConfigID(userID, wechatConfigID string) (string, error) {
	var result = &UserWechatOpenIDMap{}
	err := store.DB().Model(result).Where("user_id=? and wechat_config_id=?", userID, wechatConfigID).First(result).Error
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return result.OpenID, nil
}

func ExportAllUsers(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := store.DB().Model(&User{}).Preload("UserRoles.Role.RoleMenus").Preload(clause.Associations)
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	var list []*User
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

func PBToUser(in *apipb.UserInfo) *User {
	if in == nil {
		return nil
	}
	u := &User{}
	u.ID = in.Id
	u.TenantID = in.TenantID
	u.ProjectID = in.ProjectID
	u.UserName = in.UserName
	u.Nickname = in.Nickname
	u.UserRoles = PBToUserRoles(in.UserRoles)
	u.RoleIDs = in.RoleIDs
	u.Enable = in.Enable
	u.Email = in.Email
	u.Mobile = in.Mobile
	u.IDCard = in.IdCard
	u.Avatar = in.Avatar
	u.EID = in.Eid
	u.Title = in.Title
	u.Description = in.Description
	u.RealName = in.RealName
	u.Gender = in.Gender
	u.Password = in.Password
	u.Type = in.Type
	u.Group = in.Group
	u.WechatUnionID = in.WechatUnionID
	u.WechatOpenID = in.WechatOpenID
	u.City = in.City
	u.Country = in.Country
	u.Province = in.Province
	u.WechatConfigID = in.WechatConfigID
	u.IsMust = in.IsMust
	u.WechatOpenIDMaps = PBToUserWechatOpenIDMaps(in.WechatOpenIDMaps)
	u.Age = in.Age
	u.Height = in.Height
	u.Weight = in.Weight
	if len(in.VipExpired) == 10 {
		in.VipExpired = in.VipExpired + " 15:59:59"
	}
	vipExpired := utils.ParseTime(in.VipExpired)
	if !vipExpired.IsZero() {
		u.VipExpired = &vipExpired
		u.IsVip = time.Until(vipExpired).Seconds() > 0
	}
	return u
}

func UserToPB(in *User) *apipb.UserInfo {
	if in == nil {
		return nil
	}
	return &apipb.UserInfo{
		Id: in.ID, TenantID: in.TenantID, ProjectID: in.ProjectID,
		UserName: in.UserName, Nickname: in.Nickname,
		UserRoles: UserRolesToPB(in.UserRoles), RoleIDs: in.RoleIDs,
		Enable: in.Enable, Email: in.Email, Mobile: in.Mobile,
		IdCard: in.IDCard, Avatar: in.Avatar, Eid: in.EID,
		Title: in.Title, Description: in.Description, RealName: in.RealName,
		Gender: in.Gender, Type: in.Type, Group: in.Group,
		City: in.City, Country: in.Country, Province: in.Province,
		CreatedAt:      utils.FormatTime(in.CreatedAt),
		WechatConfigID: in.WechatConfigID, IsMust: in.IsMust,
		WechatOpenIDMaps: UserWechatOpenIDMapsToPB(in.WechatOpenIDMaps),
		Age:              in.Age, Height: in.Height, Weight: in.Weight,
		IsVip: in.IsVip,
	}
}

func UsersToPB(in []*User) []*apipb.UserInfo {
	var list []*apipb.UserInfo
	for _, u := range in {
		list = append(list, UserToPB(u))
	}
	return list
}

func PBToUserRoles(userRoles []*apipb.UserRole) []*UserRole {
	var list []*UserRole
	for _, ur := range userRoles {
		list = append(list, &UserRole{
			Model: commonmodel.Model{ID: ur.Id}, UserID: ur.UserID, RoleID: ur.RoleID,
		})
	}
	return list
}

func UserRolesToPB(userRoles []*UserRole) []*apipb.UserRole {
	var list []*apipb.UserRole
	for _, ur := range userRoles {
		list = append(list, &apipb.UserRole{Id: ur.ID, UserID: ur.UserID, RoleID: ur.RoleID})
	}
	return list
}

func UserWechatOpenIDMapsToPB(maps []*UserWechatOpenIDMap) []*apipb.UserWechatOpenIDMap {
	var list []*apipb.UserWechatOpenIDMap
	for _, m := range maps {
		list = append(list, &apipb.UserWechatOpenIDMap{
			Id: m.ID, UserID: m.UserID, UnionID: m.UnionID, OpenID: m.OpenID, WechatConfigID: m.WechatConfigID,
		})
	}
	return list
}

func PBToUserWechatOpenIDMaps(maps []*apipb.UserWechatOpenIDMap) []*UserWechatOpenIDMap {
	var list []*UserWechatOpenIDMap
	for _, m := range maps {
		list = append(list, &UserWechatOpenIDMap{
			Model: commonmodel.Model{ID: m.Id}, UserID: m.UserID, UnionID: m.UnionID, OpenID: m.OpenID, WechatConfigID: m.WechatConfigID,
		})
	}
	return list
}

func UserProfileToUser(in *apipb.UserProfile) *User {
	if in == nil {
		return nil
	}
	return &User{
		UserName: in.UserName, Nickname: in.Nickname, Email: in.Email,
		Mobile: in.Mobile, IDCard: in.IdCard, Avatar: in.Avatar,
		RealName: in.RealName, Gender: in.Gender, EID: in.Eid,
		ChineseName: in.ChineseName, EnglishName: in.EnglishName, StaffNo: in.StaffNo,
	}
}

type CheckRegisterWithWechatResp struct {
	commonmodel.CommonResponse
	Data bool `json:"data"`
}

func LoginByStaffNo(req *apipb.LoginByStaffNoRequest, resp *apipb.LoginByStaffNoResponse) {
	m := map[string]interface{}{}
	if req.StaffNo != "" {
		m["staff_no"] = strings.ToLower(req.StaffNo)
	} else if req.UserName != "" {
		m["user_name"] = strings.ToLower(req.UserName)
	} else {
		resp.Code = commonmodel.BadRequest
		resp.Message = "staffNo或userName参数不能为空"
		return
	}
	u := &User{}
	err := store.DB().Model(u).Preload("UserRoles").First(u, m).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	if err == gorm.ErrRecordNotFound {
		resp.Code = apipb.Code_UserIsNotExist
		resp.Message = "用户不存在!"
		return
	}
	if !u.Enable {
		resp.Code = commonmodel.UserDisabled
		resp.Message = "用户已禁用!"
		return
	}
	if u.LockedExpired > time.Now().Unix() {
		resp.Code = commonmodel.UserDisabled
		resp.Message = "账号已锁定，请稍后再试"
		return
	}
	err = scrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password))
	if err != nil && err == scrypt.ErrMismatchedHashAndPassword {
		recordLoginFailure(u.ID, u.ErrNumber)
		resp.Code = commonmodel.UserNameOrPasswordIsWrong
		return
	} else if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	clearLoginFailure(u.ID)
	if auth.IsPwdExpired(u.PasswordUpdatedAt) {
		resp.Code = 41007
		resp.Message = "密码已过期，请修改密码"
		return
	}
	if auth.HasEnabledMFA(u.ID) {
		resp.Code = 41008
		resp.Message = "需要 MFA 二次验证"
		return
	}
	currentUser := &apipb.CurrentUser{
		Id: u.ID, UserName: u.UserName, Gender: u.Gender,
		RoleIDs: u.GetRoleIDs(), TenantID: u.TenantID, Nickname: u.Nickname, Avatar: u.Avatar,
	}
	t, err := token.EncodeToken(currentUser)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	resp.Data = t
	resp.User = &apipb.UserProfile{
		Id: u.ID, UserName: u.UserName, ChineseName: u.ChineseName,
		EnglishName: u.EnglishName, Mobile: u.Mobile, StaffNo: u.StaffNo,
	}
}

func LogoutByUserName(req *apipb.LogoutByUserNameRequest, resp *apipb.CommonResponse) {
	var userID string
	err := store.DB().Model(User{}).Select("id").Where("user_name=?", strings.ToLower(req.UserName)).Scan(&userID).Error
	if err == gorm.ErrRecordNotFound {
		resp.Code = apipb.Code_UserIsNotExist
		resp.Message = "用户不存在"
		return
	} else if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	if err := token.DefaultTokenCache.DelByUserID(userID); err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	resp.Message = userID
}

func UpdateBasics(m *User) error {
	data := map[string]interface{}{"gender": m.Gender, "age": m.Age, "nickname": m.Nickname}
	if m.Height != 0 {
		data["height"] = m.Height
	}
	return store.DB().Model(&User{}).Where("id = ?", m.ID).Updates(data).Error
}

func UpdateUserAgeHeightWeight(m *User) error {
	data := map[string]interface{}{}
	if m.Age != 0 {
		data["age"] = m.Age
	}
	if m.Height != 0 {
		data["height"] = m.Height
	}
	if m.Weight != 0 {
		data["weight"] = m.Weight
	}
	if len(data) == 0 {
		return nil
	}
	return store.DB().Model(&User{}).Where("id = ?", m.ID).Updates(data).Error
}

func UpdateProfileAndUserName(m *User) error {
	return store.DB().Model(m).Select("gender", "country", "province", "city", "county",
		"birthday", "nickname", "description", "eid", "avatar", "mobile", "email", "real_name",
		"`group`", "title", "`type`", "id_card").Where("id=?", m.ID).Updates(m).Error
}

func LoginLockMaxErrCount() int32 { return loginLockMaxErrCount }
func LoginLockLockMinutes() int   { return loginLockLockMinutes }
