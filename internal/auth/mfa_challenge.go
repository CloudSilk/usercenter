package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/patrickmn/go-cache"
)

// MFA 二阶段登录的 challenge 令牌存储。
//
// 登录流程：用户名密码校验通过后，若该用户绑定了启用的 MFA 因子，
// 则签发一个一次性、短生命周期的 challenge 令牌（绑 userID），返回 code=41008。
// 前端拿到 challenge 后展示 6 位码输入，调用 /mfa/verify 完成第二因素验证。
// 验证通过才签发真正的 access_token。

var mfaChallenges = cache.New(5*time.Minute, 10*time.Minute)

// IssueMFAChallenge 签发一次性 MFA challenge 令牌（绑定 userID，5 分钟有效）。
func IssueMFAChallenge(userID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := "mfa_" + hex.EncodeToString(b)
	mfaChallenges.Set(tok, userID, cache.DefaultExpiration)
	return tok, nil
}

// ConsumeMFAChallenge 校验并消费 challenge 令牌（一次性）。返回绑定的 userID。
func ConsumeMFAChallenge(tok string) (string, bool) {
	v, ok := mfaChallenges.Get(tok)
	if !ok {
		return "", false
	}
	mfaChallenges.Delete(tok) // 单次使用
	id, _ := v.(string)
	return id, true
}

// HasEnabledMFA 判断 principal 是否绑定了启用的 MFA 因子。
func HasEnabledMFA(principalID string) bool {
	if store.DB() == nil {
		return false
	}
	var count int64
	store.DB().Model(&MFAFactor{}).Where("principal_id = ? AND enable = ?", principalID, true).Count(&count)
	return count > 0
}

// VerifyMFACode 用 6 位码逐个校验 principal 的启用因子（解密 secret 后 TOTP 验证）。
// 命中即更新 last_used_at，返回 true。
func VerifyMFACode(principalID, code string) bool {
	if store.DB() == nil {
		return false
	}
	var factors []*MFAFactor
	store.DB().Where("principal_id = ? AND enable = ?", principalID, true).Find(&factors)
	for _, f := range factors {
		secret, err := DecryptPII(f.SecretEnc)
		if err != nil {
			continue
		}
		if VerifyTOTP(secret, code) {
			if e := store.DB().Model(&MFAFactor{}).Where("id = ?", f.ID).Update("last_used_at", time.Now().Unix()).Error; e != nil {
				log.Errorf(context.Background(), "update mfa factor last_used_at failed: %v", e)
			}
			return true
		}
	}
	return false
}
