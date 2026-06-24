package wechat_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	ucmodel "github.com/CloudSilk/usercenter/internal/wechatconfig"
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

// S5: 并发调用 GetAccessToken（写 AccessToken map），验证加锁后无 data race / panic。
// 用 `go test -race ./wechat/` 运行可检测到锁缺失时的竞争。
func TestAccessTokenConcurrentAccessNoRace(t *testing.T) {
	w := wechat.NewWechatOpenPlatformWeb(&ucmodel.WechatConfig{
		AppName: "myapp",
		Secret:  "0123456789abcdef", // AES-128 key (16 bytes)
		AppID:   "wxtest",
	})
	// 注入 mock httpGet，返回成功的 access token 响应，使 GetAccessToken 走到写 map 分支
	w.SetHTTPGet(func(url string) (*http.Response, error) {
		body := `{"access_token":"tok","expires_in":7200,"refresh_token":"rf","openid":"openid","scope":"snsapi_login","unionid":"union-1"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = w.GetAccessToken("any-code")
		}()
	}
	wg.Wait()
}

// S6: 注入一个会"超时"的 httpGet，验证 GetAccessToken 能正确返回错误而非永久阻塞。
func TestAccessTokenHandlesHTTPError(t *testing.T) {
	w := wechat.NewWechatOpenPlatformWeb(&ucmodel.WechatConfig{
		AppName: "myapp",
		Secret:  "0123456789abcdef",
		AppID:   "wxtest",
	})
	w.SetHTTPGet(func(url string) (*http.Response, error) {
		return nil, errors.New("simulated timeout")
	})

	if _, err := w.GetAccessToken("any-code"); err == nil {
		t.Fatal("expected error when HTTP call fails, got nil")
	}
}
