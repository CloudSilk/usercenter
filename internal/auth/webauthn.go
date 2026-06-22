package auth

// WebAuthn/Passkey 实现(REDESIGN #11,选型:go-webauthn/webauthn)
// 适配层 + RP 配置 + 注册/认证流程封装

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	webauthn "github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnCredential 存储格式(序列化为 JSON 存入 MFAFactor.SecretEnc)
type WebAuthnCredential struct {
	ID              []byte `json:"id"`
	PublicKey       []byte `json:"publicKey"`
	AttestationType string `json:"attestationType"`
	Transport       []string `json:"transport,omitempty"`
	SignCount       uint32  `json:"signCount"`
}

// WebAuthnUser 实现 webauthn.User 接口
type WebAuthnUser struct {
	ID          []byte
	Name        string
	DisplayName string
	Credentials []webauthn.Credential
}

func (u *WebAuthnUser) WebAuthnID() []byte           { return u.ID }
func (u *WebAuthnUser) WebAuthnName() string         { return u.Name }
func (u *WebAuthnUser) WebAuthnDisplayName() string  { return u.DisplayName }
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }
func (u *WebAuthnUser) WebAuthnIcon() string         { return "" }

// WebAuthnConfig RP 配置(从 systemconfig 加载)
type WebAuthnConfig struct {
	RPID          string
	RPDisplayName string
	RPOrigins     []string
}

var webAuthnInstance *webauthn.WebAuthn

// InitWebAuthn 初始化(由 main.go 调用)
func InitWebAuthn(cfg *WebAuthnConfig) error {
	if cfg == nil || cfg.RPID == "" {
		return nil
	}
	w, err := webauthn.New(&webauthn.Config{
		RPID:          cfg.RPID,
		RPDisplayName: cfg.RPDisplayName,
		RPOrigins:     cfg.RPOrigins,
	})
	if err != nil {
		return fmt.Errorf("init webauthn: %w", err)
	}
	webAuthnInstance = w
	return nil
}

func IsWebAuthnEnabled() bool { return webAuthnInstance != nil }

// BeginRegistration 开始注册
func BeginRegistration(userID, userName, displayName string, existingCreds []webauthn.Credential) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
	if !IsWebAuthnEnabled() {
		return nil, nil, fmt.Errorf("webauthn not enabled")
	}
	user := &WebAuthnUser{ID: []byte(userID), Name: userName, DisplayName: displayName, Credentials: existingCreds}
	return webAuthnInstance.BeginRegistration(user)
}

// FinishRegistration 完成注册
func FinishRegistration(userID, userName, displayName string, existingCreds []webauthn.Credential, session *webauthn.SessionData, parsedResponse *protocol.ParsedCredentialCreationData) (*webauthn.Credential, error) {
	if !IsWebAuthnEnabled() {
		return nil, fmt.Errorf("webauthn not enabled")
	}
	user := &WebAuthnUser{ID: []byte(userID), Name: userName, DisplayName: displayName, Credentials: existingCreds}
	return webAuthnInstance.CreateCredential(user, *session, parsedResponse)
}

// BeginLogin 开始认证
func BeginLogin(userID, userName, displayName string, existingCreds []webauthn.Credential) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	if !IsWebAuthnEnabled() {
		return nil, nil, fmt.Errorf("webauthn not enabled")
	}
	user := &WebAuthnUser{ID: []byte(userID), Name: userName, DisplayName: displayName, Credentials: existingCreds}
	return webAuthnInstance.BeginLogin(user)
}

// FinishLogin 完成认证
func FinishLogin(userID string, existingCreds []webauthn.Credential, session *webauthn.SessionData, parsedResponse *protocol.ParsedCredentialAssertionData) (*webauthn.Credential, error) {
	if !IsWebAuthnEnabled() {
		return nil, fmt.Errorf("webauthn not enabled")
	}
	user := &WebAuthnUser{ID: []byte(userID), Credentials: existingCreds}
	return webAuthnInstance.ValidateLogin(user, *session, parsedResponse)
}

// SerializeCredential 序列化为 JSON(存入 MFAFactor.SecretEnc)
func SerializeCredential(c *webauthn.Credential) (string, error) {
	wc := WebAuthnCredential{
		ID: c.ID, PublicKey: c.PublicKey,
		AttestationType: c.AttestationType, SignCount: c.Authenticator.SignCount,
	}
	for _, t := range c.Transport {
		wc.Transport = append(wc.Transport, string(t))
	}
	b, err := json.Marshal(wc)
	return string(b), err
}

// DeserializeCredentials 反序列化
func DeserializeCredentials(jsonStrs []string) ([]webauthn.Credential, error) {
	var creds []webauthn.Credential
	for _, s := range jsonStrs {
		var wc WebAuthnCredential
		if err := json.Unmarshal([]byte(s), &wc); err != nil {
			continue
		}
		transports := make([]protocol.AuthenticatorTransport, len(wc.Transport))
		for i, t := range wc.Transport {
			transports[i] = protocol.AuthenticatorTransport(t)
		}
		creds = append(creds, webauthn.Credential{
			ID: wc.ID, PublicKey: wc.PublicKey,
			AttestationType: wc.AttestationType,
			Transport: transports,
			Authenticator: webauthn.Authenticator{SignCount: wc.SignCount},
		})
	}
	return creds, nil
}

// Suppress unused
var _ = time.Now
