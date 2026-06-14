package main

import (
	"log"
	"os"
	"strings"

	"m5755/server/internal/domain"
	"m5755/server/internal/paychannel"
)

// buildPaymentChannels 从 env 构造微信/支付宝渠道签名器(#60)。
// 商户密钥/私钥/证书一律走 env 注入,绝不入码。语义:
//   - 某渠道 env 完全未配置 → 该渠道留 nil(预下单/验签在请求时 fail-closed 503),不静默放行。
//   - 某渠道 env 部分配置或密钥非法 → 不启用该渠道(留 nil)并**显著告警**,绝不带病启用。
//   - 两渠道皆可独立启用;真单跑通需真实商户资质(见 #60 业务前置)。
//
// publicBaseURL 用于推导回调/同步跳回地址(notify_url / return_url),与 paymentUrl 同源。
func buildPaymentChannels(publicBaseURL string) domain.PaymentChannels {
	base := strings.TrimRight(publicBaseURL, "/")
	var ch domain.PaymentChannels

	wxCfg := paychannel.WechatConfig{
		MchID:                os.Getenv("WECHAT_MCH_ID"),
		AppID:                os.Getenv("WECHAT_APP_ID"),
		SerialNo:             os.Getenv("WECHAT_SERIAL_NO"),
		APIv3Key:             os.Getenv("WECHAT_APIV3_KEY"),
		PrivateKeyPEM:        os.Getenv("WECHAT_PRIVATE_KEY_PEM"),
		PlatformPublicKeyPEM: os.Getenv("WECHAT_PLATFORM_PUBLIC_KEY_PEM"),
		NotifyURL:            base + "/pay/wxnotify",
	}
	if wxConfigured(wxCfg) {
		if miss := wxCfg.Validate(); len(miss) > 0 {
			log.Printf("警告:微信支付配置不完整 %v —— 微信支付不启用(收银台隐藏微信,预下单/回调 fail-closed)", miss)
		} else if signer, err := paychannel.NewWechatSigner(wxCfg); err != nil {
			log.Printf("警告:微信支付密钥/证书非法(%v)—— 微信支付不启用,绝不带病放行", err)
		} else {
			ch.Wechat = signer
			log.Printf("微信支付渠道已启用(mchId=%s, notify=%s)", wxCfg.MchID, wxCfg.NotifyURL)
		}
	}

	aliCfg := paychannel.AlipayConfig{
		AppID:              os.Getenv("ALIPAY_APP_ID"),
		AppPrivateKeyPEM:   os.Getenv("ALIPAY_APP_PRIVATE_KEY_PEM"),
		AlipayPublicKeyPEM: os.Getenv("ALIPAY_PUBLIC_KEY_PEM"),
		NotifyURL:          base + "/pay/alinotify",
		ReturnURL:          base + "/pay/return?status=handed",
		Gateway:            os.Getenv("ALIPAY_GATEWAY"), // 空 = 默认正式网关
	}
	if aliConfigured(aliCfg) {
		if miss := aliCfg.Validate(); len(miss) > 0 {
			log.Printf("警告:支付宝配置不完整 %v —— 支付宝不启用(收银台隐藏支付宝,预下单/回调 fail-closed)", miss)
		} else if signer, err := paychannel.NewAlipaySigner(aliCfg); err != nil {
			log.Printf("警告:支付宝应用私钥/公钥非法(%v)—— 支付宝不启用,绝不带病放行", err)
		} else {
			ch.Alipay = signer
			log.Printf("支付宝渠道已启用(appId=%s, notify=%s)", aliCfg.AppID, aliCfg.NotifyURL)
		}
	}

	if ch.Wechat == nil && ch.Alipay == nil {
		log.Printf("提示:无任何入站支付渠道启用(微信/支付宝商户资质未就绪)——收银台将提示暂无可用支付方式,/pay/*notify fail-closed")
	}
	return ch
}

// wxConfigured / aliConfigured 判断该渠道是否被运维"有意配置"(任一商户字段非空)。
// 仅当有意配置才进入 Validate/启用流程;完全空配视为该渠道不启用(无告警噪声)。
func wxConfigured(c paychannel.WechatConfig) bool {
	return c.MchID != "" || c.AppID != "" || c.SerialNo != "" || c.APIv3Key != "" ||
		c.PrivateKeyPEM != "" || c.PlatformPublicKeyPEM != ""
}

func aliConfigured(c paychannel.AlipayConfig) bool {
	return c.AppID != "" || c.AppPrivateKeyPEM != "" || c.AlipayPublicKeyPEM != ""
}
