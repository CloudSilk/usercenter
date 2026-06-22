package model

import (
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
)

type User = user.User
type UserRole = user.UserRole
type UserWechatOpenIDMap = user.UserWechatOpenIDMap
type CheckRegisterWithWechatResp = user.CheckRegisterWithWechatResp

var DefaultPwd = ""
func SetDefaultPwd(pwd string) {
	if pwd != "" {
		DefaultPwd = pwd
		user.SetDefaultPwd(pwd)
	}
}
func SetLoginLock(maxErr, lockMin int)  { user.SetLoginLock(maxErr, lockMin) }
func CreateUser(u *User, w bool) error  { return user.CreateUser(u, w) }
func DeleteUser(id string) error        { return user.DeleteUser(id) }
func QueryUser(req *apipb.QueryUserRequest, resp *apipb.QueryUserResponse, preload bool) { user.QueryUser(req, resp, preload) }
func GetAllUsers(req *apipb.GetAllUsersRequest) ([]*User, error)                       { return user.GetAllUsers(req) }
func GetUserById(id string) (User, error)                                              { return user.GetUserById(id) }
func UpdateUser(u *User) error                                                         { return user.UpdateUser(u) }
func EnableUser(id string, enable bool) error                                          { return user.EnableUser(id, enable) }
func ResetPwd(id, pwd string) error                                                    { return user.ResetPwd(id, pwd) }
func GetUserTenantID(id string) (string, error)                                        { return user.GetUserTenantID(id) }
func UpdatePwd(id, old, new string) error                                              { return user.UpdatePwd(id, old, new) }
func UpdateProfile(m *User, upd bool) error                                            { return user.UpdateProfile(m, upd) }
func Login(req *apipb.LoginRequest, resp *apipb.LoginResponse)                         { user.Login(req, resp) }
func LoginByStaffNo(req *apipb.LoginByStaffNoRequest, resp *apipb.LoginByStaffNoResponse) { user.LoginByStaffNo(req, resp) }
func LoginByWechat(reg bool, req *User, resp *apipb.LoginResponse)                    { user.LoginByWechat(reg, req, resp) }
func LogoutByUserName(req *apipb.LogoutByUserNameRequest, resp *apipb.CommonResponse) { user.LogoutByUserName(req, resp) }
func CheckRegisterWithWechat(openID string) (string, error)                           { return user.CheckRegisterWithWechat(openID) }
func Logout(t string) error                                                           { return user.Logout(t) }
func GetUserProfile(id string, needMenu bool) (*apipb.UserProfile, error)             { return user.GetUserProfile(id, needMenu) }
func StatisticUserCount(t int, tid, g string) (int64, error)                         { return user.StatisticUserCount(t, tid, g) }
func BindPhone(uid, phone string) error                                               { return user.BindPhone(uid, phone) }
func GetOpenIDByUserIDAndConfigID(uid, cfgID string) (string, error)                  { return user.GetOpenIDByUserIDAndConfigID(uid, cfgID) }
func ExportAllUsers(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) { user.ExportAllUsers(req, resp) }
func UpdateBasics(m *User) error                                                      { return user.UpdateBasics(m) }
func UpdateUserAgeHeightWeight(m *User) error                                         { return user.UpdateUserAgeHeightWeight(m) }
func UpdateProfileAndUserName(m *User) error                                          { return user.UpdateProfileAndUserName(m) }
func GetAuthorizedMenu[T permission.MenuAuthItem](tx *gorm.DB, m map[string]T, hidden bool) ([]*permission.Menu, error) {
	return permission.GetAuthorizedMenu(tx, m, hidden)
}
func PBToUser(in *apipb.UserInfo) *User               { return user.PBToUser(in) }
func UserToPB(in *User) *apipb.UserInfo               { return user.UserToPB(in) }
func UsersToPB(in []*User) []*apipb.UserInfo          { return user.UsersToPB(in) }
func PBToUserRoles(in []*apipb.UserRole) []*UserRole  { return user.PBToUserRoles(in) }
func UserRolesToPB(in []*UserRole) []*apipb.UserRole  { return user.UserRolesToPB(in) }
func UserWechatOpenIDMapsToPB(in []*UserWechatOpenIDMap) []*apipb.UserWechatOpenIDMap { return user.UserWechatOpenIDMapsToPB(in) }
func PBToUserWechatOpenIDMaps(in []*apipb.UserWechatOpenIDMap) []*UserWechatOpenIDMap { return user.PBToUserWechatOpenIDMaps(in) }
func UserProfileToUser(in *apipb.UserProfile) *User   { return user.UserProfileToUser(in) }
