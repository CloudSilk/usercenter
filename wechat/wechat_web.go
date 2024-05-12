package wechat

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	ucmodel "github.com/CloudSilk/usercenter/model"
	"github.com/patrickmn/go-cache"
)

const (
	platformQrConnect                  = "https://open.weixin.qq.com/connect/qrconnect?appid=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s#wechat_redirect"
	platformAuthorize                  = "https://open.weixin.qq.com/connect/oauth2/authorize?appid=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s#wechat_redirect"
	platformGetAccessToken             = "https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code"
	platformRefreshAccessToken         = "https://api.weixin.qq.com/sns/oauth2/refresh_token?appid=%s&grant_type=refresh_token&refresh_token=%s"
	platformGetUserInfo                = "https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s&lang=zh_CN"
	platformCheckAccessToken           = "https://api.weixin.qq.com/sns/auth?access_token=%s&openid=%s"
	getWechatOfficialAccoutAccessToken = "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s"
	createWechatOfficialAccountQRCode  = "https://api.weixin.qq.com/cgi-bin/qrcode/create?access_token=%s"
	getWechatOfficialAccoutUserInfo    = "https://api.weixin.qq.com/cgi-bin/user/info?access_token=%s&openid=%s&lang=zh_CN"
	sendTplMsgURL                      = "https://api.weixin.qq.com/cgi-bin/message/template/send?access_token=%s"
	scopeLogin                         = "snsapi_login"
	scopeUserInfo                      = "snsapi_userinfo"
)

var (
	wechatOpenPlatformWebs = make(map[string]*WechatOpenPlatformWeb)
)

type WechatOpenPlatformWeb struct {
	// AppID is the app id of the wechat open platform web
	WechatConfig              *ucmodel.WechatConfig
	AccessToken               map[string]GetAccessTokenResponse
	OfficialAccoutAccessToken *GetWechatOfficialAccoutAccessTokenResponse
	lock                      sync.Mutex
	// 存储Token，并且设置过期时间
	QRConnectResult *cache.Cache
}

func NewWechatOpenPlatformWeb(wechatConfig *ucmodel.WechatConfig) *WechatOpenPlatformWeb {
	return &WechatOpenPlatformWeb{
		WechatConfig:    wechatConfig,
		AccessToken:     make(map[string]GetAccessTokenResponse),
		lock:            sync.Mutex{},
		QRConnectResult: cache.New(5*time.Minute, 10*time.Minute),
	}
}

// GetAuthURL get auth url
func (w *WechatOpenPlatformWeb) GetAuthURL() (string, error) {
	state, err := w.EncryptState()
	if err != nil {
		return "", err
	}
	if w.WechatConfig.AppType == 2 {
		w.QRConnectResult.Set(state, &QRConnectResult{
			Finished: false,
		}, cache.DefaultExpiration)
		return fmt.Sprintf(platformAuthorize, w.WechatConfig.AppID, url.QueryEscape(w.WechatConfig.RedirectUrl), scopeUserInfo, state), nil
	}
	return fmt.Sprintf(platformQrConnect, w.WechatConfig.AppID, url.QueryEscape(w.WechatConfig.RedirectUrl), scopeLogin, state), nil
}

// 将AppName+当前时间戳使用Secret进行加密，作为state参数
func (w *WechatOpenPlatformWeb) EncryptState() (string, error) {
	ciphertext, nonce, err := encrypt([]byte(w.WechatConfig.Secret), []byte(w.WechatConfig.AppName))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s_%s", w.WechatConfig.AppName, base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(nonce)), nil
}

// DecryptState 解密state参数
func (w *WechatOpenPlatformWeb) DecryptState(data, nonce string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}
	nonceData, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return "", err
	}
	decrypted, err := decrypt([]byte(w.WechatConfig.Secret), nonceData, ciphertext)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

func encrypt(key, plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func decrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// GetAccessToken get access token by code
func (w *WechatOpenPlatformWeb) GetAccessToken(code string) (*GetAccessTokenResponse, error) {
	resp, err := http.Get(fmt.Sprintf(platformGetAccessToken, w.WechatConfig.AppID, w.WechatConfig.Secret, code))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var getAccessTokenResponse GetAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&getAccessTokenResponse); err != nil {
		return nil, err
	}

	if getAccessTokenResponse.ErrCode != 0 {
		return nil, fmt.Errorf("get access token error: %s", getAccessTokenResponse.ErrMsg)
	}
	w.AccessToken[getAccessTokenResponse.UnionID] = getAccessTokenResponse

	return &getAccessTokenResponse, nil
}

// RefreshAccessToken refresh access token
func (w *WechatOpenPlatformWeb) RefreshAccessToken(unionID string) (string, error) {
	resp, err := http.Get(fmt.Sprintf(platformRefreshAccessToken, w.WechatConfig.AppID, w.AccessToken[unionID].RefreshToken))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var getAccessTokenResponse GetAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&getAccessTokenResponse); err != nil {
		return "", err
	}

	if getAccessTokenResponse.ErrCode != 0 {
		return "", fmt.Errorf("get access token error: %s", getAccessTokenResponse.ErrMsg)
	}
	w.AccessToken[unionID] = getAccessTokenResponse

	return getAccessTokenResponse.AccessToken, nil
}

// 校验Token是否有效
func (w *WechatOpenPlatformWeb) CheckAccessToken(unionID string) bool {
	resp, err := http.Get(fmt.Sprintf(platformCheckAccessToken, w.AccessToken[unionID].AccessToken, w.AccessToken[unionID].OpenID))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var getAccessTokenResponse GetAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&getAccessTokenResponse); err != nil {
		return false
	}

	if getAccessTokenResponse.ErrCode != 0 {
		return false
	}

	return true
}

// 获取个人信息
func (w *WechatOpenPlatformWeb) GetUserInfo(unionID string) (*GetUserInfoResponse, error) {
	resp, err := http.Get(fmt.Sprintf(platformGetUserInfo, w.AccessToken[unionID].AccessToken, w.AccessToken[unionID].OpenID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var getUserInfoResponse GetUserInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&getUserInfoResponse); err != nil {
		return nil, err
	}

	if getUserInfoResponse.ErrCode != 0 {
		return nil, fmt.Errorf("get user info error: %s", getUserInfoResponse.ErrMsg)
	}

	return &getUserInfoResponse, nil
}

func (w *WechatOpenPlatformWeb) GetWechatOfficialAccoutAccessToken() (*GetWechatOfficialAccoutAccessTokenResponse, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	// 先判断是否有accessToken和是否有效
	if w.OfficialAccoutAccessToken != nil && w.CheckWechatOfficialAccoutAccessToken(w.OfficialAccoutAccessToken.AccessToken) {
		return w.OfficialAccoutAccessToken, nil
	}

	resp, err := http.Get(fmt.Sprintf(getWechatOfficialAccoutAccessToken, w.WechatConfig.AppID, w.WechatConfig.Secret))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var getWechatOfficialAccoutAccessTokenResponse GetWechatOfficialAccoutAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&getWechatOfficialAccoutAccessTokenResponse); err != nil {
		return nil, err
	}

	if getWechatOfficialAccoutAccessTokenResponse.ErrCode != 0 {
		return nil, fmt.Errorf("get wechat official accout access token error: %s", getWechatOfficialAccoutAccessTokenResponse.ErrMsg)
	}
	w.OfficialAccoutAccessToken = &getWechatOfficialAccoutAccessTokenResponse
	return &getWechatOfficialAccoutAccessTokenResponse, nil
}

// 校验WechatOfficialAccoutAccessToken是否有效
func (w *WechatOpenPlatformWeb) CheckWechatOfficialAccoutAccessToken(accessToken string) bool {
	return time.Now().Unix() < w.OfficialAccoutAccessToken.ExpiresIn
}

func (w *WechatOpenPlatformWeb) GetWechatOfficialAccoutUserInfo(openID string) (*GetWechatOfficialAccoutUserInfoResponse, error) {
	_, err := w.GetWechatOfficialAccoutAccessToken()
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(fmt.Sprintf(getWechatOfficialAccoutUserInfo, w.OfficialAccoutAccessToken.AccessToken, openID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var getWechatOfficialAccoutUserInfoResponse GetWechatOfficialAccoutUserInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&getWechatOfficialAccoutUserInfoResponse); err != nil {
		return nil, err
	}

	if getWechatOfficialAccoutUserInfoResponse.ErrCode != 0 {
		return nil, fmt.Errorf("get wechat official accout user info error: %s", getWechatOfficialAccoutUserInfoResponse.ErrMsg)
	}

	return &getWechatOfficialAccoutUserInfoResponse, nil
}

func (w *WechatOpenPlatformWeb) GetWechatOfficialAccoutQRCode(isTemp bool, expireSeconds int) (*GetWechatOfficialAccoutQRCodeResponse, error) {
	_, err := w.GetWechatOfficialAccoutAccessToken()
	if err != nil {
		return nil, err
	}

	params := make(map[string]interface{})
	if isTemp {
		params["action_name"] = "QR_SCENE"
		params["expire_seconds"] = expireSeconds
	} else {
		params["action_name"] = "QR_LIMIT_SCENE"
	}
	params["action_info"] = map[string]interface{}{
		"scene": map[string]interface{}{
			"scene_str": w.WechatConfig.AppName,
		},
	}
	url := fmt.Sprintf(createWechatOfficialAccountQRCode, w.OfficialAccoutAccessToken.AccessToken)
	jsonParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonParams))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	result := &GetWechatOfficialAccoutQRCodeResponse{}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return nil, err
	}
	w.QRConnectResult.Set(result.Ticket, &QRConnectResult{
		Response: result,
	}, time.Duration(result.ExpireSeconds)*time.Second)
	return result, nil
}

func (w *WechatOpenPlatformWeb) GetQRConnectResult(ticket string) *QRConnectResult {
	result, ok := w.QRConnectResult.Get(ticket)
	if !ok {
		return nil
	}
	return result.(*QRConnectResult)
}

func (w *WechatOpenPlatformWeb) UpdateQRConnectResult(ticket string, finished bool, success bool, token string) {
	result, ok := w.QRConnectResult.Get(ticket)
	if !ok {
		return
	}
	qrConnectResult, ok := result.(*QRConnectResult)
	if !ok {
		return
	}
	qrConnectResult.Finished = finished
	qrConnectResult.Success = success
	qrConnectResult.Token = token
}

func (w *WechatOpenPlatformWeb) DeleteQRConnectResult(ticket string) {
	w.QRConnectResult.Delete(ticket)
}

func (w *WechatOpenPlatformWeb) SendTplMsg(req SendTplMsgRequest) (*SendTplMsgResponse, error) {
	_, err := w.GetWechatOfficialAccoutAccessToken()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(sendTplMsgURL, w.OfficialAccoutAccessToken.AccessToken)
	jsonParams, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(jsonParams))
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonParams))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result := &SendTplMsgResponse{}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return nil, err
	}
	return result, nil
}

type SendTplMsgRequest struct {
	ToUser      string `json:"touser"`
	TemplateID  string `json:"template_id"`
	Url         string `json:"url"`
	Miniprogram struct {
		AppID    string `json:"appid"`
		PagePath string `json:"pagepath"`
	} `json:"miniprogram"`
	ClientMsgID string                       `json:"client_msg_id"`
	Data        map[string]map[string]string `json:"data"`
}

type SendTplMsgResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	MsgID   int    `json:"msgid"`
}

type QRConnectResult struct {
	Response *GetWechatOfficialAccoutQRCodeResponse
	Finished bool
	Success  bool
	Token    string
}

type GetWechatOfficialAccoutAccessTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type GetWechatOfficialAccoutQRCodeResponse struct {
	ErrCode       int    `json:"errcode"`
	ErrMsg        string `json:"errmsg"`
	Ticket        string `json:"ticket"`
	ExpireSeconds int64  `json:"expire_seconds"`
	URL           string `json:"url"`
	CreatedAt     int64  `json:"created_at"`
}

type GetUserInfoResponse struct {
	ErrCode    int      `json:"errcode"`
	ErrMsg     string   `json:"errmsg"`
	OpenID     string   `json:"openid"`
	Nickname   string   `json:"nickname"`
	Sex        int      `json:"sex"`
	Province   string   `json:"province"`
	City       string   `json:"city"`
	Country    string   `json:"country"`
	HeadImgUrl string   `json:"headimgurl"`
	Privilege  []string `json:"privilege"`
	UnionID    string   `json:"unionid"`
}

type GetAccessTokenResponse struct {
	ErrCode      int    `json:"errcode"`
	ErrMsg       string `json:"errmsg"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid"`
}

type GetWechatOfficialAccoutUserInfoResponse struct {
	ErrCode        int    `json:"errcode"`
	ErrMsg         string `json:"errmsg"`
	Subscribe      int    `json:"subscribe"`
	OpenID         string `json:"openid"`
	Language       string `json:"language"`
	SubscribeTime  int64  `json:"subscribe_time"`
	UnionID        string `json:"unionid"`
	Remark         string `json:"remark"`
	GroupID        int    `json:"groupid"`
	TagIDList      []int  `json:"tagid_list"`
	SubscribeScene string `json:"subscribe_scene"`
	QrScene        int    `json:"qr_scene"`
	QrSceneStr     string `json:"qr_scene_str"`
}

func GetWechatOpenPlatformWeb(app string) *WechatOpenPlatformWeb {
	config := wechatOpenPlatformWebs[app]
	return config
}
