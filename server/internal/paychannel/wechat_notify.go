package paychannel

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// WechatNotifyEnvelope 微信 APIv3 回调外层信封(签名验过后解析)。
type WechatNotifyEnvelope struct {
	ID           string `json:"id"`
	EventType    string `json:"event_type"`
	ResourceType string `json:"resource_type"`
	Resource     struct {
		Algorithm      string `json:"algorithm"`
		Ciphertext     string `json:"ciphertext"`
		AssociatedData string `json:"associated_data"`
		Nonce          string `json:"nonce"`
	} `json:"resource"`
}

// WechatTransaction 解密后的交易资源(只取反欺诈/对账必需字段)。
type WechatTransaction struct {
	OutTradeNo    string `json:"out_trade_no"` // = 平台 platformOrderId
	TransactionID string `json:"transaction_id"`
	TradeState    string `json:"trade_state"` // SUCCESS / REFUND / NOTPAY / CLOSED / ...
	Amount        struct {
		Total    int    `json:"total"`    // 订单总金额(分)
		Currency string `json:"currency"` // CNY
	} `json:"amount"`
}

// DecryptNotifyResource 用 APIv3 密钥(AES-256-GCM)解密回调 resource 密文。
// associatedData 与 nonce 来自信封;APIv3Key 必须为 32 字节。
func (s *WechatSigner) DecryptNotifyResource(env *WechatNotifyEnvelope) (*WechatTransaction, error) {
	if len(s.cfg.APIv3Key) != 32 {
		return nil, errors.New("APIv3 密钥必须为 32 字节(AES-256)")
	}
	if env.Resource.Algorithm != "AEAD_AES_256_GCM" {
		return nil, fmt.Errorf("不支持的回调加密算法: %s", env.Resource.Algorithm)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Resource.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("回调密文 base64 解码失败: %w", err)
	}
	block, err := aes.NewCipher([]byte(s.cfg.APIv3Key))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := []byte(env.Resource.Nonce)
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("回调 nonce 长度非法")
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(env.Resource.AssociatedData))
	if err != nil {
		return nil, fmt.Errorf("回调资源解密失败(AEAD): %w", err)
	}
	var txn WechatTransaction
	if err := json.Unmarshal(plain, &txn); err != nil {
		return nil, fmt.Errorf("交易资源解析失败: %w", err)
	}
	return &txn, nil
}

// ---------- 预下单(JSAPI / H5) ----------

// WechatPrepayInput 预下单入参(方式无关订单 → 渠道侧字段映射)。
type WechatPrepayInput struct {
	OutTradeNo  string // = platformOrderId
	Description string // 商品描述
	TotalFen    int    // 金额(分)
	OpenID      string // JSAPI 必填(玩家在该 appid 下的 openid);H5 留空
	PayerIP     string // 用户终端 IP(H5 必填)
}

// BuildJSAPIPrepayBody 构造 JSAPI 预下单请求体 JSON(/v3/pay/transactions/jsapi)。
func (s *WechatSigner) BuildJSAPIPrepayBody(in WechatPrepayInput) ([]byte, error) {
	if in.OpenID == "" {
		return nil, errors.New("JSAPI 预下单缺少 openid")
	}
	req := map[string]any{
		"appid":        s.cfg.AppID,
		"mchid":        s.cfg.MchID,
		"description":  in.Description,
		"out_trade_no": in.OutTradeNo,
		"notify_url":   s.cfg.NotifyURL,
		"amount":       map[string]any{"total": in.TotalFen, "currency": "CNY"},
		"payer":        map[string]any{"openid": in.OpenID},
	}
	return json.Marshal(req)
}

// BuildH5PrepayBody 构造 H5 预下单请求体 JSON(/v3/pay/transactions/h5)。
func (s *WechatSigner) BuildH5PrepayBody(in WechatPrepayInput) ([]byte, error) {
	if in.PayerIP == "" {
		return nil, errors.New("H5 预下单缺少用户终端 IP")
	}
	req := map[string]any{
		"appid":        s.cfg.AppID,
		"mchid":        s.cfg.MchID,
		"description":  in.Description,
		"out_trade_no": in.OutTradeNo,
		"notify_url":   s.cfg.NotifyURL,
		"amount":       map[string]any{"total": in.TotalFen, "currency": "CNY"},
		"scene_info":   map[string]any{"payer_client_ip": in.PayerIP, "h5_info": map[string]any{"type": "Wap"}},
	}
	return json.Marshal(req)
}
