package paychannel

import (
	"net/url"
	"strings"
	"testing"
)

func alipayTestConfig(t *testing.T) AlipayConfig {
	t.Helper()
	priv, pub, _ := testRSAKeyPair(t)
	return AlipayConfig{
		AppID:              "2021000000000001",
		AppPrivateKeyPEM:   priv,
		AlipayPublicKeyPEM: pub, // 单测自洽:应用私钥/公钥同对,验签可过
		NotifyURL:          "https://sdk.example.com/pay/alinotify",
		ReturnURL:          "https://sdk.example.com/pay/return?status=handed",
	}
}

func TestAlipayConfigValidate(t *testing.T) {
	if miss := (AlipayConfig{}).Validate(); len(miss) != 4 {
		t.Fatalf("空配置应报 4 项缺失,得 %v", miss)
	}
	if miss := alipayTestConfig(t).Validate(); len(miss) != 0 {
		t.Fatalf("完整配置不应报缺失,得 %v", miss)
	}
}

func TestAlipayBuildWapPayURL(t *testing.T) {
	s, err := NewAlipaySigner(alipayTestConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.BuildWapPayURL(AlipayWapInput{OutTradeNo: "P5755abc", Subject: "648 元宝", TotalAmount: "328.00"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	if q.Get("app_id") != "2021000000000001" || q.Get("method") != "alipay.trade.wap.pay" || q.Get("sign_type") != "RSA2" {
		t.Fatalf("URL 参数不符: %s", u)
	}
	if q.Get("sign") == "" {
		t.Fatal("应带 sign")
	}
	if !strings.Contains(q.Get("biz_content"), "P5755abc") || !strings.Contains(q.Get("biz_content"), "328.00") {
		t.Fatalf("biz_content 缺订单/金额: %s", q.Get("biz_content"))
	}
}

// TestAlipayNotifyVerifyRoundtrip 用应用私钥对 notify 参数加签,再用公钥验,应通过;篡改金额应失败。
func TestAlipayNotifyVerifyRoundtrip(t *testing.T) {
	s, err := NewAlipaySigner(alipayTestConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	params := map[string]string{
		"out_trade_no": "P5755abc",
		"trade_no":     "20210000",
		"trade_status": "TRADE_SUCCESS",
		"total_amount": "328.00",
		"app_id":       "2021000000000001",
		"sign_type":    "RSA2",
	}
	// 模拟支付宝加签:剔除 sign/sign_type/空,字典序 k=v&...
	signMe := make(map[string]string)
	for k, v := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		signMe[k] = v
	}
	sig, err := s.signParams(signMe)
	if err != nil {
		t.Fatal(err)
	}
	params["sign"] = sig

	if err := s.VerifyNotifySign(params); err != nil {
		t.Fatalf("合法通知应验过: %v", err)
	}
	// 篡改金额 → 验签失败
	tampered := make(map[string]string, len(params))
	for k, v := range params {
		tampered[k] = v
	}
	tampered["total_amount"] = "0.01"
	if err := s.VerifyNotifySign(tampered); err == nil {
		t.Fatal("篡改金额应验签失败")
	}
	// 缺 sign
	noSign := map[string]string{"out_trade_no": "P5755abc"}
	if err := s.VerifyNotifySign(noSign); err == nil {
		t.Fatal("缺 sign 应失败")
	}
}

func TestCanonicalAlipayParams(t *testing.T) {
	got := canonicalAlipayParams(map[string]string{"b": "2", "a": "1", "c": "", "sign": "x"})
	if got != "a=1&b=2" {
		t.Fatalf("规范化串应剔除 sign/空并字典序: %q", got)
	}
}
