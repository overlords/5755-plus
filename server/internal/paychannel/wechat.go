// Package paychannel 实现微信支付 APIv3 与支付宝手机网站支付的渠道侧加签/验签与预下单。
// 仅依赖 Go 标准库 crypto;商户密钥/私钥由调用方从 env 注入(fail-closed),本包不持久化、不打日志。
package paychannel

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WechatConfig 微信支付 APIv3 商户配置(env 注入,绝不入码)。
//   - MchID:商户号
//   - AppID:绑定的公众号/小程序/App appid
//   - SerialNo:商户 API 证书序列号(随请求头 Wechatpay-Serial 上送)
//   - APIv3Key:APIv3 密钥(用于解密回调资源,AES-256-GCM)
//   - PrivateKeyPEM:商户 API 私钥(PKCS#8 PEM),用于 APIv3 请求签名
//   - PlatformPublicKeyPEM:微信支付平台证书公钥(PEM),用于验回调签名
//   - NotifyURL:回调通知地址(平台服务端 /pay/wxnotify)
type WechatConfig struct {
	MchID                string
	AppID                string
	SerialNo             string
	APIv3Key             string
	PrivateKeyPEM        string
	PlatformPublicKeyPEM string
	NotifyURL            string
	Gateway              string // 可选:API 网关(默认 https://api.mch.weixin.qq.com;微信无独立 H5 沙箱)
}

// Validate 返回缺失字段名列表;非空即配置未就绪(fail-closed 依据)。
func (c WechatConfig) Validate() []string {
	var miss []string
	if c.MchID == "" {
		miss = append(miss, "WECHAT_MCH_ID")
	}
	if c.AppID == "" {
		miss = append(miss, "WECHAT_APP_ID")
	}
	if c.SerialNo == "" {
		miss = append(miss, "WECHAT_SERIAL_NO")
	}
	if c.APIv3Key == "" {
		miss = append(miss, "WECHAT_APIV3_KEY")
	}
	if c.PrivateKeyPEM == "" {
		miss = append(miss, "WECHAT_PRIVATE_KEY_PEM")
	}
	if c.PlatformPublicKeyPEM == "" {
		miss = append(miss, "WECHAT_PLATFORM_PUBLIC_KEY_PEM")
	}
	if c.NotifyURL == "" {
		miss = append(miss, "WECHAT_NOTIFY_URL")
	}
	return miss
}

// WechatSigner 持有解析好的私钥/公钥,提供 APIv3 加签与回调验签。
type WechatSigner struct {
	cfg        WechatConfig
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

// NewWechatSigner 解析私钥(请求签名)与平台证书公钥(回调验签)。
// PrivateKeyPEM 必填;PlatformPublicKeyPEM 可空(仅做预下单、不验回调时),
// 但调用方在生产应两者俱全(见 WechatConfig.Validate)。
func NewWechatSigner(cfg WechatConfig) (*WechatSigner, error) {
	priv, err := parseRSAPrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("微信私钥解析失败: %w", err)
	}
	s := &WechatSigner{cfg: cfg, privateKey: priv}
	if cfg.PlatformPublicKeyPEM != "" {
		pub, err := parseRSAPublicKey(cfg.PlatformPublicKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("微信平台公钥解析失败: %w", err)
		}
		s.publicKey = pub
	}
	return s, nil
}

// AuthorizationHeader 构造 APIv3 `Authorization` 头(WECHATPAY2-SHA256-RSA2048)。
// 签名串 = method\nurlPath\ntimestamp\nnonce\nbody\n,SHA256withRSA,base64。
func (s *WechatSigner) AuthorizationHeader(method, urlPath, body string, ts time.Time, nonce string) (string, error) {
	timestamp := strconv.FormatInt(ts.Unix(), 10)
	message := wechatSignMessage(method, urlPath, timestamp, nonce, body)
	sig, err := s.signSHA256(message)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		s.cfg.MchID, nonce, sig, timestamp, s.cfg.SerialNo,
	), nil
}

// wechatSignMessage 拼接 APIv3 请求/回调验签的标准串(每段以 \n 结尾)。
func wechatSignMessage(parts ...string) string {
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(p)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// signSHA256 SHA256withRSA(PKCS#1 v1.5)签名 → base64。
func (s *WechatSigner) signSHA256(message string) (string, error) {
	sum := sha256.Sum256([]byte(message))
	raw, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// VerifyNotifySignature 验证微信回调签名(Wechatpay-* 头 + 原始 body)。
// 验签串 = timestamp\nnonce\nbody\n;用平台证书公钥 SHA256withRSA 验。
func (s *WechatSigner) VerifyNotifySignature(timestamp, nonce, body, signatureB64 string) error {
	if s.publicKey == nil {
		return errors.New("未配置微信平台公钥,无法验回调签名")
	}
	if timestamp == "" || nonce == "" || signatureB64 == "" {
		return errors.New("微信回调缺少签名头")
	}
	// 防重放:时间戳须在 ±5 分钟窗口(微信文档建议)。
	if tsInt, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		if delta := time.Now().Unix() - tsInt; delta > 300 || delta < -300 {
			return errors.New("微信回调时间戳超出窗口")
		}
	} else {
		return errors.New("微信回调时间戳非法")
	}
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("微信回调签名 base64 解码失败: %w", err)
	}
	message := wechatSignMessage(timestamp, nonce, body)
	sum := sha256.Sum256([]byte(message))
	if err := rsa.VerifyPKCS1v15(s.publicKey, crypto.SHA256, sum[:], sig); err != nil {
		return fmt.Errorf("微信回调签名验证失败: %w", err)
	}
	return nil
}

// ---------- H5 预下单出网调用 ----------

// defaultWechatGateway 微信支付 APIv3 正式网关(微信无独立 H5 沙箱网关,默认即正式)。
const defaultWechatGateway = "https://api.mch.weixin.qq.com"

var wechatHTTPClient = &http.Client{Timeout: 15 * time.Second}

// PrepayH5 H5 预下单出网调用(/v3/pay/transactions/h5):构造并 APIv3 签名请求 → POST 微信网关
// → 验响应签名(平台证书公钥)→ 解析 h5_url。返回供收银台拉起微信的 h5_url。
// 商户密钥/证书由 env 注入;单一平台公钥,微信平台证书轮换期须更新 env 重启(serial 多证书池为后续增强)。
func (s *WechatSigner) PrepayH5(in WechatPrepayInput) (string, error) {
	body, err := s.BuildH5PrepayBody(in)
	if err != nil {
		return "", err
	}
	gateway := s.cfg.Gateway
	if gateway == "" {
		gateway = defaultWechatGateway
	}
	const urlPath = "/v3/pay/transactions/h5"
	nonce, err := wechatNonce()
	if err != nil {
		return "", err
	}
	auth, err := s.AuthorizationHeader("POST", urlPath, string(body), time.Now(), nonce)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", gateway+urlPath, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", auth)
	req.Header.Set("User-Agent", "m5755-server")
	resp, err := wechatHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("微信预下单请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("微信预下单 http=%d", resp.StatusCode)
	}
	// 验响应签名(平台证书公钥;未配置公钥则跳过——生产 Validate 要求必配)
	if s.publicKey != nil {
		if err := s.VerifyNotifySignature(
			resp.Header.Get("Wechatpay-Timestamp"),
			resp.Header.Get("Wechatpay-Nonce"),
			string(respBody),
			resp.Header.Get("Wechatpay-Signature"),
		); err != nil {
			return "", fmt.Errorf("微信预下单响应验签失败: %w", err)
		}
	}
	var parsed struct {
		H5URL string `json:"h5_url"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil || parsed.H5URL == "" {
		return "", errors.New("微信预下单响应缺 h5_url")
	}
	return parsed.H5URL, nil
}

// wechatNonce 生成 32 位十六进制随机串(APIv3 nonce_str)。
func wechatNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ---------- PEM 解析辅助 ----------

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemStr)))
	if block == nil {
		return nil, errors.New("PEM 解码失败(非法私钥)")
	}
	// PKCS#8(微信/支付宝常见)
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := key.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("私钥不是 RSA 类型")
	}
	// 回退 PKCS#1
	if rk, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return rk, nil
	}
	return nil, errors.New("私钥既非 PKCS#8 也非 PKCS#1")
}

func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemStr)))
	if block == nil {
		return nil, errors.New("PEM 解码失败(非法公钥)")
	}
	// 证书形态:取证书内公钥
	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		if pk, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return pk, nil
		}
		return nil, errors.New("证书公钥不是 RSA 类型")
	}
	// PKIX 公钥
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if pk, ok := pub.(*rsa.PublicKey); ok {
			return pk, nil
		}
		return nil, errors.New("公钥不是 RSA 类型")
	}
	// PKCS#1 公钥
	if pk, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return pk, nil
	}
	return nil, errors.New("公钥解析失败")
}
