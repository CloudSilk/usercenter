package model

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"

	scrypt "github.com/elithrar/simple-scrypt"
)

var (
	numberReg      = regexp.MustCompile("\\d+")
	lowerLetterReg = regexp.MustCompile("[a-z]+")
	upperLetterReg = regexp.MustCompile("[A-Z]+")
)

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

// EncryptedPassword 对密码进行加密
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

type PwdStrength int

const (
	PwdStrengthOnliyNumber PwdStrength = iota
	PwdStrengthOnliyChar
	PwdStrengthMix
	PwdStrengthAdvance
)

func generatePasswd(length int, pwdStrength PwdStrength) string {
	//初始化密码切片
	passwd := make([]byte, length)
	//源字符串
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

	//使用 crypto/rand 生成密码学安全的随机索引（拒绝采样由 rand.Int 内部保证，无模偏）
	max := big.NewInt(int64(len(sourceStr)))
	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			//crypto/rand 读取失败极罕见；回退到首字符以保证不 panic
			passwd[i] = sourceStr[0]
			continue
		}
		passwd[i] = sourceStr[idx.Int64()]
	}
	return string(passwd)
}
