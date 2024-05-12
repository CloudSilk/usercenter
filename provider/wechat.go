package provider

import (
	"context"
	"fmt"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/wechat"
)

type WechatProvider struct {
	apipb.UnimplementedWechatServer
}

func (u *WechatProvider) SendTplMsg(ctx context.Context, in *apipb.SendTplMsgRequest) (*apipb.SendTplMsgResponse, error) {
	resp := &apipb.SendTplMsgResponse{
		Code: commonmodel.Success,
	}
	wechatOpenPlatformWeb := wechat.GetWechatOpenPlatformWeb(in.App)
	if wechatOpenPlatformWeb == nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "非法APP"
		return resp, nil
	}
	openID, err := model.GetOpenIDByUserIDAndConfigID(in.ToUser, wechatOpenPlatformWeb.WechatConfig.ID)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return resp, nil
	}
	if openID == "" {
		resp.Message = "找不到OpenID"
		resp.Code = apipb.Code_UserIsNotExist
		return resp, nil
	}

	req := wechat.SendTplMsgRequest{
		ToUser:      openID,
		TemplateID:  in.TemplateID,
		Url:         in.Url,
		ClientMsgID: in.ClientMsgID,
		Data:        map[string]map[string]string{},
	}
	if in.MiniprogramApp != "" {
		miniProgramConfig := wechat.GetMiniProgram(in.MiniprogramApp)
		if miniProgramConfig == nil {
			resp.Code = apipb.Code_BadRequest
			resp.Message = "小程序APP名称不存在"
			return resp, nil
		}
		req.Miniprogram.AppID = miniProgramConfig.MiniAppConfig.AppID
		req.Miniprogram.PagePath = in.MiniprogramPagePath
	}

	for _, v := range in.Data {
		req.Data[v.Key] = map[string]string{
			"value": v.Value,
		}
	}
	fmt.Println(req)
	result, err := wechatOpenPlatformWeb.SendTplMsg(req)

	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else if result.ErrCode != 0 {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = fmt.Sprintf("微信返回的错误码：%d,错误信息:%s,消息ID:%d", result.ErrCode, result.ErrMsg, result.MsgID)
	}
	return resp, nil
}
