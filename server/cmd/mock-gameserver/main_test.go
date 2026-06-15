package main

import (
	"testing"

	"m5755/server/internal/domain"
)

// TestMockSignMatchesPlatform 把 mock 的 signCallback 与平台真相源 domain.callbackSign 绑死:
// mock 签出的 sign 必须能被平台 domain.VerifyCallbackSign 验过,否则支付宝沙箱联调的「全链」会假绿
// (mock 永远验不过、或恒真)。平台出站签名口径已为 HMAC-SHA256(ADR-0016);若再变,此测试立即 fail,
// 逼 mock 的 signCallback 同步跟进。用回调体定形的 9 个签名字段 + 中文 UTF-8,贴近真实回调。
func TestMockSignMatchesPlatform(t *testing.T) {
	const secret = "m5755-dev-callback-secret-v1" // dev 默认(bootstrap_dev.go),与平台 CALLBACK_SECRET 一致
	// 回调体定形(ADR-0016):account·orderId·cpOrderId·amount·payAmount·commodity·serverId·serverName·serverKeyId(·sign)。
	payload := map[string]string{
		"account":     "s_abc123",
		"orderId":     "P5755123456789",
		"cpOrderId":   "cp_order_1",
		"amount":      "6.00",
		"payAmount":   "6.00",
		"commodity":   "60 元宝",
		"serverId":    "s1",
		"serverName":  "星河一区", // 中文 UTF-8 字节参与签名
		"serverKeyId": "dev-server-key",
	}
	payload["sign"] = signCallback(payload, secret)

	if !domain.VerifyCallbackSign(payload, secret) {
		t.Fatal("mock signCallback 与平台 domain.callbackSign 不一致——沙箱联调会假绿;" +
			"平台出站签名口径若再变,请同步 mock 的 signCallback")
	}

	// 篡改任一业务字段后,平台重算签名应不匹配(证明绑定的是内容完整性、不是恒真)。
	tampered := make(map[string]string, len(payload))
	for k, v := range payload {
		tampered[k] = v
	}
	tampered["amount"] = "9999.00" // sign 仍是基于 amount=6.00 的旧值
	if domain.VerifyCallbackSign(tampered, secret) {
		t.Fatal("篡改 amount 后平台验签仍通过——签名未覆盖该字段,口径有误")
	}
}
