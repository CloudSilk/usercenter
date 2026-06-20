package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id 参数(NIST 800-63B 推荐)
const (
	argon2Memory      = 64 * 1024 // 64 MB
	argon2Iterations  = 3
	argon2Parallelism = 2
	argon2SaltLength  = 16
	argon2KeyLength   = 32
)

// EncryptedPasswordArgon2 使用 Argon2id 加密密码(自描述格式)
// 格式:$argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
func EncryptedPasswordArgon2(password string) (string, error) {
	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)
	b64salt := base64.RawStdEncoding.EncodeToString(salt)
	b64hash := base64.RawStdEncoding.EncodeToString(hash)
	return "$argon2id$v=19$m=" + itoa(argon2Memory) + ",t=" + itoa(argon2Iterations) + ",p=" + itoa(argon2Parallelism) + "$" + b64salt + "$" + b64hash, nil
}

// CompareArgon2 校验 Argon2id 密码
func CompareArgon2(encodedHash, password string) error {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return errors.New("invalid argon2id format")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return err
	}
	actualHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return err
	}
	computedHash := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)
	if subtle.ConstantTimeCompare(actualHash, computedHash) != 1 {
		return errors.New("password mismatch")
	}
	return nil
}

// IsArgon2Hash 判断是否为 Argon2id 格式(用于透明 rehash)
func IsArgon2Hash(encoded string) bool {
	return strings.HasPrefix(encoded, "$argon2id$")
}

// IsScryptHash 判断是否为旧 scrypt 格式(需 rehash)
func IsScryptHash(encoded string) bool {
	return !IsArgon2Hash(encoded) && len(encoded) > 0
}

// itoa 简易 int→string(避免 import strconv)
func itoa(n uint32) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
