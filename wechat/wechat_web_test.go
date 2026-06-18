package wechat_test

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	ucmodel "github.com/CloudSilk/usercenter/model"
	"github.com/CloudSilk/usercenter/wechat"
)

// B3: state 加密往返正常，且对密文/nonce 篡改可被检出（AES-GCM 认证）。
func TestStateRoundtripAndTamperDetection(t *testing.T) {
	w := wechat.NewWechatOpenPlatformWeb(&ucmodel.WechatConfig{
		AppName: "myapp",
		Secret:  "0123456789abcdef", // AES-128 key (16 bytes)
	})

	state, err := w.EncryptState()
	if err != nil {
		t.Fatalf("EncryptState: %v", err)
	}
	parts := strings.SplitN(state, "_", 3)
	if len(parts) != 3 {
		t.Fatalf("unexpected state format: %q", state)
	}

	// 合法 state 应解密回 appName（含时间戳新鲜度校验）
	app, err := w.DecryptState(parts[1], parts[2])
	if err != nil || app != "myapp" {
		t.Fatalf("valid state should decrypt to myapp, got %q err=%v", app, err)
	}

	// 篡改密文 → AES-GCM 认证失败
	if _, err := w.DecryptState("YXRhbWVyZWRjaXBoZXI=", parts[2]); err == nil {
		t.Fatal("tampered ciphertext should fail to decrypt")
	}
	// 篡改 nonce（保持 12 字节长度，避免触发 GCM 的 nonce 长度 panic）→ 认证失败
	fakeNonce := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 12))
	if _, err := w.DecryptState(parts[1], fakeNonce); err == nil {
		t.Fatal("tampered nonce should fail to decrypt")
	}
}
