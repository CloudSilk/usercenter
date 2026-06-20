package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

type MFAFactor struct {
	ID           string `json:"id" gorm:"primaryKey;size:36"`
	PrincipalID  string `json:"principalID" gorm:"index;size:36"`
	Type         string `json:"type" gorm:"size:20;index;comment:totp/webauthn/sms"`
	SecretEnc    string `json:"-" gorm:"size:1000;comment:加密存储"`
	Name         string `json:"name" gorm:"size:100;comment:设备名称"`
	Enable       bool   `json:"enable" gorm:"index;default:true"`
	CreatedAt    int64  `json:"createdAt"`
	LastUsedAt   int64  `json:"lastUsedAt"`
}

func (MFAFactor) TableName() string { return "mfa_factor" }

func GenerateTOTPSecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(secret), nil
}

func GenerateTOTPURI(secret, account, issuer string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", issuer, account, secret, issuer)
}

func VerifyTOTP(secret, code string) bool {
	if len(code) != 6 {
		return false
	}
	now := time.Now().Unix() / 30
	for offset := -1; offset <= 1; offset++ {
		if generateTOTPCode(secret, now+int64(offset)) == code {
			return true
		}
	}
	return false
}

func generateTOTPCode(secret string, timestamp int64) string {
	key, err := base32.StdEncoding.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, uint64(timestamp))
	h := hmac.New(sha1.New, key)
	h.Write(msg)
	hash := h.Sum(nil)
	offset := int(hash[len(hash)-1] & 0x0f)
	code := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", code%1000000)
}

const (
	ACRLevel1 = "acr:level1"
	ACRLevel2 = "acr:level2"
	ACRLevel3 = "acr:level3"
)

func RequiredACRForAction(action string) string {
	switch action {
	case "delete_user", "reset_password", "change_password", "update_role":
		return ACRLevel2
	case "export_data", "delete_tenant":
		return ACRLevel3
	default:
		return ACRLevel1
	}
}
