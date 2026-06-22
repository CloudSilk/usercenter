package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"

	scrypt "github.com/elithrar/simple-scrypt"
)

// 从 model/password.go 迁入(REDESIGN §4 阶段0)。密码哈希属于认证体系(internal/auth)。

var (
	numberReg      = regexp.MustCompile("\\d+")
	lowerLetterReg  = regexp.MustCompile("[a-z]+")
	upperLetterReg  = regexp.MustCompile("[A-Z]+")
)

// ValidPasswdStrength 校验密码强度(>=8位 + 数字 + 小写 + 大写)
func ValidPasswdStrength(str string) bool {
	if len([]rune(str)) < 8 {
		return false
	}
	result := numberReg.MatchString(str)
	if !result {
		return false
	}
	result = lowerLetterReg.MatchString(str)
	if !result {
		return false
	}
	return upperLetterReg.MatchString(str)
}

// EncryptedPassword 对密码进行 scrypt 加密
func EncryptedPassword(password string) (string, error) {
	hash, err := scrypt.GenerateFromPassword([]byte(password), scrypt.DefaultParams)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

const (
	NUmStr  = "0123456789"
	CharStr = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	SpecStr = "+=-@#~,.[]()!%^*$"
)

// PwdStrength 密码强度等级
type PwdStrength int

const (
	PwdStrengthOnliyNumber PwdStrength = iota
	PwdStrengthOnliyChar
	PwdStrengthMix
	PwdStrengthAdvance
)

// GeneratePasswd 生成密码学安全的随机密码(crypto/rand 拒绝采样)
func GeneratePasswd(length int, pwdStrength PwdStrength) string {
	passwd := make([]byte, length)
	var sourceStr string
	switch pwdStrength {
	case PwdStrengthOnliyNumber:
		sourceStr = NUmStr
	case PwdStrengthOnliyChar:
		sourceStr = fmt.Sprintf("%s%s", NUmStr, CharStr)
	case PwdStrengthMix:
		sourceStr = fmt.Sprintf("%s%s", NUmStr, CharStr)
	default:
		sourceStr = fmt.Sprintf("%s%s%s", NUmStr, CharStr, SpecStr)
	}
	max := big.NewInt(int64(len(sourceStr)))
	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			passwd[i] = sourceStr[0]
			continue
		}
		passwd[i] = sourceStr[idx.Int64()]
	}
	return string(passwd)
}
