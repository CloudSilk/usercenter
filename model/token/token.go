package token

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"encoding/base64"
	"encoding/json"

	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/golang-jwt/jwt/v4"
)

const defaultExpired = 30 * 24 * time.Hour

var deviceTypes = map[string]int32{
	"":  0,
	"0": 0, //web
	"1": 1, //android phone
	"2": 2, //android pad
	"3": 3, //iphone
	"4": 4, //ipad
}

var secretKey = "c4c902bb-b4ca-4246-a9c0-fb8b218c9a69"

// SetSecretKey 设置Token加密key
func SetSecretKey(key string) {
	secretKey = key
}

// EncodeToken 生产Token
func EncodeToken(user *apipb.CurrentUser) (string, error) {
	expired := DefaultTokenCache.TokenExpired()

	token := jwt.New(jwt.SigningMethodHS256)
	claims := make(jwt.MapClaims)
	claims["exp"] = time.Now().Add(time.Minute * time.Duration(expired)).Unix()
	claims["iat"] = time.Now().Unix()
	claims["id"] = user.Id
	claims["userName"] = user.UserName
	claims["domain"] = user.Domain
	claims["deviceType"] = user.DeviceType
	claims["clientIP"] = user.ClientIP
	claims["sessionID"] = user.SessionID
	claims["tenantID"] = user.TenantID
	claims["key"] = user.Key
	roleIDs, _ := json.Marshal(user.RoleIDs)
	claims["roleIDs"] = string(roleIDs)
	claims["type"] = user.Type
	claims["group"] = user.Group
	claims["nickname"] = user.Nickname
	claims["avatar"] = user.Avatar
	claims["isVip"] = user.IsVip
	claims["vipExpired"] = user.VipExpired

	token.Claims = claims
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}
	err = DefaultTokenCache.StoreToken(fmt.Sprint(user.Id), tokenString)
	if err != nil {
		return "", err
	}
	return tokenString, err
}

func arrayToString(array []int32) string {
	str := make([]string, len(array))
	for i, a := range array {
		str[i] = fmt.Sprint(a)
	}
	return strings.Join(str, ",")
}

func stringToIntArray(str string) ([]int, error) {
	array := strings.Split(str, ",")
	var tenantIDs []int
	for _, str := range array {
		tenantID, err := strconv.Atoi(str)
		if err != nil {
			return nil, err
		}
		tenantIDs = append(tenantIDs, tenantID)
	}
	return tenantIDs, nil
}

func getTenantID(str string) (int, error) {
	tenantIDs, err := stringToIntArray(str)
	if err != nil {
		return 0, err
	}
	if len(tenantIDs) == 0 {
		return 0, nil
	}
	return tenantIDs[0], nil
}

// DecodeToken  解析token
func DecodeToken(t string) (*apipb.CurrentUser, error) {
	if t == "" {
		return nil, errors.New("未登录，请先登录!!")
	}
	token, err := jwt.Parse(t,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secretKey), nil
		})
	if token != nil && token.Valid {
		return ExtractorCurrentUser(token), nil
	}

	//刷新token用
	if token != nil && errors.As(err, &jwt.ErrTokenExpired) {
		return ExtractorCurrentUser(token), err
	}

	return nil, err

}

func ExtractorCurrentUser(t *jwt.Token) *apipb.CurrentUser {
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil
	}

	currentUser := apipb.CurrentUser{}
	id, _ := (claims["id"]).(string)
	currentUser.Id = id
	currentUser.UserName = (claims["userName"]).(string)

	if _, ok := claims["domain"]; ok {
		currentUser.Domain = claims["domain"].(string)
	}
	if _, ok := claims["nickname"]; ok {
		currentUser.Nickname = claims["nickname"].(string)
	}
	if _, ok := claims["avatar"]; ok {
		currentUser.Avatar = claims["avatar"].(string)
	}

	if _, ok := claims["deviceType"]; ok {
		currentUser.DeviceType = int32(claims["deviceType"].(float64))
	}

	if _, ok := claims["type"]; ok {
		currentUser.Type = int32(claims["type"].(float64))
	}

	if _, ok := claims["group"]; ok {
		currentUser.Group = claims["group"].(string)
	}

	if _, ok := claims["tenantID"]; ok {
		currentUser.TenantID = claims["tenantID"].(string)
	}

	if _, ok := claims["clientIP"]; ok {
		currentUser.ClientIP = claims["clientIP"].(string)
	}

	if _, ok := claims["sessionID"]; ok {
		currentUser.SessionID = claims["sessionID"].(string)
	}

	if _, ok := claims["key"]; ok {
		currentUser.Key = claims["key"].(string)
	}
	if _, ok := claims["isVip"]; ok {
		currentUser.IsVip = claims["isVip"].(bool)
	}
	if _, ok := claims["vipExpired"]; ok {
		currentUser.VipExpired = int64(claims["vipExpired"].(float64))
	}

	if _, ok := claims["roleIDs"]; ok {
		str, ok := claims["roleIDs"].(string)
		if ok {
			err := json.Unmarshal([]byte(str), &currentUser.RoleIDs)
			if err != nil {
				fmt.Println(err)
			}
		}
	}

	return &currentUser
}

func GetUserID(t string) (string, error) {
	array := strings.Split(t, ".")
	if len(array) != 3 {
		return "", nil
	}
	var currentUser apipb.CurrentUser
	b, err := base64.RawStdEncoding.DecodeString(array[1])
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(b, &currentUser)
	if err != nil {
		return "", err
	}

	return fmt.Sprint(currentUser.Id), err
}

func GetSessionID(t string) (string, error) {
	array := strings.Split(t, ".")
	if len(array) != 3 {
		return "", nil
	}
	return getSessionID(array[1])
}

func getSessionID(sig string) (string, error) {
	var currentUser apipb.CurrentUser
	b, err := base64.RawStdEncoding.DecodeString(sig)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(b, &currentUser)
	if err != nil {
		return "", err
	}

	return currentUser.SessionID, err
}
