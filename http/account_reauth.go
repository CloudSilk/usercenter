package http

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/user"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
)

const accountReauthTTL = 5 * time.Minute

type accountReauthChallenge struct {
	PrincipalID string
	Action      string
}

var (
	accountReauthChallenges = cache.New(accountReauthTTL, 10*time.Minute)
	accountReauthConsumeMu  sync.Mutex
)

// ReverifyAccount verifies the current password or a fresh code sent to the
// account's bound phone and, when enabled, an MFA code. The returned proof is
// short-lived, single-use, and bound to one exact high-risk action.
func ReverifyAccount(c *gin.Context) {
	principalID := strings.TrimSpace(ucm.GetUserID(c))
	var req struct {
		Password  string `json:"password"`
		PhoneCode string `json:"phoneCode"`
		MFACode   string `json:"mfaCode"`
		Action    string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, fmt.Errorf("重新验证参数不完整"))
		return
	}
	req.Action = strings.TrimSpace(req.Action)
	if principalID == "" || req.Action == "" || len(req.Action) > 200 {
		writeBadRequest(c, fmt.Errorf("重新验证参数无效"))
		return
	}
	primaryVerified := false
	if strings.TrimSpace(req.Password) != "" {
		primaryVerified = user.VerifyPassword(principalID, req.Password) == nil
	} else if strings.TrimSpace(req.PhoneCode) != "" {
		account, err := user.GetUserById(principalID)
		if err == nil && strings.TrimSpace(account.Mobile) != "" {
			boundPhone, normalizeErr := user.NormalizePhone(account.Mobile)
			verifiedPhone, verifyErr := user.VerifyPhoneCode(
				c.Request.Context(), account.Mobile, req.PhoneCode, c.ClientIP(), phoneRequestID(c),
			)
			primaryVerified = normalizeErr == nil && verifyErr == nil && verifiedPhone == boundPhone
		}
	}
	if !primaryVerified {
		writeBadRequest(c, fmt.Errorf("当前密码或验证码不正确"))
		return
	}
	if auth.HasEnabledMFA(principalID) && !auth.VerifyMFACode(principalID, strings.TrimSpace(req.MFACode)) {
		writeBadRequest(c, fmt.Errorf("当前密码或验证码不正确"))
		return
	}

	proofBytes := make([]byte, 32)
	if _, err := rand.Read(proofBytes); err != nil {
		writeErr(c, err)
		return
	}
	proof := "reauth_" + hex.EncodeToString(proofBytes)
	accountReauthChallenges.Set(proof, accountReauthChallenge{PrincipalID: principalID, Action: req.Action}, accountReauthTTL)
	recordAudit(c, "account_reverified", principalID, req.Action)
	writeOK(c, gin.H{"data": gin.H{"proof": proof, "expiresIn": int(accountReauthTTL.Seconds())}})
}

func requireAccountReauth(c *gin.Context, action string) bool {
	proof := strings.TrimSpace(c.GetHeader("X-UserCenter-Reauth"))
	if proof == "" {
		writeBadRequest(c, fmt.Errorf("请先重新验证身份"))
		return false
	}
	accountReauthConsumeMu.Lock()
	value, ok := accountReauthChallenges.Get(proof)
	accountReauthChallenges.Delete(proof)
	accountReauthConsumeMu.Unlock()
	challenge, valid := value.(accountReauthChallenge)
	if !ok || !valid || challenge.PrincipalID != strings.TrimSpace(ucm.GetUserID(c)) || challenge.Action != action {
		writeBadRequest(c, fmt.Errorf("重新验证已失效，请重试"))
		return false
	}
	return true
}
