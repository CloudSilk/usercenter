package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"dubbo.apache.org/dubbo-go/v3/config"
	"github.com/CloudSilk/pkg/model"
	ucmodel "github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

func GetUser(c *gin.Context) (bool, *apipb.CurrentUser) {
	obj, exists := c.Get("User")
	if !exists {
		return false, nil
	}
	user, ok := obj.(*apipb.CurrentUser)
	if !ok {
		return false, nil
	}
	return true, user
}

func GetUserID(c *gin.Context) string {
	exists, user := GetUser(c)
	if !exists || user == nil {
		return ""
	}
	return user.Id
}

func GetUserName(c *gin.Context) string {
	exists, user := GetUser(c)
	if !exists || user == nil {
		return ""
	}
	return user.UserName
}

func GetTenantID(c *gin.Context) string {
	exists, user := GetUser(c)
	if !exists || user == nil {
		return ""
	}
	return user.TenantID
}

func GetAccessToken(c *gin.Context) string {
	token := c.GetHeader("Authorization")
	if token == "" {
		token = c.GetHeader("authorization")
	}
	token = strings.Replace(token, "Bearer ", "", 1)
	return token
}

func AuthRequired(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/swagger/") || strings.HasPrefix(c.Request.URL.Path, "/web/") {
		return
	}
	t := GetAccessToken(c)
	currentUser, code, err := ucmodel.Authenticate(t, c.Request.Method, c.Request.URL.Path, true)

	if code != model.Success {
		message := ""
		if err != nil {
			message = err.Error()
		}
		c.AbortWithStatusJSON(http.StatusOK, model.CommonResponse{
			Code:    code,
			Message: message,
		})
		return
	}

	c.Set("User", currentUser)
}

var IdentityImpl = new(apipb.IdentityClientImpl)

func InitIdentity() {
	config.SetConsumerService(IdentityImpl)
}

func Authenticate(t, method, url string, checkAuth bool) (*apipb.CurrentUser, int, error) {
	resp, err := IdentityImpl.Authenticate(context.Background(), &apipb.AuthenticateRequest{
		Token:     t,
		Method:    method,
		Url:       url,
		CheckAuth: checkAuth,
	})
	if err != nil {
		return nil, model.InternalServerError, err
	}
	if resp.Code != model.Success {
		return nil, int(resp.Code), errors.New(resp.Message)
	}
	return resp.CurrentUser, model.Success, nil
}

func AuthRequiredWithRPC(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
		return
	}
	t := GetAccessToken(c)
	currentUser, code, err := Authenticate(t, c.Request.Method, c.Request.URL.Path, true)

	if code != model.Success {
		message := ""
		if err != nil {
			message = err.Error()
		}
		c.AbortWithStatusJSON(http.StatusOK, model.CommonResponse{
			Code:    code,
			Message: message,
		})
		return
	}

	c.Set("User", currentUser)
}
