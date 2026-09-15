package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	cmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/pkg/utils/middleware"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	userm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/CloudSilk/usercenter/wechat"
	"github.com/gin-gonic/gin"
)

type MiniLoginRequest struct {
	JsCode          string `json:"jsCode"`
	PhoneNumberCode string `json:"phoneNumberCode"`
	EncryptedData   string `json:"encryptedData"`
	IV              string `json:"iv"`
	Register        bool   `json:"register"`
	App             string `json:"app"`
	Nickname        string `json:"nickname"`
	DeviceType      *int32 `json:"deviceType"`
	DeviceName      string `json:"deviceName"`
}

// wechatMiniLogin 微信小程序登录
//
//	@Summary 微信小程序登录/注册
//	@Tags 微信登录
//	@Param body body MiniLoginRequest true "登录请求(jsCode 必填,register=true 时可携带手机号与资料)"
//	@Success 200 {object} apipb.LoginResponse
//	@Router /api/wechat/mini/login [post]
func wechatMiniLogin(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &MiniLoginRequest{}
	resp := &apipb.LoginResponse{Code: apipb.Code_Success}
	if err := c.BindJSON(req); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,请求参数无效:%v", transID, err)
		return
	}
	miniProgramConfig := wechat.GetMiniProgram(req.App)
	if miniProgramConfig == nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	result, err := wechat.Code2SessionDirect(miniProgramConfig.MiniAppConfig.AppID, miniProgramConfig.MiniAppConfig.Secret, req.JsCode)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,Code2Session出错:%v", transID, err)
		return
	}

	ui := &apipb.UserInfo{
		WechatUnionID: result.UnionID,
		WechatOpenID:  result.OpenID,
		TenantID:      miniProgramConfig.MiniAppConfig.TenantID,
	}
	if req.Register {
		if req.EncryptedData != "" && req.IV != "" {
			plainData, err := miniProgramConfig.MiniProgram.GetEncryptor().Decrypt(result.SessionKey, req.EncryptedData, req.IV)
			if err != nil {
				log.Warnf(context.Background(), "TransID:%s,解密出错:%v", transID, err)
			} else {
				ui.Avatar = plainData.AvatarURL
				ui.Nickname = plainData.NickName
				ui.City = plainData.City
				ui.Country = plainData.Country
				ui.Province = plainData.Province
				ui.Gender = plainData.Gender == 1
				ui.Mobile = plainData.PhoneNumber
			}
		}
		if req.Nickname != "" {
			ui.Nickname = req.Nickname
		}
		ui.Enable = true
		ui.WechatConfigID = miniProgramConfig.MiniAppConfig.ID
		ui.UserRoles = []*apipb.UserRole{{RoleID: miniProgramConfig.MiniAppConfig.DefaultRoleID}}
		if req.PhoneNumberCode != "" {
			result2, err := miniProgramConfig.MiniProgram.GetAuth().GetPhoneNumber(req.PhoneNumberCode)
			if err != nil {
				log.Warnf(context.Background(), "TransID:%s,GetPhoneNumber:%v", transID, err)
			} else {
				ui.Mobile = result2.PhoneInfo.PhoneNumber
			}
		}
		if ui.Mobile != "" {
			ui.UserName = ui.Mobile
		} else if result.UnionID != "" {
			ui.UserName = result.UnionID
		} else {
			ui.UserName = result.OpenID
		}
	}

	loginSession, deviceName := loginSessionContext(c, req.DeviceType, req.DeviceName)
	if strings.TrimSpace(req.DeviceName) == "" {
		deviceName = "微信小程序"
	}
	user.LoginByWechatWithSession(req.Register, user.PBToUser(ui), resp, loginSession)
	var persisted *persistedLoginSession
	if resp.Code == apipb.Code_Success {
		persisted, err = persistLoginSession(c, resp.Data, deviceName)
		if err != nil {
			resp.Code = apipb.Code_InternalServerError
			resp.Message = err.Error()
			resp.Data = ""
		}
	}
	recordLoginAttempt(c, ui.UserName, "", "wechat", resp.Code == 41008, loginSession, deviceName, resp, persisted)
	c.JSON(http.StatusOK, resp)
}

// wechatMiniCheckRegister 检查微信用户是否注册过
//
//	@Summary 检查微信用户是否已注册
//	@Tags 微信登录
//	@Param body body MiniLoginRequest true "检查请求(jsCode/app)"
//	@Success 200 {object} user.CheckRegisterWithWechatResp
//	@Router /api/wechat/mini/register/check [post]
func wechatMiniCheckRegister(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &MiniLoginRequest{}
	resp := &user.CheckRegisterWithWechatResp{}
	resp.Code = cmodel.Success
	if err := c.BindJSON(req); err != nil {
		resp.Code = cmodel.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,请求参数无效:%v", transID, err)
		return
	}
	miniProgram := wechat.GetMiniProgram(req.App)
	if miniProgram == nil {
		resp.Code = cmodel.BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	result, err := wechat.Code2SessionDirect(miniProgram.MiniAppConfig.AppID, miniProgram.MiniAppConfig.Secret, req.JsCode)
	if err != nil {
		resp.Code = cmodel.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	userID, err := user.CheckRegisterWithWechat(result.OpenID)
	if err != nil {
		resp.Code = cmodel.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = userID != ""
	}
	c.JSON(http.StatusOK, resp)
}

// bindPhone 绑定手机号
//
//	@Summary 微信小程序绑定手机号
//	@Tags 微信登录
//	@Param authorization header string true "jwt token"
//	@Param body body MiniLoginRequest true "绑定请求(app/phoneNumberCode)"
//	@Success 200 {object} apipb.LoginResponse
//	@Router /api/wechat/mini/phone/bind [post]
func bindPhone(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &MiniLoginRequest{}
	resp := &apipb.LoginResponse{Code: apipb.Code_Success}
	if err := c.BindJSON(req); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,请求参数无效:%v", transID, err)
		return
	}
	phoneNumber := ""
	if req.PhoneNumberCode != "" {
		miniProgram := wechat.GetMiniProgram(req.App)
		if miniProgram == nil {
			resp.Code = apipb.Code_BadRequest
			resp.Message = "非法应用"
			c.JSON(http.StatusOK, resp)
			return
		}
		result2, err := miniProgram.MiniProgram.GetAuth().GetPhoneNumber(req.PhoneNumberCode)
		if err != nil {
			resp.Code = apipb.Code_InternalServerError
			resp.Message = err.Error()
			log.Warnf(context.Background(), "TransID:%s,GetPhoneNumber:%v", transID, err)
		} else {
			phoneNumber = result2.PhoneInfo.PhoneNumber
		}
	}
	if err := user.BindPhone(userm.GetUserID(c), phoneNumber); err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		log.Warnf(context.Background(), "TransID:%s,BindPhone Error:%v", transID, err)
	}
	c.JSON(http.StatusOK, resp)
}

// wechatWebLogin 微信网页登录
//
//	@Summary 微信网页扫码登录回调
//	@Tags 微信登录
//	@Param code query string true "微信授权 code"
//	@Param state query string true "状态参数(app_siteID_deviceType 格式)"
//	@Success 200 {object} apipb.LoginResponse
//	@Router /api/wechat/web/login [get]
func wechatWebLogin(c *gin.Context) {
	resp := &apipb.LoginResponse{Code: apipb.Code_Success}
	code := c.Query("code")
	if code == "" {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法请求"
		c.JSON(http.StatusOK, resp)
		return
	}
	state := c.Query("state")
	if state == "" {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	array := strings.Split(state, "_")
	if len(array) != 3 {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	app := array[0]
	wechatOpenPlatformWeb := wechat.GetWechatOpenPlatformWeb(app)
	if wechatOpenPlatformWeb == nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	if _, err := wechatOpenPlatformWeb.DecryptState(array[1], array[2]); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法请求"
		c.JSON(http.StatusOK, resp)
		return
	}
	accessToken, err := wechatOpenPlatformWeb.GetAccessToken(code)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	userInfo := &apipb.UserInfo{
		WechatUnionID: accessToken.UnionID,
		WechatOpenID:  accessToken.OpenID,
		TenantID:      wechatOpenPlatformWeb.WechatConfig.TenantID,
	}
	wechatUserInfo, err := wechatOpenPlatformWeb.GetUserInfo(accessToken.UnionID)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	userInfo.Nickname = wechatUserInfo.Nickname
	userInfo.Avatar = wechatUserInfo.HeadImgUrl
	userInfo.Gender = wechatUserInfo.Sex == 1
	userInfo.Country = wechatUserInfo.Country
	userInfo.Province = wechatUserInfo.Province
	userInfo.UserName = wechatUserInfo.UnionID
	userInfo.WechatConfigID = wechatOpenPlatformWeb.WechatConfig.ID
	userInfo.Enable = true

	loginSession, deviceName := loginSessionContext(c, nil, "微信网页登录")
	user.LoginByWechatWithSession(true, user.PBToUser(userInfo), resp, loginSession)
	var persisted *persistedLoginSession
	if resp.Code == apipb.Code_Success {
		persisted, err = persistLoginSession(c, resp.Data, deviceName)
		if err != nil {
			resp.Code = apipb.Code_InternalServerError
			resp.Message = err.Error()
			resp.Data = ""
		}
	}
	recordLoginAttempt(c, userInfo.UserName, "", "wechat", resp.Code == 41008, loginSession, deviceName, resp, persisted)
	if wechatOpenPlatformWeb.GetQRConnectResult(state) != nil {
		if resp.Code == apipb.Code_Success {
			wechatOpenPlatformWeb.UpdateQRConnectResult(state, true, true, resp.Data)
		} else {
			wechatOpenPlatformWeb.UpdateQRConnectResult(state, true, false, "")
		}
		c.String(http.StatusOK, "登录成功！")
	} else {
		c.JSON(http.StatusOK, resp)
	}
}

func RegisterWechatRouter(r *gin.Engine) {
	g := r.Group("/api/wechat")
	g.GET("notify/:app", WechatNotify)
	g.POST("notify/:app", WechatNotify)
	g.POST("mini/login", wechatMiniLogin)
	g.POST("mini/register/check", wechatMiniCheckRegister)
	g.POST("mini/phone/bind", bindPhone)
	g.GET("connect/qrconnect", getQRConnect)
	g.GET("web/login", wechatWebLogin)
	g.POST("web/login", wechatWebLogin)
	g.GET("qrcode", getQRCode)
	g.GET("qrcode/result", checkQRScannResult)
	RegisterWechatPayRouter(r)
	wechat.InitWechat()
}

// getQRCode 获取微信公众号带参二维码
//
//	@Summary 获取微信公众号带参二维码
//	@Tags 微信登录
//	@Param app query string true "应用名称"
//	@Param isTemp query string false "是否临时二维码(true/false)"
//	@Param expireSeconds query string false "临时二维码有效期(秒,默认300)"
//	@Success 200 {object} GetQRCodeResponse
//	@Router /api/wechat/qrcode [get]
func getQRCode(c *gin.Context) {
	resp := &GetQRCodeResponse{Code: apipb.Code_Success}
	app := c.Query("app")
	if app == "" {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	wechatOpenPlatformWeb := wechat.GetWechatOpenPlatformWeb(app)
	if wechatOpenPlatformWeb == nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	isTemp := c.Query("isTemp")
	expireSecondsStr := c.Query("expireSeconds")
	expireSeconds := 300
	if expireSecondsStr != "" {
		expireSeconds, _ = strconv.Atoi(expireSecondsStr)
	}
	result, err := wechatOpenPlatformWeb.GetWechatOfficialAccoutQRCode(isTemp == "true", expireSeconds)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data.Ticket = result.Ticket
		resp.Data.QRCode = result.URL
	}
	c.JSON(http.StatusOK, resp)
}

// getQRConnect 获取微信网页扫码授权地址
//
//	@Summary 获取微信网页扫码授权地址
//	@Tags 微信登录
//	@Param app query string true "应用名称"
//	@Param poll query string false "是否轮询模式(true/false)"
//	@Success 200 {object} GetQRConnectResponse
//	@Router /api/wechat/connect/qrconnect [get]
func getQRConnect(c *gin.Context) {
	resp := &GetQRConnectResponse{Code: apipb.Code_Success}
	app := c.Query("app")
	if app == "" {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	wechatOpenPlatformWeb := wechat.GetWechatOpenPlatformWeb(app)
	if wechatOpenPlatformWeb == nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error
	if c.Query("poll") == "true" {
		resp.Data, err = wechatOpenPlatformWeb.GetPollingAuthURL()
	} else {
		resp.Data, err = wechatOpenPlatformWeb.GetAuthURL()
	}
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// checkQRScannResult 轮询二维码扫码结果
//
//	@Summary 轮询二维码扫码结果
//	@Tags 微信登录
//	@Param app query string true "应用名称"
//	@Param ticket query string true "二维码 ticket"
//	@Success 200 {object} CheckQRScannResultResponse
//	@Router /api/wechat/qrcode/result [get]
func checkQRScannResult(c *gin.Context) {
	app := c.Query("app")
	ticket := c.Query("ticket")
	resp := &CheckQRScannResultResponse{Code: apipb.Code_Success}
	if app == "" || ticket == "" {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法请求"
		c.JSON(http.StatusOK, resp)
		return
	}
	wechatOpenPlatformWeb := wechat.GetWechatOpenPlatformWeb(app)
	if wechatOpenPlatformWeb == nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	result := wechatOpenPlatformWeb.GetQRConnectResult(ticket)
	if result == nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = "非法请求"
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data.Finished = result.Finished
	resp.Data.Token = result.Token
	if result.Finished {
		wechatOpenPlatformWeb.DeleteQRConnectResult(ticket)
	}
	c.JSON(http.StatusOK, resp)
}

type GetQRConnectResponse struct {
	Code    apipb.Code `json:"code"`
	Message string     `json:"message"`
	Data    string     `json:"data"`
}

type GetQRCodeResponse struct {
	Code    apipb.Code `json:"code"`
	Message string     `json:"message"`
	Data    struct {
		QRCode string `json:"qrcode"`
		Ticket string `json:"ticket"`
	} `json:"data"`
}

type CheckQRScannResultResponse struct {
	Code    apipb.Code `json:"code"`
	Message string     `json:"message"`
	Data    struct {
		Finished bool   `json:"finished"`
		Token    string `json:"token"`
	} `json:"data"`
}
