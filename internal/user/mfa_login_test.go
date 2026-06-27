package user_test

import (
	"testing"

	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
)

// enrollTOTPFactor 为 userID 绑定一个启用的 TOTP 因子（加密 secret 落库），返回明文 secret。
func enrollTOTPFactor(t *testing.T, userID string) string {
	t.Helper()
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate totp secret: %v", err)
	}
	enc, err := auth.EncryptPII(secret)
	if err != nil {
		t.Fatalf("encrypt pii: %v", err)
	}
	f := &auth.MFAFactor{
		ID:          "factor-" + userID,
		PrincipalID: userID,
		Type:        "totp",
		SecretEnc:   enc,
		Name:        "test-device",
		Enable:      true,
	}
	if err := store.DB().Create(f).Error; err != nil {
		t.Fatalf("create mfa factor: %v", err)
	}
	return secret
}

func doLogin(t *testing.T, userName, password string) *apipb.LoginResponse {
	t.Helper()
	resp := &apipb.LoginResponse{Code: apipb.Code_Success}
	user.Login(&apipb.LoginRequest{UserName: userName, Password: password}, resp)
	return resp
}

// TestMFALogin_RequiresSecondFactor 回归：绑定 MFA 因子后，密码登录必须返回 41008 而非 access_token。
// 修复前 Login 完全忽略 MFA，直接签发 token —— 等于 MFA 形同虚设。
func TestMFALogin_RequiresSecondFactor(t *testing.T) {
	const pwd = "Secret123!"
	u := mustCreateUser(t, "mfauser", pwd)
	enrollTOTPFactor(t, u.ID)

	resp := doLogin(t, "mfauser", pwd)
	if resp.Code != 41008 {
		t.Fatalf("expected 41008 (MfaRequired) when user has MFA factor, got %v (data=%v)", resp.Code, resp.Data)
	}
	if resp.Data == "" {
		t.Fatal("expected non-empty MFA challenge token in resp.Data")
	}
}

// TestMFALogin_RejectsWrongCode 错误验证码 → 41010，且 challenge 一次性消费。
func TestMFALogin_RejectsWrongCode(t *testing.T) {
	const pwd = "Secret456!"
	u := mustCreateUser(t, "mfawrong", pwd)
	enrollTOTPFactor(t, u.ID)

	challenge := doLogin(t, "mfawrong", pwd).Data
	resp := &apipb.LoginResponse{Code: apipb.Code_Success}
	user.CompleteMFALogin(challenge, "000000", resp)
	if resp.Code != 41010 {
		t.Fatalf("expected 41010 (MfaCodeInvalid) for wrong code, got %v", resp.Code)
	}
}

// TestMFALogin_CompletesWithCorrectCode 正确码 → 签发 access_token（二阶段完成）。
func TestMFALogin_CompletesWithCorrectCode(t *testing.T) {
	const pwd = "Secret789!"
	u := mustCreateUser(t, "mfacorrect", pwd)
	secret := enrollTOTPFactor(t, u.ID)

	// 第一阶段：拿到 challenge
	challenge := doLogin(t, "mfacorrect", pwd).Data
	if challenge == "" {
		t.Fatal("expected challenge from first-phase login")
	}

	// 第二阶段：用当前窗口的正确码完成
	code := auth.GenerateTOTPCode(secret)
	resp := &apipb.LoginResponse{Code: apipb.Code_Success}
	user.CompleteMFALogin(challenge, code, resp)
	if resp.Code != apipb.Code_Success {
		t.Fatalf("expected success after correct MFA code, got %v (%s)", resp.Code, resp.Message)
	}
	if resp.Data == "" {
		t.Fatal("expected access token in resp.Data after MFA completion")
	}
}

// TestMFALogin_ChallengeSingleUse challenge 一次性：用过（即便失败）后再次使用应失效。
func TestMFALogin_ChallengeSingleUse(t *testing.T) {
	const pwd = "Secret000!"
	u := mustCreateUser(t, "mfaonce", pwd)
	enrollTOTPFactor(t, u.ID)

	challenge := doLogin(t, "mfaonce", pwd).Data
	// 第一次消费（错误码）→ 消费掉 challenge
	user.CompleteMFALogin(challenge, "000000", &apipb.LoginResponse{})
	// 同一 challenge 再用 → 应无效
	resp := &apipb.LoginResponse{Code: apipb.Code_Success}
	user.CompleteMFALogin(challenge, "123456", resp)
	if resp.Code != 41009 {
		t.Fatalf("expected 41009 (challenge consumed/expired) on reuse, got %v", resp.Code)
	}
}

// TestLogin_NoMFA_NormalToken 无 MFA 因子时登录正常签发 token（向后兼容）。
func TestLogin_NoMFA_NormalToken(t *testing.T) {
	const pwd = "Plain123!"
	mustCreateUser(t, "nomfauser", pwd)

	resp := doLogin(t, "nomfauser", pwd)
	if resp.Code != apipb.Code_Success {
		t.Fatalf("expected success for user without MFA, got %v (%s)", resp.Code, resp.Message)
	}
	if resp.Data == "" {
		t.Fatal("expected access token for non-MFA login")
	}
}
