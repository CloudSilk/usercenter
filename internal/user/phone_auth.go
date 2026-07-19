package user

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
)

var (
	ErrPhoneLoginDisabled = errors.New("手机验证码登录未启用")
	ErrInvalidPhone       = errors.New("请输入正确的中国大陆手机号")
	ErrInvalidPhoneCode   = errors.New("验证码不正确")
	ErrPhoneCodeExpired   = errors.New("验证码已过期，请重新获取")
	ErrPhoneCodeUsed      = errors.New("验证码已使用，请重新获取")
	ErrPhoneCodeLocked    = errors.New("验证码错误次数过多，请重新获取")
	ErrPhoneDelivery      = errors.New("短信发送失败，请稍后重试")
)

var (
	phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)
	codePattern  = regexp.MustCompile(`^\d{6}$`)
)

// PhoneVerificationChallenge contains only keyed digests and display-safe
// metadata. Raw phone numbers, client IPs and codes are never stored here.
type PhoneVerificationChallenge struct {
	commonmodel.Model
	PhoneHash   string     `gorm:"index:idx_phone_code_phone_requested,priority:1;size:64;not null"`
	PhoneMasked string     `gorm:"size:20;not null"`
	IPHash      string     `gorm:"index:idx_phone_code_ip_requested,priority:1;size:64;not null"`
	CodeHash    string     `gorm:"size:64;not null"`
	RequestedAt time.Time  `gorm:"index:idx_phone_code_phone_requested,priority:2;index:idx_phone_code_ip_requested,priority:2;not null"`
	ExpiresAt   time.Time  `gorm:"index;not null"`
	Attempts    int        `gorm:"not null;default:0"`
	UsedAt      *time.Time `gorm:"index"`
}

func (PhoneVerificationChallenge) TableName() string {
	return "phone_verification_challenge"
}

// PhoneCodeSender is injected by the embedding application. It must return an
// error unless the downstream SMS provider accepted the message.
type PhoneCodeSender func(context.Context, string, string, int) error

// PhoneUserReadyHook lets an embedding application atomically prepare its own
// user-scoped resources before the access token is returned.
type PhoneUserReadyHook func(context.Context, string, bool) error

type PhoneAuthConfig struct {
	Enabled          bool
	TenantID         string
	HashKey          string
	CodeTTLSeconds   int
	CooldownSeconds  int
	MaxAttempts      int
	PhoneHourlyLimit int
	IPHourlyLimit    int
	ExposeDebugCode  bool
	SendCode         PhoneCodeSender
	UserReady        PhoneUserReadyHook
	Now              func() time.Time
	GenerateCode     func() (string, error)
}

type PhoneCodeIssueResult struct {
	PhoneMasked string
	ExpiresIn   int
	RetryAfter  int
	DebugCode   string
}

type PhoneLoginResult struct {
	Auth        *apipb.LoginResponse
	UserID      string
	Username    string
	PhoneMasked string
	IsNewUser   bool
}

// PhoneRateLimitError includes the durable limit scope and retry delay.
type PhoneRateLimitError struct {
	After int
	Scope string
}

func (e *PhoneRateLimitError) Error() string {
	return fmt.Sprintf("请求过于频繁，请在 %d 秒后重试", e.After)
}

type phoneAuthService struct {
	cfg       PhoneAuthConfig
	pepper    []byte
	operation sync.Mutex
}

var phoneAuthRuntime struct {
	sync.RWMutex
	service *phoneAuthService
}

// ConfigurePhoneAuth replaces the process-wide phone login configuration.
// Call it before registering the public phone routes.
func ConfigurePhoneAuth(cfg PhoneAuthConfig) error {
	if !cfg.Enabled {
		phoneAuthRuntime.Lock()
		phoneAuthRuntime.service = &phoneAuthService{cfg: cfg}
		phoneAuthRuntime.Unlock()
		return nil
	}
	if strings.TrimSpace(cfg.TenantID) == "" || strings.TrimSpace(cfg.HashKey) == "" || cfg.SendCode == nil {
		return errors.New("phone auth tenant, hash key and sender are required")
	}
	if cfg.CodeTTLSeconds < 60 || cfg.CodeTTLSeconds > 900 {
		return errors.New("phone auth code TTL must be between 60 and 900 seconds")
	}
	if cfg.CooldownSeconds < 30 || cfg.CooldownSeconds > cfg.CodeTTLSeconds {
		return errors.New("phone auth cooldown must be between 30 seconds and the code TTL")
	}
	if cfg.MaxAttempts < 1 || cfg.MaxAttempts > 10 {
		return errors.New("phone auth max attempts must be between 1 and 10")
	}
	if cfg.PhoneHourlyLimit < 1 || cfg.PhoneHourlyLimit > 20 ||
		cfg.IPHourlyLimit < cfg.PhoneHourlyLimit || cfg.IPHourlyLimit > 200 {
		return errors.New("phone auth hourly limits are invalid")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.GenerateCode == nil {
		cfg.GenerateCode = generatePhoneCode
	}
	phoneAuthRuntime.Lock()
	phoneAuthRuntime.service = &phoneAuthService{cfg: cfg, pepper: []byte(cfg.HashKey)}
	phoneAuthRuntime.Unlock()
	return nil
}

func PhoneAuthEnabled() bool {
	service := currentPhoneAuth()
	return service != nil && service.cfg.Enabled
}

func IssuePhoneCode(ctx context.Context, phone, clientIP, requestID string) (PhoneCodeIssueResult, error) {
	service := currentPhoneAuth()
	if service == nil || !service.cfg.Enabled {
		return PhoneCodeIssueResult{}, ErrPhoneLoginDisabled
	}
	return service.issueCode(ctx, phone, clientIP, requestID)
}

func LoginByPhoneCode(ctx context.Context, phone, code, clientIP, requestID string) (PhoneLoginResult, error) {
	service := currentPhoneAuth()
	if service == nil || !service.cfg.Enabled {
		return PhoneLoginResult{}, ErrPhoneLoginDisabled
	}
	return service.login(ctx, phone, code, clientIP, requestID)
}

func currentPhoneAuth() *phoneAuthService {
	phoneAuthRuntime.RLock()
	defer phoneAuthRuntime.RUnlock()
	return phoneAuthRuntime.service
}

func (s *phoneAuthService) issueCode(ctx context.Context, phone, clientIP, requestID string) (PhoneCodeIssueResult, error) {
	normalized, err := NormalizePhone(phone)
	if err != nil {
		s.recordAudit("", "", "phone_code_request", MaskPhone(phone), clientIP, "failure:invalid_phone request_id="+requestID)
		return PhoneCodeIssueResult{}, err
	}
	masked := MaskPhone(normalized)

	s.operation.Lock()
	defer s.operation.Unlock()

	now := s.cfg.Now().UTC()
	phoneHash := s.digest("phone", normalized)
	ipHash := s.digest("ip", normalizePhoneClientIP(clientIP))
	if retry, err := s.enforceLimits(ctx, phoneHash, ipHash, now); err != nil {
		reason := "failure:rate_limited"
		if retry != nil {
			reason = "failure:" + retry.Scope + "_limited"
		}
		s.recordAudit("", "", "phone_code_request", masked, clientIP, reason+" request_id="+requestID)
		return PhoneCodeIssueResult{}, err
	}

	code, err := s.cfg.GenerateCode()
	if err != nil {
		return PhoneCodeIssueResult{}, fmt.Errorf("generate phone code: %w", err)
	}
	challenge := &PhoneVerificationChallenge{
		PhoneHash:   phoneHash,
		PhoneMasked: masked,
		IPHash:      ipHash,
		RequestedAt: now,
		ExpiresAt:   now.Add(time.Duration(s.cfg.CodeTTLSeconds) * time.Second),
	}
	challenge.CodeHash = s.codeDigest(challenge.ID, phoneHash, code)
	// commonmodel.Model generates ID in BeforeCreate, while CodeHash binds to
	// the ID. Create once to obtain the ID, then persist the digest immediately.
	if err := store.DB().WithContext(ctx).Create(challenge).Error; err != nil {
		return PhoneCodeIssueResult{}, fmt.Errorf("store phone challenge: %w", err)
	}
	challenge.CodeHash = s.codeDigest(challenge.ID, phoneHash, code)
	if err := store.DB().WithContext(ctx).Model(challenge).Update("code_hash", challenge.CodeHash).Error; err != nil {
		_ = store.DB().WithContext(ctx).Unscoped().Delete(challenge).Error
		return PhoneCodeIssueResult{}, fmt.Errorf("finalize phone challenge: %w", err)
	}
	if err := s.cfg.SendCode(ctx, normalized, code, s.cfg.CodeTTLSeconds); err != nil {
		_ = store.DB().WithContext(ctx).Unscoped().Delete(challenge).Error
		s.recordAudit("", "", "phone_code_request", masked, clientIP, "failure:delivery request_id="+requestID)
		return PhoneCodeIssueResult{}, fmt.Errorf("%w: %v", ErrPhoneDelivery, err)
	}
	_ = store.DB().WithContext(ctx).Model(&PhoneVerificationChallenge{}).
		Where("phone_hash = ? AND id <> ? AND used_at IS NULL", phoneHash, challenge.ID).
		Update("used_at", now).Error
	s.recordAudit("", "", "phone_code_request", masked, clientIP, "success request_id="+requestID)
	s.cleanup(ctx, now)

	result := PhoneCodeIssueResult{
		PhoneMasked: masked,
		ExpiresIn:   s.cfg.CodeTTLSeconds,
		RetryAfter:  s.cfg.CooldownSeconds,
	}
	if s.cfg.ExposeDebugCode {
		result.DebugCode = code
	}
	return result, nil
}

func (s *phoneAuthService) login(ctx context.Context, phone, code, clientIP, requestID string) (PhoneLoginResult, error) {
	normalized, err := NormalizePhone(phone)
	if err != nil {
		s.recordAudit("", "", "phone_login", MaskPhone(phone), clientIP, "failure:invalid_phone request_id="+requestID)
		return PhoneLoginResult{}, err
	}
	masked := MaskPhone(normalized)
	code = strings.TrimSpace(code)
	if !codePattern.MatchString(code) {
		s.recordAudit("", "", "phone_login", masked, clientIP, "failure:invalid_code request_id="+requestID)
		return PhoneLoginResult{}, ErrInvalidPhoneCode
	}

	s.operation.Lock()
	defer s.operation.Unlock()

	now := s.cfg.Now().UTC()
	phoneHash := s.digest("phone", normalized)
	var challenge PhoneVerificationChallenge
	err = store.DB().WithContext(ctx).Where("phone_hash = ?", phoneHash).
		Order("requested_at DESC").First(&challenge).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.recordAudit("", "", "phone_login", masked, clientIP, "failure:code_not_found request_id="+requestID)
		return PhoneLoginResult{}, ErrInvalidPhoneCode
	}
	if err != nil {
		return PhoneLoginResult{}, fmt.Errorf("load phone challenge: %w", err)
	}
	switch {
	case challenge.UsedAt != nil:
		return PhoneLoginResult{}, ErrPhoneCodeUsed
	case !now.Before(challenge.ExpiresAt):
		return PhoneLoginResult{}, ErrPhoneCodeExpired
	case challenge.Attempts >= s.cfg.MaxAttempts:
		return PhoneLoginResult{}, ErrPhoneCodeLocked
	}
	expected := s.codeDigest(challenge.ID, phoneHash, code)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(challenge.CodeHash)) != 1 {
		attempts := challenge.Attempts + 1
		if err := store.DB().WithContext(ctx).Model(&PhoneVerificationChallenge{}).
			Where("id = ? AND used_at IS NULL", challenge.ID).
			Update("attempts", attempts).Error; err != nil {
			return PhoneLoginResult{}, fmt.Errorf("record phone code attempt: %w", err)
		}
		if attempts >= s.cfg.MaxAttempts {
			s.recordAudit("", "", "phone_login", masked, clientIP, "failure:attempts_exceeded request_id="+requestID)
			return PhoneLoginResult{}, ErrPhoneCodeLocked
		}
		s.recordAudit("", "", "phone_login", masked, clientIP, "failure:invalid_code request_id="+requestID)
		return PhoneLoginResult{}, ErrInvalidPhoneCode
	}
	consume := store.DB().WithContext(ctx).Model(&PhoneVerificationChallenge{}).
		Where("id = ? AND used_at IS NULL AND attempts < ?", challenge.ID, s.cfg.MaxAttempts).
		Update("used_at", now)
	if consume.Error != nil {
		return PhoneLoginResult{}, fmt.Errorf("consume phone code: %w", consume.Error)
	}
	if consume.RowsAffected != 1 {
		return PhoneLoginResult{}, ErrPhoneCodeUsed
	}

	u, isNew, err := s.findOrCreatePhoneUser(ctx, normalized, phoneHash)
	if err != nil {
		s.recordAudit("", "", "phone_login", masked, clientIP, "failure:user_initialization request_id="+requestID)
		return PhoneLoginResult{}, err
	}
	if !u.Enable {
		return PhoneLoginResult{}, errors.New("用户已禁用")
	}
	if u.LockedExpired > now.Unix() {
		return PhoneLoginResult{}, errors.New("账号已锁定，请稍后再试")
	}
	if s.cfg.UserReady != nil {
		if err := s.cfg.UserReady(ctx, u.ID, isNew); err != nil {
			return PhoneLoginResult{}, fmt.Errorf("prepare phone user: %w", err)
		}
	}

	resp := &apipb.LoginResponse{Code: apipb.Code_Success}
	issueVerifiedUserToken(u, resp)
	if resp.Code == apipb.Code_Success {
		s.recordAudit(u.ID, u.UserName, "phone_login", masked, clientIP, "success request_id="+requestID)
	} else {
		s.recordAudit(u.ID, u.UserName, "phone_login", masked, clientIP, "failure:token request_id="+requestID)
	}
	return PhoneLoginResult{
		Auth:        resp,
		UserID:      u.ID,
		Username:    u.UserName,
		PhoneMasked: masked,
		IsNewUser:   isNew,
	}, nil
}

func issueVerifiedUserToken(u *User, resp *apipb.LoginResponse) {
	if auth.HasEnabledMFA(u.ID) {
		challenge, err := auth.IssueMFAChallenge(u.ID)
		if err != nil {
			resp.Code = apipb.Code_InternalServerError
			resp.Message = err.Error()
			return
		}
		resp.Code = 41008
		resp.Message = "需要 MFA 二次验证"
		resp.Data = challenge
		return
	}
	current := &apipb.CurrentUser{
		Id: u.ID, UserName: u.UserName, Gender: u.Gender,
		RoleIDs: u.GetEnabledRoleIDs(), TenantID: u.TenantID, Nickname: u.Nickname, Avatar: u.Avatar,
	}
	encoded, err := token.EncodeToken(current)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return
	}
	resp.Data = encoded
}

func (s *phoneAuthService) findOrCreatePhoneUser(ctx context.Context, phone, phoneHash string) (*User, bool, error) {
	var users []*User
	if err := store.DB().WithContext(ctx).Preload("UserRoles").
		Where("tenant_id = ? AND mobile = ?", s.cfg.TenantID, phone).
		Limit(2).Find(&users).Error; err != nil {
		return nil, false, fmt.Errorf("load phone user: %w", err)
	}
	if len(users) > 1 {
		return nil, false, errors.New("multiple users are bound to the same phone")
	}
	if len(users) == 1 {
		return users[0], false, nil
	}

	u := &User{
		TenantModel: commonmodel.TenantModel{TenantID: s.cfg.TenantID},
		UserName:    "mobile_" + phoneHash[:16],
		Password:    "Aa1!" + auth.GeneratePasswd(20, auth.PwdStrengthAdvance),
		Nickname:    "用户 " + phone[len(phone)-4:],
		Mobile:      phone,
		Enable:      true,
	}
	if err := CreateUser(u, false); err != nil {
		// Another instance may have created the same mobile concurrently.
		var existing User
		if loadErr := store.DB().WithContext(ctx).Preload("UserRoles").
			Where("tenant_id = ? AND mobile = ?", s.cfg.TenantID, phone).
			First(&existing).Error; loadErr == nil {
			return &existing, false, nil
		}
		return nil, false, fmt.Errorf("create phone user: %w", err)
	}
	return u, true, nil
}

func (s *phoneAuthService) enforceLimits(ctx context.Context, phoneHash, ipHash string, now time.Time) (*PhoneRateLimitError, error) {
	var latest PhoneVerificationChallenge
	err := store.DB().WithContext(ctx).Where("phone_hash = ?", phoneHash).
		Order("requested_at DESC").First(&latest).Error
	if err == nil {
		remaining := time.Duration(s.cfg.CooldownSeconds)*time.Second - now.Sub(latest.RequestedAt)
		if remaining > 0 {
			retry := &PhoneRateLimitError{After: max(1, int(remaining.Seconds()+0.999)), Scope: "cooldown"}
			return retry, retry
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("load phone cooldown: %w", err)
	}

	window := now.Add(-time.Hour)
	var phoneCount int64
	if err := store.DB().WithContext(ctx).Model(&PhoneVerificationChallenge{}).
		Where("phone_hash = ? AND requested_at >= ?", phoneHash, window).Count(&phoneCount).Error; err != nil {
		return nil, fmt.Errorf("count phone requests: %w", err)
	}
	if phoneCount >= int64(s.cfg.PhoneHourlyLimit) {
		retry := &PhoneRateLimitError{
			After: phoneWindowReset(ctx, "phone_hash", phoneHash, window, now),
			Scope: "phone",
		}
		return retry, retry
	}
	var ipCount int64
	if err := store.DB().WithContext(ctx).Model(&PhoneVerificationChallenge{}).
		Where("ip_hash = ? AND requested_at >= ?", ipHash, window).Count(&ipCount).Error; err != nil {
		return nil, fmt.Errorf("count IP requests: %w", err)
	}
	if ipCount >= int64(s.cfg.IPHourlyLimit) {
		retry := &PhoneRateLimitError{
			After: phoneWindowReset(ctx, "ip_hash", ipHash, window, now),
			Scope: "ip",
		}
		return retry, retry
	}
	return nil, nil
}

func phoneWindowReset(ctx context.Context, field, value string, windowStart, now time.Time) int {
	var oldest PhoneVerificationChallenge
	if err := store.DB().WithContext(ctx).
		Where(field+" = ? AND requested_at >= ?", value, windowStart).
		Order("requested_at ASC").First(&oldest).Error; err != nil {
		return 3600
	}
	return max(1, int(oldest.RequestedAt.Add(time.Hour).Sub(now).Seconds()+0.999))
}

func (s *phoneAuthService) digest(kind, value string) string {
	mac := hmac.New(sha256.New, s.pepper)
	_, _ = mac.Write([]byte(kind))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *phoneAuthService) codeDigest(id, phoneHash, code string) string {
	return s.digest("code", id+"|"+phoneHash+"|"+code)
}

func (s *phoneAuthService) recordAudit(userID, userName, action, masked, clientIP, detail string) {
	audit.RecordAudit(store.DB(), userID, userName, action, masked, clientIP, detail)
}

func (s *phoneAuthService) cleanup(ctx context.Context, now time.Time) {
	_ = store.DB().WithContext(ctx).Where("expires_at < ?", now.Add(-24*time.Hour)).
		Delete(&PhoneVerificationChallenge{}).Error
}

// NormalizePhone accepts common +86/0086 prefixes and visual separators.
func NormalizePhone(value string) (string, error) {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(value)
	switch {
	case strings.HasPrefix(value, "+86"):
		value = strings.TrimPrefix(value, "+86")
	case strings.HasPrefix(value, "0086"):
		value = strings.TrimPrefix(value, "0086")
	}
	if !phonePattern.MatchString(value) {
		return "", ErrInvalidPhone
	}
	return value, nil
}

func MaskPhone(value string) string {
	normalized, err := NormalizePhone(value)
	if err != nil {
		return "***"
	}
	return normalized[:3] + "****" + normalized[7:]
}

func normalizePhoneClientIP(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	return value
}

func generatePhoneCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
