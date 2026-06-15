package paychannel

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AlipayConfig 支付宝手机网站支付配置(env 注入,绝不入码)。
//   - AppID:支付宝应用 appid
//   - AppPrivateKeyPEM:应用私钥(PKCS#8 / PKCS#1 PEM),用于请求/参数 RSA2 加签
//   - AlipayPublicKeyPEM:支付宝公钥(PKIX/证书 PEM),用于验异步通知签名
//   - NotifyURL:异步通知地址(平台服务端 /pay/alinotify)
//   - ReturnURL:同步跳回地址(收银台 sentinel,可空)
//   - Gateway:网关地址(默认 https://openapi.alipay.com/gateway.do)
type AlipayConfig struct {
	AppID              string
	AppPrivateKeyPEM   string
	AlipayPublicKeyPEM string
	NotifyURL          string
	ReturnURL          string
	Gateway            string
}

const defaultAlipayGateway = "https://openapi.alipay.com/gateway.do"

// Validate 返回缺失字段名列表;非空即配置未就绪(fail-closed 依据)。
func (c AlipayConfig) Validate() []string {
	var miss []string
	if c.AppID == "" {
		miss = append(miss, "ALIPAY_APP_ID")
	}
	if c.AppPrivateKeyPEM == "" {
		miss = append(miss, "ALIPAY_APP_PRIVATE_KEY_PEM")
	}
	if c.AlipayPublicKeyPEM == "" {
		miss = append(miss, "ALIPAY_PUBLIC_KEY_PEM")
	}
	if c.NotifyURL == "" {
		miss = append(miss, "ALIPAY_NOTIFY_URL")
	}
	return miss
}

// AlipaySigner 持有解析好的私钥/公钥,提供请求加签与异步通知验签。
type AlipaySigner struct {
	cfg        AlipayConfig
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

// NewAlipaySigner 解析应用私钥(请求签名)与支付宝公钥(通知验签)。
func NewAlipaySigner(cfg AlipayConfig) (*AlipaySigner, error) {
	if cfg.Gateway == "" {
		cfg.Gateway = defaultAlipayGateway
	}
	priv, err := parseRSAPrivateKey(cfg.AppPrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("支付宝应用私钥解析失败: %w", err)
	}
	s := &AlipaySigner{cfg: cfg, privateKey: priv}
	if cfg.AlipayPublicKeyPEM != "" {
		pub, err := parseRSAPublicKey(cfg.AlipayPublicKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("支付宝公钥解析失败: %w", err)
		}
		s.publicKey = pub
	}
	return s, nil
}

// AlipayWapInput 手机网站支付预下单入参(方式无关订单 → 渠道侧字段映射)。
type AlipayWapInput struct {
	OutTradeNo  string // = orderId
	Subject     string // 商品标题
	TotalAmount string // 金额(元,两位小数字符串,如 "328.00")
}

// BuildWapPayURL 构造手机网站支付(alipay.trade.wap.pay)跳转 URL。
// SDK WebView 加载该 URL 即进入支付宝收银,付完支付宝异步 POST /pay/alinotify。
func (s *AlipaySigner) BuildWapPayURL(in AlipayWapInput) (string, error) {
	if in.OutTradeNo == "" || in.TotalAmount == "" {
		return "", errors.New("支付宝预下单缺少订单号或金额")
	}
	bizContent := fmt.Sprintf(
		`{"out_trade_no":%q,"total_amount":%q,"subject":%q,"product_code":"QUICK_WAP_WAY"}`,
		in.OutTradeNo, in.TotalAmount, in.Subject,
	)
	params := map[string]string{
		"app_id":      s.cfg.AppID,
		"method":      "alipay.trade.wap.pay",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  s.cfg.NotifyURL,
		"biz_content": bizContent,
	}
	if s.cfg.ReturnURL != "" {
		params["return_url"] = s.cfg.ReturnURL
	}
	sign, err := s.signParams(params)
	if err != nil {
		return "", err
	}
	params["sign"] = sign

	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return s.cfg.Gateway + "?" + q.Encode(), nil
}

// signParams 对参数(剔除 sign/空值)按键字典序拼成 k=v&... 串,SHA256withRSA → base64。
func (s *AlipaySigner) signParams(params map[string]string) (string, error) {
	str := canonicalAlipayParams(params)
	sum := sha256.Sum256([]byte(str))
	raw, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// VerifyNotifySign 验证支付宝异步通知签名(RSA2)。
// 规则:剔除 sign / sign_type / 空值,余下按键字典序 k=v 用 & 连接,用支付宝公钥验。
func (s *AlipaySigner) VerifyNotifySign(params map[string]string) error {
	if s.publicKey == nil {
		return errors.New("未配置支付宝公钥,无法验通知签名")
	}
	sign := params["sign"]
	if sign == "" {
		return errors.New("支付宝通知缺少 sign")
	}
	if st := params["sign_type"]; st != "" && st != "RSA2" {
		return fmt.Errorf("不支持的支付宝 sign_type: %s", st)
	}
	filtered := make(map[string]string, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		filtered[k] = v
	}
	str := canonicalAlipayParams(filtered)
	rawSig, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return fmt.Errorf("支付宝签名 base64 解码失败: %w", err)
	}
	sum := sha256.Sum256([]byte(str))
	if err := rsa.VerifyPKCS1v15(s.publicKey, crypto.SHA256, sum[:], rawSig); err != nil {
		return fmt.Errorf("支付宝通知签名验证失败: %w", err)
	}
	return nil
}

// canonicalAlipayParams 按键字典序拼接 k=v&...(剔除 sign/空由调用方负责)。
func canonicalAlipayParams(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || v == "" {
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
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(params[k])
	}
	return sb.String()
}
