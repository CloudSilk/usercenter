package http

import (
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/store"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MFA 多因子注册（TOTP）。
//
//   POST /admin/api/mfa/totp/enroll  生成密钥 + otpauth URI（前端渲染二维码），暂不落库
//   POST /admin/api/mfa/totp/confirm  用户扫码后输入 6 位码校验，通过则加密落库 MFAFactor
//   GET  /admin/api/mfa/factors       列出当前用户已绑定的因子
//   DELETE /admin/api/mfa/:id         解绑因子

func registerMFARoutes(g *gin.RouterGroup) {
	m := g.Group("/mfa")
	m.POST("/totp/enroll", mfaTOTPEnroll)
	m.POST("/totp/confirm", mfaTOTPConfirm)
	m.GET("/factors", mfaListFactors)
	m.DELETE("/:id", mfaDeleteFactor)
}

// mfaTOTPEnroll 生成密钥。返回 secret + otpauth URI（前端用 qrcode 库渲染二维码）。
// secret 在 confirm 前不落库；前端需在 confirm 时回传。
func mfaTOTPEnroll(c *gin.Context) {
	account := ucm.GetUserName(c)
	if account == "" {
		account = ucm.GetUserID(c)
	}
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		writeErr(c, err)
		return
	}
	writeOK(c, gin.H{"data": gin.H{
		"secret": secret,
		"uri":    auth.GenerateTOTPURI(secret, account, "UserCenter"),
		"account": account,
	}})
}

// mfaTOTPConfirm 校验 6 位码并落库。
// body: {secret, code, name} —— secret 来自 enroll，name 为设备名（可空）。
func mfaTOTPConfirm(c *gin.Context) {
	var req struct {
		Secret string `json:"secret" binding:"required"`
		Code   string `json:"code" binding:"required"`
		Name   string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, err)
		return
	}
	if !auth.VerifyTOTP(req.Secret, req.Code) {
		writeOK(c, gin.H{"code": 40000, "message": "验证码不正确"})
		return
	}
	enc, err := auth.EncryptPII(req.Secret)
	if err != nil {
		writeErr(c, err)
		return
	}
	factor := &auth.MFAFactor{
		ID:          uuid.NewString(),
		PrincipalID: ucm.GetUserID(c),
		Type:        "totp",
		SecretEnc:   enc,
		Name:        req.Name,
		Enable:      true,
	}
	if err := store.DB().Create(factor).Error; err != nil {
		writeErr(c, err)
		return
	}
	recordAudit(c, "mfa_totp_enroll", factor.ID, ucm.GetUserID(c))
	writeOK(c, gin.H{"data": gin.H{"id": factor.ID, "name": factor.Name}})
}

func mfaListFactors(c *gin.Context) {
	var list []*auth.MFAFactor
	err := store.DB().Where("principal_id = ?", ucm.GetUserID(c)).Find(&list).Error
	if err != nil {
		writeErr(c, err)
		return
	}
	writeOK(c, gin.H{"data": list})
}

func mfaDeleteFactor(c *gin.Context) {
	id := c.Param("id")
	if err := store.DB().Delete(&auth.MFAFactor{}, "id = ?", id).Error; err != nil {
		writeErr(c, err)
		return
	}
	recordAudit(c, "mfa_unbind", id, "")
	writeOK(c, nil)
}
