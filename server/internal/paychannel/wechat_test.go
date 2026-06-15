package paychannel

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testRSAKeyPair 生成一对自造 RSA 测试密钥(PKCS#8 私钥 PEM + PKIX 公钥 PEM)。
// 仅用于单测验证加签/验签逻辑正确;真单跑通需真实商户资质(见 issue #60 业务前置)。
func testRSAKeyPair(t *testing.T) (privPEM, pubPEM string, key *rsa.PrivateKey) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	pkix, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix}))
	return privPEM, pubPEM, k
}

func wechatTestConfig(t *testing.T) (WechatConfig, *rsa.PrivateKey) {
	t.Helper()
	priv, pub, key := testRSAKeyPair(t)
	return WechatConfig{
		MchID:                "1900000001",
		AppID:                "wxtestappid",
		SerialNo:             "SERIAL123",
		APIv3Key:             "01234567890123456789012345678901", // 32 字节
		PrivateKeyPEM:        priv,
		PlatformPublicKeyPEM: pub, // 单测中商户私钥即"平台公钥"对端,验签自洽
		NotifyURL:            "https://sdk.example.com/pay/wxnotify",
	}, key
}

func TestWechatConfigValidate(t *testing.T) {
	if miss := (WechatConfig{}).Validate(); len(miss) != 7 {
		t.Fatalf("空配置应报 7 项缺失,得 %v", miss)
	}
	cfg, _ := wechatTestConfig(t)
	if miss := cfg.Validate(); len(miss) != 0 {
		t.Fatalf("完整配置不应报缺失,得 %v", miss)
	}
}

func TestWechatAuthorizationHeaderShape(t *testing.T) {
	cfg, _ := wechatTestConfig(t)
	s, err := NewWechatSigner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	hdr, err := s.AuthorizationHeader("POST", "/v3/pay/transactions/jsapi", `{"a":1}`, time.Now(), "nonce123")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`mchid="1900000001"`, `serial_no="SERIAL123"`, `nonce_str="nonce123"`, "WECHATPAY2-SHA256-RSA2048"} {
		if !strings.Contains(hdr, want) {
			t.Fatalf("Authorization 头缺 %q: %s", want, hdr)
		}
	}
}

// TestWechatNotifySignVerify 用同一私钥造签名头,验签应通过;改 body 应失败。
func TestWechatNotifySignVerify(t *testing.T) {
	cfg, key := wechatTestConfig(t)
	s, err := NewWechatSigner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"id":"evt_1","event_type":"TRANSACTION.SUCCESS"}`
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "nonceABC"
	message := wechatSignMessage(ts, nonce, body)
	sum := sha256.Sum256([]byte(message))
	raw, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	sigB64 := base64.StdEncoding.EncodeToString(raw)

	if err := s.VerifyNotifySignature(ts, nonce, body, sigB64); err != nil {
		t.Fatalf("合法签名应验过: %v", err)
	}
	if err := s.VerifyNotifySignature(ts, nonce, body+"tampered", sigB64); err == nil {
		t.Fatal("篡改 body 应验签失败")
	}
	// 过期时间戳
	old := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	if err := s.VerifyNotifySignature(old, nonce, body, sigB64); err == nil {
		t.Fatal("过期时间戳应拒绝")
	}
}

// TestWechatDecryptNotifyResource 自造 AEAD_AES_256_GCM 密文,解出交易资源。
func TestWechatDecryptNotifyResource(t *testing.T) {
	cfg, _ := wechatTestConfig(t)
	s, err := NewWechatSigner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	txnJSON := `{"out_trade_no":"P5755abc","transaction_id":"4200001","trade_state":"SUCCESS","amount":{"total":32800,"currency":"CNY"}}`
	block, _ := aes.NewCipher([]byte(cfg.APIv3Key))
	gcm, _ := cipher.NewGCM(block)
	nonce := []byte("123456789012") // 12 字节
	aad := []byte("transaction")
	ct := gcm.Seal(nil, nonce, []byte(txnJSON), aad)

	env := &WechatNotifyEnvelope{}
	env.Resource.Algorithm = "AEAD_AES_256_GCM"
	env.Resource.Ciphertext = base64.StdEncoding.EncodeToString(ct)
	env.Resource.AssociatedData = string(aad)
	env.Resource.Nonce = string(nonce)

	txn, err := s.DecryptNotifyResource(env)
	if err != nil {
		t.Fatalf("解密应成功: %v", err)
	}
	if txn.OutTradeNo != "P5755abc" || txn.TradeState != "SUCCESS" || txn.Amount.Total != 32800 || txn.Amount.Currency != "CNY" {
		t.Fatalf("交易资源解析不符: %+v", txn)
	}
}

func TestWechatBuildPrepayBody(t *testing.T) {
	cfg, _ := wechatTestConfig(t)
	s, _ := NewWechatSigner(cfg)
	// JSAPI 缺 openid 应报错
	if _, err := s.BuildJSAPIPrepayBody(WechatPrepayInput{OutTradeNo: "P1", TotalFen: 100}); err == nil {
		t.Fatal("JSAPI 缺 openid 应报错")
	}
	body, err := s.BuildH5PrepayBody(WechatPrepayInput{OutTradeNo: "P5755x", Description: "648 元宝", TotalFen: 32800, PayerIP: "1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	if m["out_trade_no"] != "P5755x" || m["notify_url"] != cfg.NotifyURL {
		t.Fatalf("预下单体字段不符: %s", body)
	}
	amt := m["amount"].(map[string]any)
	if amt["total"].(float64) != 32800 {
		t.Fatalf("金额应为分: %v", amt["total"])
	}
}

// TestWechatPrepayH5OutCall 验证 H5 预下单出网调用全链:构造签名请求 → mock 微信网关 →
// 验响应签名(自洽密钥)→ 解析 h5_url;并验篡改响应被拒。不需真实商户资质(httptest + 测试密钥对)。
func TestWechatPrepayH5OutCall(t *testing.T) {
	cfg, key := wechatTestConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "WECHATPAY2-SHA256-RSA2048 ") {
			w.WriteHeader(401)
			return
		}
		respBody := `{"h5_url":"https://wx.tenpay.com/cgi-bin/h5pay?prepay_id=wx123"}`
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := "respnonce1234567"
		sum := sha256.Sum256([]byte(ts + "\n" + nonce + "\n" + respBody + "\n"))
		raw, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		w.Header().Set("Wechatpay-Timestamp", ts)
		w.Header().Set("Wechatpay-Nonce", nonce)
		w.Header().Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(raw))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
	defer srv.Close()
	cfg.Gateway = srv.URL
	s, err := NewWechatSigner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h5url, err := s.PrepayH5(WechatPrepayInput{OutTradeNo: "P5755x", Description: "648 元宝", TotalFen: 32800, PayerIP: "1.2.3.4"})
	if err != nil {
		t.Fatalf("预下单出网应成功: %v", err)
	}
	if h5url != "https://wx.tenpay.com/cgi-bin/h5pay?prepay_id=wx123" {
		t.Fatalf("h5_url 解析错: %s", h5url)
	}

	// 篡改响应签名 → 验签失败拒绝
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Wechatpay-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
		w.Header().Set("Wechatpay-Nonce", "n")
		w.Header().Set("Wechatpay-Signature", "AAAA")
		_, _ = w.Write([]byte(`{"h5_url":"https://evil/x"}`))
	}))
	defer bad.Close()
	cfg.Gateway = bad.URL
	s2, _ := NewWechatSigner(cfg)
	if _, err := s2.PrepayH5(WechatPrepayInput{OutTradeNo: "P5755y", Description: "x", TotalFen: 100, PayerIP: "1.2.3.4"}); err == nil {
		t.Fatal("响应验签失败应拒绝")
	}

	// 合法签名但 body 被掉包(签名对应另一内容)→ 内容完整性校验应拒(防 MITM 用合法签名配 evil h5_url)。
	tampered := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signedBody := `{"h5_url":"https://wx.tenpay.com/legit"}` // 真正被签名的内容
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := "tampnonce1234567"
		sum := sha256.Sum256([]byte(ts + "\n" + nonce + "\n" + signedBody + "\n"))
		raw, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		w.Header().Set("Wechatpay-Timestamp", ts)
		w.Header().Set("Wechatpay-Nonce", nonce)
		w.Header().Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(raw))
		w.Header().Set("Wechatpay-Serial", "ROTATED999")      // 顺带验 serial 进 error 不致命
		_, _ = w.Write([]byte(`{"h5_url":"https://evil/x"}`)) // 实发被掉包的 body
	}))
	defer tampered.Close()
	cfg.Gateway = tampered.URL
	s3, _ := NewWechatSigner(cfg)
	if _, err := s3.PrepayH5(WechatPrepayInput{OutTradeNo: "P5755z", Description: "x", TotalFen: 100, PayerIP: "1.2.3.4"}); err == nil {
		t.Fatal("合法签名 + 篡改 body 应验签失败被拒")
	}
}
