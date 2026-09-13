package http

import (
	"net/http"
	"strings"

	"github.com/CloudSilk/usercenter/internal/wechatpay"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// PayConfigInfo 商户配置请求/响应体,读写均不携带 APIv3Key 与私钥明文。
type PayConfigInfo struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenantID" binding:"required"`
	WechatConfigID string `json:"wechatConfigID" binding:"required"`
	AppID          string `json:"appID" binding:"required"`
	MchID          string `json:"mchID" binding:"required"`
	MchSerialNo    string `json:"mchSerialNo" binding:"required"`
	APIV3Key       string `json:"apiV3Key" binding:"omitempty"`
	PrivateKey     string `json:"privateKey" binding:"omitempty"`
	NotifyURL      string `json:"notifyURL" binding:"omitempty,url,startswith=https"`
	Enable         bool   `json:"enable"`
	Description    string `json:"description" binding:"max=200"`
}

// PayConfigResponse 管理端通用响应。
type PayConfigResponse struct {
	Code    apipb.Code `json:"code"`
	Message string     `json:"message,omitempty"`
}

// PayConfigQueryRequest 分页查询请求。
type PayConfigQueryRequest struct {
	PageIndex      int    `form:"pageIndex" binding:"omitempty,gt=0"`
	PageSize       int    `form:"pageSize" binding:"omitempty,gt=0,lt=1000"`
	TenantID       string `form:"tenantID"`
	MchID          string `form:"mchID"`
	WechatConfigID string `form:"wechatConfigID"`
}

// PayConfigQueryResponse 分页查询响应。
type PayConfigQueryResponse struct {
	Code    apipb.Code       `json:"code"`
	Message string           `json:"message,omitempty"`
	Data    []*PayConfigInfo `json:"data,omitempty"`
	Records int64            `json:"records"`
	Pages   int64            `json:"pages"`
	Total   int64            `json:"total"`
}

// AddPayConfigRequest 新增商户配置,密钥字段必填。
type AddPayConfigRequest struct {
	PayConfigInfo
	APIV3Key   string `json:"apiV3Key" binding:"required,min=32,max=64"`
	PrivateKey string `json:"privateKey" binding:"required"`
}

// AddWechatPayConfig 新增微信支付商户配置。
//
//	@Summary 新增微信支付商户配置
//	@Tags 微信支付配置管理
//	@Param authorization header string true "jwt token"
//	@Param body body AddPayConfigRequest true "Add PayConfig"
//	@Success 200 {object} PayConfigResponse
//	@Router /api/core/wechat/pay/config/add [post]
func AddWechatPayConfig(c *gin.Context, req *AddPayConfigRequest) (*PayConfigResponse, error) {
	if err := validatePayPrivateKey(req.PrivateKey); err != nil {
		return &PayConfigResponse{Code: apipb.Code_BadRequest, Message: err.Error()}, nil
	}
	id, err := wechatpay.CreatePayConfig(&wechatpay.PayConfig{
		TenantID:       req.TenantID,
		WechatConfigID: req.WechatConfigID,
		AppID:          req.AppID,
		MchID:          req.MchID,
		MchSerialNo:    req.MchSerialNo,
		APIV3Key:       req.APIV3Key,
		PrivateKey:     req.PrivateKey,
		NotifyURL:      strings.TrimRight(req.NotifyURL, "/"),
		Enable:         req.Enable,
		Description:    req.Description,
	})
	if err != nil {
		return &PayConfigResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &PayConfigResponse{Code: apipb.Code_Success, Message: id}, nil
}

// UpdateWechatPayConfig 更新商户配置;apiV3Key/privateKey 留空表示保持原值。
//
//	@Summary 更新微信支付商户配置
//	@Tags 微信支付配置管理
//	@Param authorization header string true "jwt token"
//	@Param body body PayConfigInfo true "Update PayConfig"
//	@Success 200 {object} PayConfigResponse
//	@Router /api/core/wechat/pay/config/update [put]
func UpdateWechatPayConfig(c *gin.Context, req *PayConfigInfo) (*PayConfigResponse, error) {
	current, err := wechatpay.GetPayConfigByID(req.ID)
	if err != nil {
		return &PayConfigResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	if req.APIV3Key == "" {
		req.APIV3Key = current.APIV3Key
	}
	if req.PrivateKey == "" {
		req.PrivateKey = current.PrivateKey
	}
	if err := validatePayPrivateKey(req.PrivateKey); err != nil {
		return &PayConfigResponse{Code: apipb.Code_BadRequest, Message: err.Error()}, nil
	}
	current.TenantID = req.TenantID
	current.WechatConfigID = req.WechatConfigID
	current.AppID = req.AppID
	current.MchID = req.MchID
	current.MchSerialNo = req.MchSerialNo
	current.APIV3Key = req.APIV3Key
	current.PrivateKey = req.PrivateKey
	current.NotifyURL = strings.TrimRight(req.NotifyURL, "/")
	current.Enable = req.Enable
	current.Description = req.Description
	if err := wechatpay.UpdatePayConfig(current); err != nil {
		return &PayConfigResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	wechatpay.InvalidatePayAPI(current.ID)
	return &PayConfigResponse{Code: apipb.Code_Success}, nil
}

// DeleteWechatPayConfig 删除商户配置。
//
//	@Summary 删除微信支付商户配置
//	@Tags 微信支付配置管理
//	@Param authorization header string true "jwt token"
//	@Param id query string true "ID"
//	@Success 200 {object} PayConfigResponse
//	@Router /api/core/wechat/pay/config/delete [delete]
func DeleteWechatPayConfig(c *gin.Context) {
	resp := &PayConfigResponse{Code: apipb.Code_Success}
	id := c.Query("id")
	if id == "" {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "id不能为空"
		c.JSON(http.StatusOK, resp)
		return
	}
	if err := wechatpay.DeletePayConfig(id); err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	wechatpay.InvalidatePayAPI(id)
	c.JSON(http.StatusOK, resp)
}

// QueryWechatPayConfigs 分页查询商户配置(密钥脱敏)。
//
//	@Summary 分页查询微信支付商户配置
//	@Tags 微信支付配置管理
//	@Param authorization header string true "jwt token"
//	@Param pageIndex query int false "从1开始"
//	@Param pageSize query int false "默认每页10条"
//	@Success 200 {object} PayConfigQueryResponse
//	@Router /api/core/wechat/pay/config/query [get]
func QueryWechatPayConfigs(c *gin.Context, req *PayConfigQueryRequest) (*PayConfigQueryResponse, error) {
	resp := &PayConfigQueryResponse{Code: apipb.Code_Success, Data: []*PayConfigInfo{}}
	result, err := wechatpay.QueryPayConfigs(&wechatpay.PayConfigQuery{
		PageIndex:      req.PageIndex,
		PageSize:       req.PageSize,
		TenantID:       req.TenantID,
		MchID:          req.MchID,
		WechatConfigID: req.WechatConfigID,
	})
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return resp, nil
	}
	for _, item := range result.Records {
		resp.Data = append(resp.Data, payConfigToInfo(item))
	}
	resp.Records = result.Total
	resp.Pages = result.Pages
	resp.Total = result.Total
	return resp, nil
}

// GetWechatPayConfigDetail 查询商户配置明细(密钥脱敏)。
//
//	@Summary 查询微信支付商户配置明细
//	@Tags 微信支付配置管理
//	@Param authorization header string true "jwt token"
//	@Param id query string true "ID"
//	@Success 200 {object} PayConfigQueryResponse
//	@Router /api/core/wechat/pay/config/detail [get]
func GetWechatPayConfigDetail(c *gin.Context) {
	resp := &PayConfigQueryResponse{Code: apipb.Code_Success}
	id := c.Query("id")
	if id == "" {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "id不能为空"
		c.JSON(http.StatusOK, resp)
		return
	}
	cfg, err := wechatpay.GetPayConfigByID(id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = []*PayConfigInfo{payConfigToInfo(cfg)}
	}
	c.JSON(http.StatusOK, resp)
}

func validatePayPrivateKey(pem string) error {
	if _, err := utils.LoadPrivateKey(pem); err != nil {
		return err
	}
	return nil
}

// payConfigToInfo 模型转 VO,APIv3Key 与私钥永不回传前端。
func payConfigToInfo(cfg *wechatpay.PayConfig) *PayConfigInfo {
	return &PayConfigInfo{
		ID:             cfg.ID,
		TenantID:       cfg.TenantID,
		WechatConfigID: cfg.WechatConfigID,
		AppID:          cfg.AppID,
		MchID:          cfg.MchID,
		MchSerialNo:    cfg.MchSerialNo,
		NotifyURL:      cfg.NotifyURL,
		Enable:         cfg.Enable,
		Description:    cfg.Description,
	}
}

// RegisterWechatPayConfigRouter 挂载微信支付商户配置管理端点。
func RegisterWechatPayConfigRouter(r *gin.Engine) {
	g := r.Group("/api/core/wechat/pay/config")
	g.POST("add", AutoHandler(AddWechatPayConfig))
	g.PUT("update", AutoHandler(UpdateWechatPayConfig))
	g.GET("query", AutoQueryHandler(QueryWechatPayConfigs))
	g.DELETE("delete", DeleteWechatPayConfig)
	g.GET("detail", GetWechatPayConfigDetail)
}
