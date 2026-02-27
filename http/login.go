package http

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/pkg/utils/middleware"
	ucmodel "github.com/CloudSilk/usercenter/model"
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
	// 注册时需要传一个昵称(微信修改了获取用户信息接口，目前无法获取到昵称)
	Nickname string `json:"nickname"`
}

// wechatMiniLogin
// @Summary 微信登录
// @Description 微信登录
// @Tags 微信相关接口
// @Accept  json
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param product body MiniLoginRequest true "登录参数"
// @Success 200 {object} apipb.LoginResponse
// @Router /api/wechat/mini/login [post]
func wechatMiniLogin(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &MiniLoginRequest{}
	resp := &apipb.LoginResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,请求参数无效:%v", transID, err)
		return
	}
	// fmt.Printf("req:%#v\n", req)
	miniProgramConfig := wechat.GetMiniProgram(req.App)
	if miniProgramConfig == nil {
		resp.Code = model.BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,非法的应用:%v", transID, req.App)
		return
	}
	miniprogram := miniProgramConfig.MiniProgram
	result, err := miniprogram.GetAuth().Code2Session(req.JsCode)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,Code2Session出错:%v", transID, err)
		return
	}

	// fmt.Printf("Code2Session:%#v\n", result)
	user := &apipb.UserInfo{
		WechatUnionID: result.UnionID,
		WechatOpenID:  result.OpenID,
		TenantID:      miniProgramConfig.MiniAppConfig.TenantID,
	}
	if req.Register {
		// 微信已废弃 getUserProfile 接口，encryptedData 和 iv 可能为空
		// 如果有加密数据则解密获取用户信息，否则使用基本信息注册
		if req.EncryptedData != "" && req.IV != "" {
			plainData, err := miniprogram.GetEncryptor().Decrypt(result.SessionKey, req.EncryptedData, req.IV)
			if err != nil {
				log.Warnf(context.Background(), "TransID:%s,解密出错:%v", transID, err)
				// 解密失败不阻断注册流程，继续使用基本信息
			} else {
				fmt.Printf("Decrypt:%#v\n", plainData)
				user.Avatar = plainData.AvatarURL
				user.Nickname = plainData.NickName
				user.City = plainData.City
				user.Country = plainData.Country
				user.Province = plainData.Province
				user.Gender = plainData.Gender == 1
				user.Mobile = plainData.PhoneNumber
			}
		}

		// 优先使用请求中传入的昵称
		if req.Nickname != "" {
			user.Nickname = req.Nickname
		}
		user.Enable = true
		user.WechatConfigID = miniProgramConfig.MiniAppConfig.ID
		user.UserRoles = []*apipb.UserRole{
			{RoleID: miniProgramConfig.MiniAppConfig.DefaultRoleID},
		}
		//获取手机号
		if req.PhoneNumberCode != "" {
			result2, err := miniprogram.GetAuth().GetPhoneNumber(req.PhoneNumberCode)
			if err != nil {
				log.Warnf(context.Background(), "TransID:%s,GetPhoneNumber:%v", transID, err)
			} else {
				user.Mobile = result2.PhoneInfo.PhoneNumber
			}
		}
		// 设置用户名：优先手机号 > UnionID > OpenID
		if user.Mobile != "" {
			user.UserName = user.Mobile
		} else if result.UnionID != "" {
			user.UserName = result.UnionID
		} else {
			user.UserName = result.OpenID
		}
	}

	ucmodel.LoginByWechat(req.Register, ucmodel.PBToUser(user), resp)
	c.JSON(http.StatusOK, resp)
}

// wechatMiniCheckRegister
// @Summary 检查是否注册过
// @Description 检查是否注册过
// @Tags 微信相关接口
// @Accept  json
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param product body MiniLoginRequest true "请求参数"
// @Success 200 {object} ucmodel.CheckRegisterWithWechatResp
// @Router /api/wechat/mini/register/check [post]
func wechatMiniCheckRegister(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &MiniLoginRequest{}
	resp := &ucmodel.CheckRegisterWithWechatResp{
		CommonResponse: model.CommonResponse{
			Code: model.Success,
		},
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,请求参数无效:%v", transID, err)
		return
	}
	// fmt.Printf("req:%#v\n", req)
	miniProgram := wechat.GetMiniProgram(req.App)
	if miniProgram == nil {
		resp.Code = model.BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,非法的应用:%v", transID, req.App)
		return
	}
	result, err := miniProgram.MiniProgram.GetAuth().Code2Session(req.JsCode)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,Code2Session出错:%v", transID, err)
		return
	}
	userID, err := ucmodel.CheckRegisterWithWechat(result.OpenID)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = userID != ""
	}
	c.JSON(http.StatusOK, resp)
}

// bindPhone
// @Summary 绑定手机号
// @Description 绑定手机号
// @Tags 微信相关接口
// @Accept  json
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param product body MiniLoginRequest true "请求参数"
// @Success 200 {object} apipb.LoginResponse
// @Router /api/wechat/mini/phone/bind [post]
func bindPhone(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &MiniLoginRequest{}
	resp := &apipb.LoginResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,请求参数无效:%v", transID, err)
		return
	}
	// fmt.Printf("req:%#v\n", req)
	//获取手机号
	phoneNumber := ""
	if req.PhoneNumberCode != "" {
		miniProgram := wechat.GetMiniProgram(req.App)
		if miniProgram == nil {
			resp.Code = model.BadRequest
			resp.Message = "非法应用"
			c.JSON(http.StatusOK, resp)
			log.Warnf(context.Background(), "TransID:%s,非法的应用:%v", transID, req.App)
			return
		}
		result2, err := miniProgram.MiniProgram.GetAuth().GetPhoneNumber(req.PhoneNumberCode)
		if err != nil {
			log.Warnf(context.Background(), "TransID:%s,GetPhoneNumber:%v", transID, err)
			resp.Code = model.InternalServerError
			resp.Message = err.Error()
		} else {
			phoneNumber = result2.PhoneInfo.PhoneNumber
		}
	}
	err = ucmodel.BindPhone(userm.GetUserID(c), phoneNumber)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		log.Warnf(context.Background(), "TransID:%s,BindPhone Error:%v", transID, err)
	}
	c.JSON(http.StatusOK, resp)
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
	wechat.InitWechat()
}

// getQRCode
// @Summary 生成带参数的二维码
// @Description 生成带参数的二维码
// @Tags 微信相关接口
// @Accept  json
// @Produce  json
// @Param app query string true "应用名称"
// @Param isTemp query bool false "是否临时二维码"
// @Param expireSeconds query int false "临时二维码过期时间"
// @Success 200 {object} GetQRCodeResponse
// @Router /api/wechat/qrcode [get]
func getQRCode(c *gin.Context) {
	resp := &GetQRCodeResponse{
		Code: apipb.Code_Success,
	}
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

// getQRConnect
// @Summary 获取微信登录二维码
// @Description 获取微信登录二维码
// @Tags 微信相关接口
// @Accept  json
// @Produce  json
// @Param app query string true "应用名称"
// @Success 200 {object} GetQRConnectResponse
// @Router /api/wechat/connect/qrconnect [get]
func getQRConnect(c *gin.Context) {
	resp := &GetQRConnectResponse{
		Code: apipb.Code_Success,
	}
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
	resp.Data, err = wechatOpenPlatformWeb.GetAuthURL()
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	// fmt.Println(resp.Data)
	c.JSON(http.StatusOK, resp)
}

// wechatWebLogin
// @Summary 微信网页登录
// @Description 微信网页登录
// @Tags 微信相关接口
// @Accept  json
// @Produce  json
// @Param code query string true "code"
// @Param state query string true "应用名称"
// @Success 200 {object} apipb.LoginResponse
// @Router /api/wechat/web/login [post]
func wechatWebLogin(c *gin.Context) {
	resp := &apipb.LoginResponse{
		Code: model.Success,
	}
	code := c.Query("code")
	if code == "" {
		resp.Code = model.BadRequest
		resp.Message = "非法请求"
		c.JSON(http.StatusOK, resp)
		return
	}

	state := c.Query("state")
	if state == "" {
		resp.Code = model.BadRequest
		resp.Message = "非法应用"
		c.JSON(http.StatusOK, resp)
		return
	}
	array := strings.Split(state, "_")
	if len(array) != 3 {
		resp.Code = model.BadRequest
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

	_, err := wechatOpenPlatformWeb.DecryptState(array[1], array[2])
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = "非法请求"
		c.JSON(http.StatusOK, resp)
		return
	}

	accessToken, err := wechatOpenPlatformWeb.GetAccessToken(code)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	// fmt.Printf("accessToken:%#v\n", accessToken)

	user := &ucmodel.User{
		WechatUnionID: accessToken.UnionID,
		WechatOpenID:  accessToken.OpenID,
		TenantModel: model.TenantModel{
			TenantID: wechatOpenPlatformWeb.WechatConfig.TenantID,
		},
	}

	wechatUserInfo, err := wechatOpenPlatformWeb.GetUserInfo(accessToken.UnionID)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}

	// fmt.Printf("wechatUserInfo:%#v\n", wechatUserInfo)

	user.Nickname = wechatUserInfo.Nickname
	user.Avatar = wechatUserInfo.HeadImgUrl
	user.Gender = wechatUserInfo.Sex == 1
	user.Country = wechatUserInfo.Country
	user.Province = wechatUserInfo.Province
	user.UserName = wechatUserInfo.UnionID
	user.WechatConfigID = wechatOpenPlatformWeb.WechatConfig.ID
	user.Enable = true

	ucmodel.LoginByWechat(true, user, resp)
	if wechatOpenPlatformWeb.WechatConfig.AppType == 2 {
		if resp.Code == model.Success {
			wechatOpenPlatformWeb.UpdateQRConnectResult(state, true, true, resp.Data)
		} else {
			wechatOpenPlatformWeb.UpdateQRConnectResult(state, true, false, "")
		}
		c.String(http.StatusOK, "登录成功！")
	} else {
		c.JSON(http.StatusOK, resp)
	}
}

// checkQRConnectResult
// @Summary 获取扫码结果
// @Description 支持获取生成带参数的二维码微信网页登录扫码结果
// @Tags 微信相关接口
// @Accept  json
// @Produce  json
// @Param app query string true "应用名称"
// @Param ticket query string true "生成带参数的二维码的ticket或者微信网页登录state"
// @Success 200 {object} CheckQRScannResultResponse
// @Router /api/wechat/qrcode/result [get]
func checkQRScannResult(c *gin.Context) {
	app := c.Query("app")
	ticket := c.Query("ticket")
	resp := &CheckQRScannResultResponse{
		Code: model.Success,
	}
	if app == "" {
		resp.Code = model.BadRequest
		resp.Message = "非法请求"
		c.JSON(http.StatusOK, resp)
		return
	}
	if ticket == "" {
		resp.Code = model.BadRequest
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
		resp.Code = model.InternalServerError
		resp.Message = "非法请求"
		c.JSON(http.StatusOK, resp)
		return
	}

	resp.Code = model.Success
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
		//是否扫码完成
		Finished bool `json:"finished"`
		//登录成功后的token
		Token string `json:"token"`
	} `json:"data"`
}
