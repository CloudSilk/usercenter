package auth

// PII 字段级加密 + 脱敏(REDESIGN #17)

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

var piiKey []byte

// SetPIIKey 设置 PII 加密密钥(32 字节,由 main.go 注入)
func SetPIIKey(key []byte) {
	if len(key) >= 32 {
		piiKey = key[:32]
	}
}

// EncryptPII 加密敏感字段(身份证/手机/邮箱)
func EncryptPII(plaintext string) (string, error) {
	if len(piiKey) == 0 {
		return "", errors.New("PII encryption key not set")
	}
	block, err := aes.NewCipher(piiKey)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(aesgcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

// DecryptPII 解密敏感字段
func DecryptPII(enc string) (string, error) {
	if len(piiKey) == 0 {
		return "", errors.New("PII encryption key not set")
	}
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(piiKey)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := aesgcm.NonceSize()
	if len(data) < ns {
		return "", errors.New("ciphertext too short")
	}
	pt, err := aesgcm.Open(nil, data[:ns], data[ns:], nil)
	return string(pt), err
}

// MaskPII 脱敏展示(不加密,仅前端展示用)
func MaskPII(s, piiType string) string {
	switch piiType {
	case "mobile":
		if len(s) >= 11 {
			return s[:3] + "****" + s[len(s)-4:]
		}
	case "email":
		at := strings.Index(s, "@")
		if at > 1 {
			return s[:2] + "***" + s[at:]
		}
	case "idCard":
		if len(s) >= 18 {
			return s[:6] + "********" + s[len(s)-4:]
		}
	}
	if len(s) > 4 {
		return s[:2] + "***" + s[len(s)-2:]
	}
	return "***"
}
