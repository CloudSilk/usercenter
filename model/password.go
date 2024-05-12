package model

import (
	"fmt"
	"math/rand"
	"regexp"
	"time"

	scrypt "github.com/elithrar/simple-scrypt"
)

func init() {
	//随机种子
	rand.Seed(time.Now().UnixNano())
}

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

//EncryptedPassword 对密码进行加密
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

//检测字符串中的空格
func test1() {
	for i := 0; i < len(CharStr); i++ {
		if CharStr[i] != ' ' {
			fmt.Printf("%c", CharStr[i])
		}
	}
}

type PwdStrength int

const (
	PwdStrengthOnliyNumber PwdStrength = iota
	PwdStrengthOnliyChar
	PwdStrengthMix
	PwdStrengthAdvance
)

func generatePasswd(length int, pwdStrength PwdStrength) string {
	//初始化密码切片
	var passwd []byte = make([]byte, length, length)
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

	//遍历，生成一个随机index索引,
	for i := 0; i < length; i++ {
		index := rand.Intn(len(sourceStr))
		passwd[i] = sourceStr[index]
	}
	return string(passwd)
}
