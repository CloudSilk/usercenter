package http

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	cmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/wechat"
	"github.com/gin-gonic/gin"
)

// /api/wechat/notify?signature=779ee239e53c506537b56e530cd96bd5869c890a&echostr=7007211687744363958×tamp=1632722226&nonce=1266362590
// WechatNotify 微信消息通知
func WechatNotify(c *gin.Context) {
	app := c.Param("app")
	wechatOpenPlatformWeb := wechat.GetWechatOpenPlatformWeb(app)
	if wechatOpenPlatformWeb == nil {
		c.Data(http.StatusOK, "text/html", []byte("非法应用"))
		return
	}
	// 从URL中获取微信服务器发送的参数
	signature := c.Query("signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")

	// 将token、timestamp和nonce排序
	params := []string{wechatOpenPlatformWeb.WechatConfig.Token, timestamp, nonce}
	sort.Strings(params)

	// 拼接字符串，并计算SHA1签名
	rawString := strings.Join(params, "")
	hash := sha1.Sum([]byte(rawString))
	hashString := hex.EncodeToString(hash[:])

	fmt.Println(hashString, signature)
	if signature != hashString {
		c.Data(http.StatusOK, "text/html", []byte("非法请求"))
		return
	}
	//区别是验证接口还是有真正的消息
	//如果是POST请求，那么就是有真正的消息
	if c.Request.Method == "POST" {
		//解析消息
		var msg WechatNotifyRequest
		if err := c.ShouldBindXML(&msg); err != nil {
			fmt.Println(err)
			c.Data(http.StatusOK, "text/html", []byte("非法请求"))
			return
		}
		//处理消息
		fmt.Printf("消息：%#v\n", msg)
		// 处理关注事件
		if msg.MsgType == "event" && (msg.Event == "subscribe" || msg.Event == "SCAN") {
			// 注册应用账号
			resp, err := wechatOpenPlatformWeb.GetWechatOfficialAccoutUserInfo(msg.FromUserName)
			if err != nil {
				fmt.Println(err)
				c.Data(http.StatusOK, "text/html", []byte("非法请求"))
				return
			}
			fmt.Printf("获取用户基本信息(UnionID机制)：%#v\n", resp)
			loginResp := &apipb.LoginResponse{
				Code: apipb.Code_Success,
			}
			model.LoginByWechat(true, &model.User{
				TenantModel: cmodel.TenantModel{
					TenantID: wechatOpenPlatformWeb.WechatConfig.TenantID,
				},
				RoleIDs: []string{wechatOpenPlatformWeb.WechatConfig.DefaultRoleID},
				UserRoles: []*model.UserRole{
					{RoleID: wechatOpenPlatformWeb.WechatConfig.DefaultRoleID},
				},
				UserName:       resp.UnionID,
				WechatOpenID:   resp.OpenID,
				WechatUnionID:  resp.UnionID,
				WechatConfigID: wechatOpenPlatformWeb.WechatConfig.ID,
				Nickname:       "未设置昵称",
				Enable:         true,
			}, loginResp)
			if loginResp.Code == apipb.Code_Success {
				wechatOpenPlatformWeb.UpdateQRConnectResult(msg.Ticket, true, true, loginResp.Data)
				// 回复欢迎消息
				c.XML(http.StatusOK, WechatNotifyRequest{
					ToUserName:   msg.FromUserName,
					FromUserName: msg.ToUserName,
					CreateTime:   time.Now().Unix(),
					MsgType:      "text",
					Content:      "欢迎关注我们的公众号，登录成功。",
				})
			} else {
				wechatOpenPlatformWeb.UpdateQRConnectResult(msg.Ticket, true, false, loginResp.Data)
				fmt.Printf("登录失败：%#v\n", loginResp)
				c.XML(http.StatusOK, WechatNotifyRequest{
					ToUserName:   msg.FromUserName,
					FromUserName: msg.ToUserName,
					CreateTime:   time.Now().Unix(),
					MsgType:      "text",
					Content:      "欢迎关注我们的公众号，登录成功失败。",
				})
			}
		} else {
			c.XML(http.StatusOK, WechatNotifyRequest{
				ToUserName:   msg.FromUserName,
				FromUserName: msg.ToUserName,
				CreateTime:   time.Now().Unix(),
				MsgType:      "text",
				Content:      "",
			})
		}
		return
	}
	c.String(http.StatusOK, "%s", c.Query("echostr"))
}

type WechatNotifyRequest struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
	Ticket       string   `xml:"Ticket"`
	Content      string   `xml:"Content"`
}
