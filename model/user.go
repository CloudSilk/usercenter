package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/model/token"
	apipb "github.com/CloudSilk/usercenter/proto"
	scrypt "github.com/elithrar/simple-scrypt"
	"github.com/golang-jwt/jwt/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type User struct {
	model.TenantModel
	ProjectID string `json:"projectID" gorm:"index;size:36"`
	//账号信息
	UserName       string      `json:"userName"  validate:"required" gorm:"size:50;index;comment:用户登录名"`
	Password       string      `json:"password"  gorm:"size:200;comment:用户登录密码"`
	Nickname       string      `json:"nickname"  validate:"required" gorm:"size:100;index;default:未设置;comment:用户昵称" `
	UserRoles      []*UserRole `json:"userRoles"`
	RoleIDs        []string    `json:"roleIDs" gorm:"-"`
	WechatUnionID  string      `json:"wechatUnionID" gorm:"size:36;index;comment:微信UionID"`
	WechatOpenID   string      `json:"wechatOpenID" gorm:"size:36;index;comment:微信OpenID"`
	WechatConfigID string      `json:"wechatConfigID" gorm:"size:36;index;comment:微信配置ID"`
	//用户类型
	Type  int32  `json:"type" gorm:"index"`
	Group string `json:"group" gorm:"index;size:50"`
	//启用账号后可以登录系统
	Enable bool `json:"enable" gorm:"index"`
	// 用户名/密码错误次数
	ErrNumber int32 `json:"errNumber"`
	// 随着密码/用户名错误次数增加，用户被锁定的时间也就越长，一天最多错误5次
	LockedExpired int64 `json:"lockedExpired"`
	// 强制修改密码
	ForceChangePwd bool `json:"forceChangePwd"`
	// 用户过期时间
	Expired                int64 `json:"expired"`
	ChangePwdErrNum        int32 `json:"changePwdErrNum"`
	ChangePwdLockedExpired int64 `json:"changePwdLockedExpired"`

	CanDel bool `json:"canDel"`

	//基本信息
	Email       string  `json:"email" gorm:"size:100;"`
	Mobile      string  `json:"mobile" gorm:"size:20;index;comment:手机号"`
	IDCard      string  `json:"idCard"  gorm:"size:18;index;comment:身份证号"`
	Avatar      string  `json:"avatar" gorm:"size:200;comment:用户头像"`
	EID         string  `json:"eid" gorm:"size:50;"`
	Title       string  `json:"title" gorm:"size:100;comment:职位"`
	Description string  `json:"description" gorm:"size:200;"`
	RealName    string  `json:"realName" gorm:"index;size:50;"`
	Gender      bool    `json:"gender"`         //性别 true=男
	Age         int32   `json:"age" gorm:"" `   //年龄
	Height      float32 `json:"height" gorm:""` //身高
	Weight      float32 `json:"weight" gorm:""` //体重

	ChineseName string `json:"chineseName" gorm:"size:50"`
	EnglishName string `json:"englishName" gorm:"size:50"`
	StaffNo     string `json:"staffNo" gorm:"index;size:50"`

	Country    string    `json:"country" gorm:"size:100;"`  //国家
	Province   string    `json:"province" gorm:"size:100;"` //省份
	City       string    `json:"city" gorm:"size:100;"`     //城市
	County     string    `json:"county" gorm:"size:100;"`   //区县
	Birthday   int64     `json:"birthday"`                  //公历出生日期包含时分
	IsVip      bool      `json:"isVip"`
	VipExpired time.Time `json:"vipExpired"`

	Tenant           *Tenant                `json:"tenant"`
	IsMust           bool                   `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
	WechatOpenIDMaps []*UserWechatOpenIDMap `json:"wechatOpenIDMaps"`
}

type UserWechatOpenIDMap struct {
	model.Model
	UserID         string `json:"userID" gorm:"index"`
	UnionID        string `json:"unionID" gorm:"size:36;index;comment:微信UionID"`
	OpenID         string `json:"openID" gorm:"size:36;index;comment:微信OpenID"`
	WechatConfigID string `json:"wechatConfigID" gorm:"size:36;index;comment:微信配置ID"`
}

func (u User) GetRoleIDs() []string {
	var roleIDs []string
	for _, role := range u.UserRoles {
		roleIDs = append(roleIDs, role.RoleID)
	}
	return roleIDs
}

type UserRole struct {
	model.Model
	UserID string `json:"userID" gorm:"index"`
	RoleID string `json:"roleID" gorm:"index"`
	Role   *Role
}

func UpdateBasics(m *User) error {
	var data = map[string]interface{}{
		"gender":   m.Gender,
		"age":      m.Age,
		"nickname": m.Nickname,
	}
	if m.Height != 0 {
		data["height"] = m.Height
	}
	return dbClient.DB().Model(&User{}).Where("id = ?", m.ID).
		Updates(data).Error
}

func UpdateUserAgeHeightWeight(m *User) error {
	var data = map[string]interface{}{}

	if m.Age != 0 {
		data["age"] = m.Age
	}
	if m.Height != 0 {
		data["height"] = m.Height
	}
	if m.Weight != 0 {
		data["weight"] = m.Weight
	}
	if m.Age == 0 && m.Height == 0 && m.Weight == 0 {
		return nil
	}

	return dbClient.DB().Model(&User{}).Where("id = ?", m.ID).Updates(data).Error
}

// @author: [guoxf](https://github.com/guoxf)
// @function: CreateUser
// @description: 新增User
// @param: user User
// @return: err error
func CreateUser(user *User, isCreateFromWechat bool) error {

	//密码加密
	if user.Password != "" {
		//校验密码强度
		if !ValidPasswdStrength(user.Password) {
			return errors.New("密码强度不够")
		}
		var err error
		user.Password, err = EncryptedPassword(user.Password)
		if err != nil {
			return err
		}
	}
	user.CanDel = true
	user.UserName = strings.ToLower(user.UserName)
	return dbClient.DB().Transaction(func(tx *gorm.DB) error {
		count, err := statisticUserCount(tx, 0, user.TenantID, "")
		if err != nil {
			return err
		}

		expired, tenantUserCount, err := getTenantUserCount(tx, user.TenantID)
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
			duplication, err = dbClient.CreateWithCheckDuplicationWithDB(tx, user, "wechat_union_id = ? and wechat_open_id = ?", user.WechatUnionID, user.WechatOpenID)
		} else {
			duplication, err = dbClient.CreateWithCheckDuplicationWithDB(tx, user, "user_name = ? or mobile = ?", user.UserName, user.Mobile)
		}

		if err != nil {
			return err
		}
		if duplication {
			return errors.New("存在相同用户名或者手机号")
		}
		return nil
	})
}

// @author: [guoxf](https://github.com/guoxf)
// @function: DeleteUser
// @description: 删除User
// @param: user User
// @return: err error
func DeleteUser(id string) (err error) {
	return dbClient.DB().Transaction(func(tx *gorm.DB) error {
		oldUser, err := GetUserById(id)
		if err != nil {
			return err
		}
		if !oldUser.CanDel {
			return errors.New("此用户不允许删除")
		}
		err = dbClient.DB().Unscoped().Delete(&UserRole{}, "user_id=?", id).Error
		if err != nil {
			return err
		}
		return dbClient.DB().Delete(&User{}, "id=?", id).Error
	})
}

type QueryUserRequest struct {
	model.CommonRequest
	UserName  string `json:"userName" form:"userName" uri:"userName"`
	Nickname  string `json:"nickname" form:"nickname" uri:"nickname"`
	IDCard    string `json:"idCard" form:"idCard" uri:"idCard"`
	Mobile    string `json:"mobile" form:"mobile" uri:"mobile"`
	WechatID  string `json:"wechatID" form:"wechatID" uri:"wechatID"`
	Title     string `json:"title" form:"title" uri:"title"`
	UserNames string `json:"userNames" form:"userNames" uri:"userNames"`
}

type QueryUserResponse struct {
	model.CommonResponse
	Data []*User `json:"data"`
}

// @author: [guoxf](https://github.com/guoxf)
// @function: GetUserInfoList
// @description: 分页查询User
// @param: user User, info PageInfo, order string, desc bool
// @return: list []*User, total int64 , err error
func QueryUser(req *apipb.QueryUserRequest, resp *apipb.QueryUserResponse, preload bool) {
	db := dbClient.DB().Model(&User{})

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

	if req.WechatID != "" {
		db = db.Where("`wechat_id` = ?", req.WechatID)
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

	if req.WechatConfigID != "" {
		db = db.Where("`wechat_config_id` = ?", req.WechatConfigID)
	}

	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}

	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`user_name`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}

	var list []*User
	if preload {
		resp.Records, resp.Pages, err = dbClient.PageQueryWithPreload(db, req.PageSize, req.PageIndex, orderStr, []string{"UserRoles", clause.Associations}, &list)
	} else {
		resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
	}
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = UsersToPB(list)
	}
	resp.Total = resp.Records
}

// @author: [guoxf](https://github.com/guoxf)
// @function: GetAllUsers
// @description: 获取所有User
// @return: users []User , err error
func GetAllUsers(req *apipb.GetAllUsersRequest) (users []*User, err error) {
	db := dbClient.DB().Model(&User{})
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

// @author: [guoxf](https://github.com/guoxf)
// @function: GetUserById
// @description: 根据id获取user
// @param: id uint
// @return: user User , err error
func GetUserById(id string) (user User, err error) {
	err = dbClient.DB().Preload("UserRoles.Role").Preload(clause.Associations).Where("id = ?", id).First(&user).Error
	user.Password = ""
	user.WechatUnionID = ""
	user.WechatOpenID = ""
	user.RoleIDs = user.GetRoleIDs()
	return
}

// @author: [guoxf](https://github.com/guoxf)
// @function: UpdateUser
// @description: 根据id更新user
// @param: user User
// @return: err error
func UpdateUser(user *User) error {
	user.UserName = strings.ToLower(user.UserName)
	return dbClient.DB().Transaction(func(tx *gorm.DB) error {
		oldUser := &User{}
		err := tx.Preload("UserRoles").Preload(clause.Associations).Where("id = ?", user.ID).First(oldUser).Error
		if err != nil {
			return err
		}
		//TODO 更新角色后，需要重新登录才能生效
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

		duplication, err := dbClient.UpdateWithCheckDuplicationAndOmit(tx, user, true, []string{"password", "can_del", "created_at", "wechat_union_id", "wechat_open_id", "height", "age"}, "id <> ? and (user_name = ? or mobile = ?)", user.ID, user.UserName, user.Mobile)
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
	return dbClient.DB().Model(&User{}).Where("id=?", id).Update("enable", enable).Error
}

func ResetPwd(id string, pwd string) error {
	password, err := EncryptedPassword(pwd)
	if err != nil {
		return err
	}
	return dbClient.DB().Model(&User{}).Where("id=?", id).Update("password", password).Error
}

func UpdatePwd(id string, oldPwd, newPwd string) error {
	var user User
	err := dbClient.DB().Where("id = ?", id).First(&user).Error
	if err != nil {
		return err
	}

	err = scrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPwd))
	if err != nil {
		return err
	}
	return ResetPwd(id, newPwd)
}

// UpdateProfile 更新个人信息
func UpdateProfile(m *User, updateUserName bool) error {
	fields := []string{"gender", "country",
		"province", "city", "county", "birthday", "nickname", "description",
		"eid", "avatar", "mobile", "email", "real_name", "title", "id_card"}
	if updateUserName {
		fields = append(fields, "user_name")
	}
	return dbClient.DB().Model(m).Select(fields).Where("id=?", m.ID).Updates(m).Error
}

func UpdateProfileAndUserName(m *User) error {
	return dbClient.DB().Model(m).Select("gender", "country",
		"province", "city", "county", "birthday", "nickname", "description",
		"eid", "avatar", "mobile", "email", "real_name", "`group`", "title", "`type`", "id_card").Where("id=?", m.ID).Updates(m).Error
}

func Login(req *apipb.LoginRequest, resp *apipb.LoginResponse) {
	req.UserName = strings.ToLower(req.UserName)
	user := &User{}
	err := dbClient.DB().Model(user).Preload("UserRoles").First(user, "user_name=?", req.UserName).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		return
	}
	if err == gorm.ErrRecordNotFound {
		resp.Code = model.UserIsNotExist
		resp.Message = "用户不存在!"
		return
	}

	if !user.Enable {
		resp.Code = model.UserDisabled
		resp.Message = "用户已禁用!"
		return
	}

	password := []byte(user.Password)
	err = scrypt.CompareHashAndPassword(password, []byte(req.Password))
	if err != nil && err == scrypt.ErrMismatchedHashAndPassword {
		resp.Code = model.UserNameOrPasswordIsWrong
		return
	} else if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	currentUser := &apipb.CurrentUser{
		Id:       user.ID,
		UserName: user.UserName,
		Gender:   user.Gender,
		RoleIDs:  user.GetRoleIDs(),
		TenantID: user.TenantID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
	}
	t, err := token.EncodeToken(currentUser)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		return
	}
	resp.Data = t
}

func LoginByStaffNo(req *apipb.LoginByStaffNoRequest, resp *apipb.LoginByStaffNoResponse) {
	m := map[string]interface{}{}
	if req.StaffNo != "" {
		m["staff_no"] = strings.ToLower(req.StaffNo)
	} else if req.UserName != "" {
		m["user_name"] = strings.ToLower(req.UserName)
	} else {
		resp.Code = model.BadRequest
		resp.Message = "staffNo或userName参数不能为空"
		return
	}
	user := &User{}
	err := dbClient.DB().Model(user).Preload("UserRoles").First(user, m).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		return
	}
	if err == gorm.ErrRecordNotFound {
		resp.Code = model.UserIsNotExist
		resp.Message = "用户不存在!"
		return
	}

	if !user.Enable {
		resp.Code = model.UserDisabled
		resp.Message = "用户已禁用!"
		return
	}

	if m["user_name"] != "" {
		password := []byte(user.Password)
		err = scrypt.CompareHashAndPassword(password, []byte(req.Password))
		if err != nil && err == scrypt.ErrMismatchedHashAndPassword {
			resp.Code = model.UserNameOrPasswordIsWrong
			return
		} else if err != nil {
			resp.Code = model.InternalServerError
			resp.Message = err.Error()
		}
	}

	currentUser := &apipb.CurrentUser{
		Id:       user.ID,
		UserName: user.UserName,
		Gender:   user.Gender,
		RoleIDs:  user.GetRoleIDs(),
		TenantID: user.TenantID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
	}
	t, err := token.EncodeToken(currentUser)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		return
	}
	resp.Data = t
	resp.User = &apipb.UserProfile{
		Id:          user.ID,
		UserName:    user.UserName,
		ChineseName: user.ChineseName,
		EnglishName: user.EnglishName,
		Mobile:      user.Mobile,
		StaffNo:     user.StaffNo,
	}
}

func LogoutByUserName(req *apipb.LogoutByUserNameRequest, resp *apipb.CommonResponse) {
	var userID string
	err := dbClient.DB().Model(User{}).Select("id").Where("user_name=?", strings.ToLower(req.UserName)).Scan(&userID).Error
	if err == gorm.ErrRecordNotFound {
		resp.Code = model.UserIsNotExist
		resp.Message = "用户不存在"
		return
	} else if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		return
	}

	if err := token.DefaultTokenCache.DelByUserID(userID); err != nil {
		log.Error(context.Background(), err)
	}

	resp.Message = userID
}

func LoginByWechat(register bool, req *User, resp *apipb.LoginResponse) {
	if req.WechatOpenID == "" && req.WechatUnionID == "" {
		resp.Code = model.BadRequest
		resp.Message = "用户微信信息不完整!"
		return
	}
	user := &User{}
	// 1、先用union id 或者open id判断是否已经创建过用户
	// 2、如果已经创建过，再用open id判断是否已经关联了open id
	// 3、如果未创建过，那么创建用户
	whereSql := ""
	value := ""
	if req.WechatUnionID != "" {
		whereSql = "wechat_union_id=?"
		value = req.WechatUnionID
	} else {
		whereSql = "wechat_open_id=?"
		value = req.WechatOpenID
	}
	err := dbClient.DB().Model(user).Preload("UserRoles").Where(whereSql, value).First(user).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		return
	}

	if err == gorm.ErrRecordNotFound {
		if register {
			// 随机生成默认密码
			req.Password = generatePasswd(16, PwdStrengthAdvance)
			req.WechatOpenIDMaps = append(req.WechatOpenIDMaps, &UserWechatOpenIDMap{
				UnionID:        req.WechatUnionID,
				OpenID:         req.WechatOpenID,
				WechatConfigID: req.WechatConfigID,
			})
			err = CreateUser(req, true)
			if err != nil && err != gorm.ErrRecordNotFound {
				resp.Code = model.InternalServerError
				resp.Message = err.Error()
				return
			}
			user = req
		} else {
			resp.Code = model.UserIsNotExist
			return
		}
	} else {
		//创建关联
		userID, err := CheckRegisterWithWechat(req.WechatOpenID)
		if err != nil {
			resp.Code = model.InternalServerError
			resp.Message = err.Error()
			return
		}
		if userID == "" {
			err := dbClient.DB().Create(&UserWechatOpenIDMap{
				UserID:         user.ID,
				UnionID:        req.WechatUnionID,
				OpenID:         req.WechatOpenID,
				WechatConfigID: req.WechatConfigID,
			}).Error
			if err != nil {
				resp.Code = model.InternalServerError
				resp.Message = err.Error()
				return
			}
		}
	}

	if !user.Enable {
		resp.Code = model.UserDisabled
		resp.Message = "用户已禁用!"
		return
	}

	currentUser := &apipb.CurrentUser{
		Id:       user.ID,
		UserName: user.UserName,
		Gender:   user.Gender,
		RoleIDs:  user.GetRoleIDs(),
		TenantID: user.TenantID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
	}
	t, err := token.EncodeToken(currentUser)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		return
	}
	resp.Data = t
}

type CheckRegisterWithWechatResp struct {
	model.CommonResponse
	Data bool `json:"data"`
}

func CheckRegisterWithWechat(openID string) (string, error) {
	user := &UserWechatOpenIDMap{}
	err := dbClient.DB().Model(user).First(user, "open_id=?", openID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return "", err
	}
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	return user.UserID, nil
}

func Logout(t string) error {
	currentUser, err := token.DecodeToken(t)
	if err != nil {
		if errors.As(err, &jwt.ErrTokenExpired) {
			log.Error(context.Background(), err)
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

type UserProfile struct {
	apipb.UserProfile
	Menus []*Menu `json:"menus"`
}

func GetUserProfile(id string, needMenu bool) (*apipb.UserProfile, error) {
	var user = &User{}
	err := dbClient.DB().Preload("UserRoles.Role.RoleMenus").Preload(clause.Associations).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	user.Password = ""
	user.WechatUnionID = ""
	user.WechatOpenID = ""

	userProfile := &apipb.UserProfile{
		Id:       user.ID,
		TenantID: user.TenantID,
		UserName: user.UserName,
		Nickname: user.Nickname,
		Email:    user.Email,
		Mobile:   user.Mobile,
		IdCard:   user.IDCard,
		Avatar:   user.Avatar,
		RealName: user.RealName,
		Gender:   user.Gender,
		Type:     user.Type,
		Group:    user.Group,

		Country:     user.Country,
		Province:    user.Province,
		City:        user.City,
		County:      user.County,
		Eid:         user.EID,
		Description: user.Description,
		Birthday:    user.Birthday,

		ChineseName: user.ChineseName,
		EnglishName: user.EnglishName,
		StaffNo:     user.StaffNo,
	}
	if !needMenu {
		return userProfile, nil
	}

	//去重
	haveMenu := make(map[string]*RoleMenu)
	for _, userRole := range user.UserRoles {
		for _, roleMenu := range userRole.Role.RoleMenus {
			oldMenu, ok := haveMenu[roleMenu.MenuID]
			if ok {
				if oldMenu.Funcs == "" {
					oldMenu.Funcs = roleMenu.Funcs
				} else if roleMenu.Funcs != "" {
					oldMenu.Funcs += "," + roleMenu.Funcs
				}
				// 合并是否显示菜单
				if oldMenu.Show || roleMenu.Show {
					oldMenu.Show = true
				}
				continue
			}
			haveMenu[roleMenu.MenuID] = roleMenu
		}
	}

	result, err := GetAuthorizedMenu(dbClient.DB(), haveMenu, true)
	if err != nil {
		return nil, err
	}
	userProfile.Menus = MenusToPB(result)
	return userProfile, nil
}

func sortMenu(menu *Menu) {
	if len(menu.Children) > 0 {
		sort.Slice(menu.Children, func(i, j int) bool {
			return menu.Children[i].Sort < menu.Children[j].Sort
		})
		for _, child := range menu.Children {
			sortMenu(child)
		}
	}
}

func StatisticUserCount(t int, tenantID, group string) (int64, error) {
	return statisticUserCount(dbClient.DB(), t, tenantID, group)
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

func BindPhone(userID string, phoneNuber string) error {
	return dbClient.DB().Model(&User{}).Where("id=?", userID).Update("mobile", phoneNuber).Error
}

func GetOpenIDByUserIDAndConfigID(userID, wechatConfigID string) (string, error) {
	var result = &UserWechatOpenIDMap{}
	err := dbClient.DB().Model(result).Where("user_id=? and wechat_config_id=?", userID, wechatConfigID).First(result).Error
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return result.OpenID, nil
}

// GetAuthorizedMenu 获取有权限的菜单
// hidden为True时，隐藏在菜单中不显示的数据
func GetAuthorizedMenu[T interface {
	GetMenuID() string
	GetFuncs() []string
	GetShow() bool
	*RoleMenu | *TenantMenu
}](tx *gorm.DB, authiruzedMenu map[string]T, hidden bool) ([]*Menu, error) {
	parents := make(map[string]*Menu)

	var allMenus []*Menu
	treeMap := make(map[string]*Menu)
	err := tx.Order("sort").Preload("MenuFuncs").Find(&allMenus).Error
	if err != nil {
		return nil, err
	}
	for _, v := range allMenus {
		treeMap[v.ID] = v
	}

	for _, roleMenu := range authiruzedMenu {
		menu := treeMap[roleMenu.GetMenuID()]
		//顶级menu不在菜单中显示
		if menu == nil || (hidden && !roleMenu.GetShow()) || (hidden && menu.Hidden) {
			continue
		}

		funcs := roleMenu.GetFuncs()
		if len(funcs) == 0 {
			menu.MenuFuncs = []*MenuFunc{}
		} else {
			var menuFuncs []*MenuFunc
			for _, fn := range menu.MenuFuncs {
				for _, f := range funcs {
					if fn.Name == f {
						menuFuncs = append(menuFuncs, fn)
					}
				}
			}
			menu.MenuFuncs = menuFuncs
		}

		if menu.ParentID == "" {
			parents[menu.ID] = menu
		} else {
			parent := treeMap[menu.ParentID]
			parent.Children = append(parent.Children, menu)
		}
	}
	var result []*Menu
	for _, menu := range parents {
		sortMenu(menu)
		result = append(result, menu)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Sort < result[j].Sort
	})
	return result, nil
}

func ExportAllUsers(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := dbClient.DB().Model(&User{}).Preload("UserRoles.Role.RoleMenus").Preload(clause.Associations)

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
