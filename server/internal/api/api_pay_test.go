package api

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
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"m5755/server/internal/domain"
	"m5755/server/internal/paychannel"
	"m5755/server/internal/store"
)

// payTestKeys 一对自造 RSA 测试密钥(单测内验证渠道验签逻辑正确;真单需真实资质)。
type payTestKeys struct {
	privPEM string
	pubPEM  string
	key     *rsa.PrivateKey
}

func genPayKeys(t *testing.T) payTestKeys {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(k)
	pkix, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
	return payTestKeys{
		privPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})),
		pubPEM:  string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix})),
		key:     k,
	}
}

// setupWithChannels 同 setup,但服务带微信/支付宝渠道签名器(测试密钥)。
// 返回 server、store、两渠道测试密钥(用于在测试内伪装渠道侧加签/加密)。
func setupWithChannels(t *testing.T) (*httptest.Server, *store.Store, payTestKeys, payTestKeys, string) {
	t.Helper()
	srv0, st := setup(t) // 复用 setup 的 DB 跳过/迁移逻辑
	srv0.Close()         // 关掉 setup 的 server,自建带渠道的

	wx := genPayKeys(t)
	ali := genPayKeys(t)
	apiv3Key := "01234567890123456789012345678901" // 32 字节

	wxSigner, err := paychannel.NewWechatSigner(paychannel.WechatConfig{
		MchID: "1900000001", AppID: "wxapp", SerialNo: "S1", APIv3Key: apiv3Key,
		PrivateKeyPEM: wx.privPEM, PlatformPublicKeyPEM: wx.pubPEM, NotifyURL: "https://x/pay/wxnotify",
	})
	if err != nil {
		t.Fatal(err)
	}
	aliSigner, err := paychannel.NewAlipaySigner(paychannel.AlipayConfig{
		AppID: "2021000000000001", AppPrivateKeyPEM: ali.privPEM, AlipayPublicKeyPEM: ali.pubPEM,
		NotifyURL: "https://x/pay/alinotify",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := domain.NewWith(st, domain.Options{
		CallbackSecret: "m5755-dev-callback-secret-v1", RealNameMock: true, SmsMock: true,
		Channels: domain.PaymentChannels{Wechat: wxSigner, Alipay: aliSigner},
	})
	srv := httptest.NewServer(NewRouter(svc, st, time.Now, "http://127.0.0.1:0"))
	t.Cleanup(srv.Close)
	return srv, st, wx, ali, apiv3Key
}

// payReceiver 与 api_m3_test.go 的 callbackReceiver 同形,但本文件独立持有以免耦合。
type payReceiver struct {
	mu   sync.Mutex
	hits []map[string]string
	srv  *httptest.Server
}

func newPayReceiver() *payReceiver {
	r := &payReceiver{}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		var m map[string]string
		_ = json.Unmarshal(b, &m)
		r.mu.Lock()
		r.hits = append(r.hits, m)
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"msg":"success"}`))
	}))
	return r
}

func (r *payReceiver) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.hits)
}

// createPaidOrder 走 SDK 链建一个待支付订单,返回 platformOrderId + 应收金额(元)。
func createPaidOrder(t *testing.T, srv *httptest.Server) (orderID, amount string) {
	t.Helper()
	account, token, _ := loginToSubaccount(t, srv)
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/orders", "", orderBody(account, token, nil), nil)
	if !ar.Success {
		t.Fatalf("建单失败: %s", ar.Message)
	}
	return ar.Data["platformOrderId"].(string), ar.Data["amount"].(string)
}

// ---------- 支付宝异步通知接收端(端到端) ----------

// signAlipayNotify 用应用私钥伪装支付宝侧加签(剔除 sign/sign_type/空,字典序 k=v&...)。
func signAlipayNotify(t *testing.T, key *rsa.PrivateKey, params map[string]string) url.Values {
	t.Helper()
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k + "=" + params[k])
	}
	sum := sha256.Sum256([]byte(sb.String()))
	raw, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("sign", base64.StdEncoding.EncodeToString(raw))
	form.Set("sign_type", "RSA2")
	return form
}

func postForm(t *testing.T, urlStr string, form url.Values) (int, string) {
	t.Helper()
	resp, err := http.Post(urlStr, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestAlipayNotifyEndToEnd(t *testing.T) {
	srv, st, _, ali, _ := setupWithChannels(t)
	rec := newPayReceiver()
	defer rec.srv.Close()
	if err := st.SetCallbackURL(t.Context(), seedGame, rec.srv.URL); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.SetCallbackURL(t.Context(), seedGame, "") })

	orderID, amount := createPaidOrder(t, srv)

	form := signAlipayNotify(t, ali.key, map[string]string{
		"out_trade_no": orderID, "trade_no": "alitxn1", "trade_status": "TRADE_SUCCESS",
		"total_amount": amount, "app_id": "2021000000000001",
	})

	// 首次:验签+反欺诈通过 → success + 充值回调触发一次
	if code, body := postForm(t, srv.URL+"/pay/alinotify", form); code != 200 || body != "success" {
		t.Fatalf("支付宝通知应回 success,得 code=%d body=%q", code, body)
	}
	if rec.count() != 1 {
		t.Fatalf("应触发一次充值回调,得 %d", rec.count())
	}

	// 幂等:同一通知重放 → 仍 success,但不重复触发回调
	if _, body := postForm(t, srv.URL+"/pay/alinotify", form); body != "success" {
		t.Fatalf("重复通知应仍回 success,得 %q", body)
	}
	if rec.count() != 1 {
		t.Fatalf("重复通知不应重复触发回调,得 %d", rec.count())
	}

	// 订单已推进
	o, err := st.GetOrder(t.Context(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	if o.PaymentStatus != "已支付" {
		t.Fatalf("订单应已支付,得 %s", o.PaymentStatus)
	}
}

func TestAlipayNotifyAmountMismatchRejected(t *testing.T) {
	srv, st, _, ali, _ := setupWithChannels(t)
	rec := newPayReceiver()
	defer rec.srv.Close()
	_ = st.SetCallbackURL(t.Context(), seedGame, rec.srv.URL)
	t.Cleanup(func() { _ = st.SetCallbackURL(t.Context(), seedGame, "") })

	orderID, _ := createPaidOrder(t, srv)
	// 金额被篡改为 0.01
	form := signAlipayNotify(t, ali.key, map[string]string{
		"out_trade_no": orderID, "trade_no": "alitxn2", "trade_status": "TRADE_SUCCESS",
		"total_amount": "0.01", "app_id": "2021000000000001",
	})
	if _, body := postForm(t, srv.URL+"/pay/alinotify", form); body != "failure" {
		t.Fatalf("金额不符应回 failure(渠道重推),得 %q", body)
	}
	if rec.count() != 0 {
		t.Fatalf("金额不符不应触发发放,得 %d", rec.count())
	}
}

func TestAlipayNotifyBadSignatureRejected(t *testing.T) {
	srv, st, _, _, _ := setupWithChannels(t)
	_ = st
	orderID, amount := createPaidOrder(t, srv)
	// 用错误私钥(另造一对)加签 → 验签失败
	wrong := genPayKeys(t)
	form := signAlipayNotify(t, wrong.key, map[string]string{
		"out_trade_no": orderID, "trade_no": "alitxn3", "trade_status": "TRADE_SUCCESS",
		"total_amount": amount, "app_id": "2021000000000001",
	})
	if _, body := postForm(t, srv.URL+"/pay/alinotify", form); body != "failure" {
		t.Fatalf("验签失败应回 failure,得 %q", body)
	}
}

// ---------- 微信 APIv3 回调接收端(端到端) ----------

// craftWechatNotify 伪装微信侧:AEAD 加密交易资源 + 平台私钥对 (ts\nnonce\nbody\n) 加签。
func craftWechatNotify(t *testing.T, key *rsa.PrivateKey, apiv3Key, outTradeNo string, fen int) (body []byte, ts, nonce, sig string) {
	t.Helper()
	txn := map[string]any{
		"out_trade_no": outTradeNo, "transaction_id": "wxtxn1", "trade_state": "SUCCESS",
		"amount": map[string]any{"total": fen, "currency": "CNY"},
	}
	plain, _ := json.Marshal(txn)
	block, _ := aes.NewCipher([]byte(apiv3Key))
	gcm, _ := cipher.NewGCM(block)
	aeadNonce := []byte("abcdefghijkl") // 12 字节
	aad := []byte("transaction")
	ct := gcm.Seal(nil, aeadNonce, plain, aad)

	env := map[string]any{
		"id": "evt1", "event_type": "TRANSACTION.SUCCESS", "resource_type": "encrypt-resource",
		"resource": map[string]any{
			"algorithm": "AEAD_AES_256_GCM", "ciphertext": base64.StdEncoding.EncodeToString(ct),
			"associated_data": string(aad), "nonce": string(aeadNonce),
		},
	}
	body, _ = json.Marshal(env)
	ts = strconv.FormatInt(time.Now().Unix(), 10)
	nonce = "notifynonce"
	msg := ts + "\n" + nonce + "\n" + string(body) + "\n"
	sum := sha256.Sum256([]byte(msg))
	raw, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	sig = base64.StdEncoding.EncodeToString(raw)
	return body, ts, nonce, sig
}

func postWechatNotify(t *testing.T, urlStr string, body []byte, ts, nonce, sig string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("POST", urlStr, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Wechatpay-Timestamp", ts)
	req.Header.Set("Wechatpay-Nonce", nonce)
	req.Header.Set("Wechatpay-Signature", sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestWechatNotifyEndToEnd(t *testing.T) {
	srv, st, wx, _, apiv3Key := setupWithChannels(t)
	rec := newPayReceiver()
	defer rec.srv.Close()
	_ = st.SetCallbackURL(t.Context(), seedGame, rec.srv.URL)
	t.Cleanup(func() { _ = st.SetCallbackURL(t.Context(), seedGame, "") })

	orderID, amount := createPaidOrder(t, srv)
	fen := amountToFen(t, amount)
	body, ts, nonce, sig := craftWechatNotify(t, wx.key, apiv3Key, orderID, fen)

	// 首次:验签+解密+反欺诈 → {code:SUCCESS} + 回调一次
	code, respBody := postWechatNotify(t, srv.URL+"/pay/wxnotify", body, ts, nonce, sig)
	if code != 200 || !strings.Contains(respBody, "SUCCESS") {
		t.Fatalf("微信通知应回 200 SUCCESS,得 code=%d body=%q", code, respBody)
	}
	if rec.count() != 1 {
		t.Fatalf("应触发一次充值回调,得 %d", rec.count())
	}
	// 幂等重放
	code2, respBody2 := postWechatNotify(t, srv.URL+"/pay/wxnotify", body, ts, nonce, sig)
	if code2 != 200 || !strings.Contains(respBody2, "SUCCESS") {
		t.Fatalf("重复微信通知应仍 SUCCESS,得 code=%d body=%q", code2, respBody2)
	}
	if rec.count() != 1 {
		t.Fatalf("幂等:不应重复触发回调,得 %d", rec.count())
	}
}

func TestWechatNotifyAmountMismatchRejected(t *testing.T) {
	srv, st, wx, _, apiv3Key := setupWithChannels(t)
	rec := newPayReceiver()
	defer rec.srv.Close()
	_ = st.SetCallbackURL(t.Context(), seedGame, rec.srv.URL)
	t.Cleanup(func() { _ = st.SetCallbackURL(t.Context(), seedGame, "") })

	orderID, _ := createPaidOrder(t, srv)
	// 篡改金额为 1 分
	body, ts, nonce, sig := craftWechatNotify(t, wx.key, apiv3Key, orderID, 1)
	code, respBody := postWechatNotify(t, srv.URL+"/pay/wxnotify", body, ts, nonce, sig)
	if code == 200 || strings.Contains(respBody, "SUCCESS") {
		t.Fatalf("金额不符应拒绝(非 SUCCESS),得 code=%d body=%q", code, respBody)
	}
	if rec.count() != 0 {
		t.Fatalf("金额不符不应发放,得 %d", rec.count())
	}
}

// amountToFen 复用 domain 的换算口径(测试侧简化:332.00→33200);此处直接 *100。
func amountToFen(t *testing.T, yuan string) int {
	t.Helper()
	f, err := strconv.ParseFloat(yuan, 64)
	if err != nil {
		t.Fatal(err)
	}
	return int(f*100 + 0.5)
}

// ---------- 收银台页 + 预下单 ----------
// 收银台首页 GET /pay/:orderId 两构建皆注册真收银台(ADR-0013):dev 注入渠道(沙箱)→ 真收银台,
// 无渠道 → dev 占位页;prod 恒真收银台(无渠道自渲染"暂无")。
// 生产内容断言见 api_pay_prod_test.go;dev 占位降级断言见 api_pay_dev_test.go(均 build-tag 特定行为)。

func TestCashierBeginAlipayReturnsURL(t *testing.T) {
	srv, _, _, _, _ := setupWithChannels(t)
	orderID, _ := createPaidOrder(t, srv)
	form := url.Values{}
	form.Set("orderId", orderID)
	form.Set("method", "alipay")
	resp, err := http.Post(srv.URL+"/pay/begin", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var ar apiResp
	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &ar)
	if !ar.Success || ar.Data["kind"] != "url" {
		t.Fatalf("支付宝预下单应返回 url 拉起: %+v", ar)
	}
	if ru, _ := ar.Data["redirectUrl"].(string); !strings.Contains(ru, "alipay.trade.wap.pay") {
		t.Fatalf("应返回支付宝 wap 跳转 URL: %v", ar.Data["redirectUrl"])
	}
}

func TestPayReturnSentinelPage(t *testing.T) {
	srv, _, _, _, _ := setupWithChannels(t)
	resp, err := http.Get(srv.URL + "/pay/return?status=handed&orderId=P5755x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("return sentinel 兜底页应 200,得 %d", resp.StatusCode)
	}
}
