package user_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type phoneTestSender struct {
	phone string
	code  string
	calls int
	err   error
}

func (s *phoneTestSender) send(_ context.Context, phone, code string, _ int) error {
	s.phone = phone
	s.code = code
	s.calls++
	return s.err
}

func configurePhoneTest(
	t *testing.T,
	sender *phoneTestSender,
	now *time.Time,
	tenantID string,
	ready user.PhoneUserReadyHook,
) {
	t.Helper()
	if err := user.ConfigurePhoneAuth(user.PhoneAuthConfig{
		Enabled:          true,
		TenantID:         tenantID,
		HashKey:          "phone-test-hash-key",
		CodeTTLSeconds:   300,
		CooldownSeconds:  60,
		MaxAttempts:      3,
		PhoneHourlyLimit: 5,
		IPHourlyLimit:    20,
		ExposeDebugCode:  true,
		SendCode:         sender.send,
		UserReady:        ready,
		Now:              func() time.Time { return *now },
		GenerateCode:     func() (string, error) { return "123456", nil },
	}); err != nil {
		t.Fatalf("ConfigurePhoneAuth: %v", err)
	}
	t.Cleanup(func() {
		_ = user.ConfigurePhoneAuth(user.PhoneAuthConfig{Enabled: false})
	})
}

func cleanPhoneRows(t *testing.T, phone string) {
	t.Helper()
	_ = store.DB().Unscoped().Where("1 = 1").Delete(&user.PhoneVerificationChallenge{}).Error
	_ = store.DB().Unscoped().Where("mobile = ?", phone).Delete(&user.User{}).Error
}

func TestPhoneCodeUsesDigestsCooldownAndMaskedAudit(t *testing.T) {
	const phone = "13800138000"
	cleanPhoneRows(t, phone)
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	sender := &phoneTestSender{}
	configurePhoneTest(t, sender, &now, "phone-tenant", nil)

	result, err := user.IssuePhoneCode(t.Context(), " +86 138-0013-8000 ", "127.0.0.1", "req-1")
	if err != nil {
		t.Fatalf("IssuePhoneCode: %v", err)
	}
	if result.PhoneMasked != "138****8000" || result.DebugCode != "123456" || sender.phone != phone {
		t.Fatalf("result=%#v sender=%#v", result, sender)
	}
	var row user.PhoneVerificationChallenge
	if err := store.DB().Order("requested_at DESC").First(&row).Error; err != nil {
		t.Fatalf("load challenge: %v", err)
	}
	if row.PhoneHash == phone || row.CodeHash == "123456" || strings.Contains(row.CodeHash, "123456") ||
		row.IPHash == "127.0.0.1" {
		t.Fatalf("challenge leaked plaintext: %#v", row)
	}
	_, err = user.IssuePhoneCode(t.Context(), phone, "127.0.0.1", "req-2")
	var retry *user.PhoneRateLimitError
	if !errors.As(err, &retry) || retry.Scope != "cooldown" || retry.After != 60 {
		t.Fatalf("cooldown error = %#v / %v", retry, err)
	}
}

func TestPhoneLoginCreatesUserRunsHookAndIssuesNativeToken(t *testing.T) {
	const phone = "13900139000"
	const tenantID = "phone-new-user-tenant"
	cleanPhoneRows(t, phone)
	_ = tenant.CreateTenant(&tenant.Tenant{
		Model:     commonmodel.Model{ID: tenantID},
		Name:      tenantID,
		Enable:    true,
		Expired:   time.Now().Add(24 * time.Hour),
		UserCount: 10,
	})
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	sender := &phoneTestSender{}
	var hookUser string
	var hookNew bool
	configurePhoneTest(t, sender, &now, tenantID, func(_ context.Context, userID string, isNew bool) error {
		hookUser, hookNew = userID, isNew
		return nil
	})
	if _, err := user.IssuePhoneCode(t.Context(), phone, "10.0.0.1", "code"); err != nil {
		t.Fatalf("IssuePhoneCode: %v", err)
	}
	result, err := user.LoginByPhoneCode(t.Context(), phone, sender.code, "10.0.0.1", "login")
	if err != nil {
		t.Fatalf("LoginByPhoneCode: %v", err)
	}
	if result.Auth == nil || result.Auth.Code != apipb.Code_Success || result.Auth.Data == "" ||
		!result.IsNewUser || hookUser == "" || !hookNew {
		t.Fatalf("result=%#v hookUser=%q hookNew=%t", result, hookUser, hookNew)
	}
	current, err := token.DecodeToken(result.Auth.Data)
	if err != nil || current.Id != result.UserID || current.TenantID != tenantID {
		t.Fatalf("DecodeToken=%#v err=%v", current, err)
	}
	var created user.User
	if err := store.DB().Where("id = ?", result.UserID).First(&created).Error; err != nil {
		t.Fatalf("load created user: %v", err)
	}
	if created.Mobile != phone || created.Password == "" || !strings.HasPrefix(created.UserName, "mobile_") {
		t.Fatalf("created user = %#v", created)
	}
	if _, err := user.LoginByPhoneCode(t.Context(), phone, sender.code, "10.0.0.1", "reuse"); !errors.Is(err, user.ErrPhoneCodeUsed) {
		t.Fatalf("reused code error = %v", err)
	}
}

func TestPhoneLoginReusesExistingUserAndEnforcesMFA(t *testing.T) {
	const phone = "13700137000"
	cleanPhoneRows(t, phone)
	u := &user.User{
		TenantModel: commonmodel.TenantModel{TenantID: "existing-tenant"},
		UserName:    "phone-existing-user",
		Nickname:    "existing",
		Mobile:      phone,
		Enable:      true,
	}
	if err := store.DB().Create(u).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}
	factor := &auth.MFAFactor{
		ID:          "phone-mfa-" + u.ID,
		PrincipalID: u.ID,
		Type:        "totp",
		SecretEnc:   "not-used-for-challenge",
		Enable:      true,
		CreatedAt:   time.Now().Unix(),
	}
	if err := store.DB().Create(factor).Error; err != nil {
		t.Fatalf("create MFA factor: %v", err)
	}
	t.Cleanup(func() { _ = store.DB().Unscoped().Delete(factor).Error })

	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	sender := &phoneTestSender{}
	var hookNew bool
	configurePhoneTest(t, sender, &now, "existing-tenant", func(_ context.Context, _ string, isNew bool) error {
		hookNew = isNew
		return nil
	})
	if _, err := user.IssuePhoneCode(t.Context(), phone, "10.0.0.2", "code"); err != nil {
		t.Fatalf("IssuePhoneCode: %v", err)
	}
	result, err := user.LoginByPhoneCode(t.Context(), phone, sender.code, "10.0.0.2", "login")
	if err != nil {
		t.Fatalf("LoginByPhoneCode: %v", err)
	}
	if result.Auth == nil || result.Auth.Code != 41008 || result.Auth.Data == "" || result.IsNewUser || hookNew {
		t.Fatalf("MFA result = %#v hookNew=%t", result, hookNew)
	}
}

func TestPhoneCodeLocksAfterConfiguredAttemptsAndExpires(t *testing.T) {
	const phone = "13600136000"
	cleanPhoneRows(t, phone)
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	sender := &phoneTestSender{}
	configurePhoneTest(t, sender, &now, "phone-tenant", nil)
	if _, err := user.IssuePhoneCode(t.Context(), phone, "10.0.0.3", "code"); err != nil {
		t.Fatalf("IssuePhoneCode: %v", err)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		_, err := user.LoginByPhoneCode(t.Context(), phone, "000000", "10.0.0.3", fmt.Sprintf("wrong-%d", attempt))
		if attempt < 3 && !errors.Is(err, user.ErrInvalidPhoneCode) {
			t.Fatalf("attempt %d error = %v", attempt, err)
		}
		if attempt == 3 && !errors.Is(err, user.ErrPhoneCodeLocked) {
			t.Fatalf("attempt %d error = %v", attempt, err)
		}
	}
	if _, err := user.LoginByPhoneCode(t.Context(), phone, sender.code, "10.0.0.3", "locked"); !errors.Is(err, user.ErrPhoneCodeLocked) {
		t.Fatalf("locked code error = %v", err)
	}

	cleanPhoneRows(t, phone)
	if _, err := user.IssuePhoneCode(t.Context(), phone, "10.0.0.3", "code-2"); err != nil {
		t.Fatalf("IssuePhoneCode 2: %v", err)
	}
	now = now.Add(301 * time.Second)
	if _, err := user.LoginByPhoneCode(t.Context(), phone, sender.code, "10.0.0.3", "expired"); !errors.Is(err, user.ErrPhoneCodeExpired) {
		t.Fatalf("expired code error = %v", err)
	}
}

func TestPhoneCodeProviderFailureLeavesNoLiveChallenge(t *testing.T) {
	const phone = "13500135000"
	cleanPhoneRows(t, phone)
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	sender := &phoneTestSender{err: errors.New("provider unavailable")}
	configurePhoneTest(t, sender, &now, "phone-tenant", nil)
	if _, err := user.IssuePhoneCode(t.Context(), phone, "10.0.0.4", "request"); !errors.Is(err, user.ErrPhoneDelivery) {
		t.Fatalf("delivery error = %v", err)
	}
	var count int64
	if err := store.DB().Model(&user.PhoneVerificationChallenge{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("challenge count=%d err=%v", count, err)
	}
}
