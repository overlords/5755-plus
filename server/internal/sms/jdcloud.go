// Package sms 实现短信验证码 provider:dev=mock(返回 devCode 供联调),
// 生产=京东云 JDCloud batchSend(真发短信,不返回 devCode)。
// 参考既有后端 U10 的 JDCloud 集成(JDCLOUD2-HMAC-SHA256 签名),Go 重写不搬运。
package sms

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config 京东云短信配置(凭据由发布配置注入,不入库)。
type Config struct {
	AccessKeyID     string
	AccessKeySecret string
	SignID          string
	TemplateID      string
	Region          string // 默认 cn-north-1
	Endpoint        string // 默认 https://sms.jdcloud-api.com
}

// Result 发送结果(真实模式),用于响应元信息;绝不含验证码本身。
type Result struct {
	ProviderRequestID string
	ProviderBizID     string
	ProviderStatus    string
}

var regionRe = regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d+$`)

// Validate 校验 jdcloud 配置就绪(缺项/非法返回原因列表);空列表=就绪。
func (c Config) Validate() []string {
	var f []string
	if len(strings.TrimSpace(c.AccessKeyID)) < 4 {
		f = append(f, "JDCLOUD_SMS_ACCESS_KEY_ID_TOO_SHORT")
	}
	if len(strings.TrimSpace(c.AccessKeySecret)) < 8 {
		f = append(f, "JDCLOUD_SMS_ACCESS_KEY_SECRET_TOO_SHORT")
	}
	if len(strings.TrimSpace(c.SignID)) < 2 {
		f = append(f, "JDCLOUD_SMS_SIGN_ID_INVALID")
	}
	if len(strings.TrimSpace(c.TemplateID)) < 2 {
		f = append(f, "JDCLOUD_SMS_TEMPLATE_ID_INVALID")
	}
	region := c.region()
	if !regionRe.MatchString(region) {
		f = append(f, "JDCLOUD_SMS_REGION_INVALID")
	}
	if ep := strings.TrimSpace(c.Endpoint); ep != "" && !strings.HasPrefix(ep, "https://") &&
		!strings.HasPrefix(ep, "http://127.0.0.1") && !strings.HasPrefix(ep, "http://localhost") {
		f = append(f, "JDCLOUD_SMS_ENDPOINT_MUST_BE_HTTPS")
	}
	return f
}

func (c Config) region() string {
	if c.Region == "" {
		return "cn-north-1"
	}
	return c.Region
}

func (c Config) endpoint() string {
	if c.Endpoint == "" {
		return "https://sms.jdcloud-api.com"
	}
	return c.Endpoint
}

type batchSendBody struct {
	Params     []string `json:"params"`
	PhoneList  []string `json:"phoneList"`
	SignID     string   `json:"signId"`
	TemplateID string   `json:"templateId"`
}

// SendJDCloud 经 JDCloud batchSend 发送验证码;成功返回 Result,失败返回 error。
// now 注入便于测试稳定签名时间。
func SendJDCloud(ctx context.Context, client *http.Client, cfg Config, phone, code string, now time.Time) (*Result, error) {
	region := cfg.region()
	endpoint, err := url.Parse(cfg.endpoint())
	if err != nil {
		return nil, fmt.Errorf("endpoint 非法: %w", err)
	}
	reqURL := endpoint.Scheme + "://" + endpoint.Host + "/v1/regions/" + region + "/batchSend"
	u, err := url.Parse(reqURL)
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(batchSendBody{
		Params:     []string{code},
		PhoneList:  []string{phone},
		SignID:     cfg.SignID,
		TemplateID: cfg.TemplateID,
	})

	headers := signJDCloudV2(cfg, "POST", u, "sms", body, map[string]string{
		"accept":       "application/json",
		"content-type": "application/json",
		"user-agent":   "5755-platform jdcloud-sms",
	}, now)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var payload map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}
	root := payload
	result := asRecord(root["result"])
	if result == nil {
		result = root
	}
	data := asRecord(result["data"])
	if data == nil {
		data = asRecord(root["data"])
	}
	status := firstString(result["status"], root["status"])
	pcode := firstString(result["code"], root["code"])
	message := firstString(result["message"], root["message"])
	reqID := firstString(root["requestId"], root["RequestId"], result["requestId"])
	if reqID == "" {
		reqID = resp.Header.Get("x-jdcloud-request-id")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !accepted(status, pcode, message) {
		return nil, fmt.Errorf("京东云短信发送失败 httpStatus=%d code=%q status=%q message=%q", resp.StatusCode, pcode, status, message)
	}
	bizID := ""
	if data != nil {
		bizID = firstString(data["sequenceNumber"], data["taskId"], data["messageId"], data["smsId"])
	}
	st := pcode
	if st == "" {
		st = status
	}
	if st == "" {
		st = "ACCEPTED"
	}
	return &Result{ProviderRequestID: reqID, ProviderBizID: bizID, ProviderStatus: st}, nil
}

// signJDCloudV2 计算 JDCLOUD2-HMAC-SHA256 头(含 authorization),返回完整请求头。
func signJDCloudV2(cfg Config, method string, u *url.URL, service string, body []byte, base map[string]string, now time.Time) map[string]string {
	jdDate := now.UTC().Format("20060102T150405Z")
	date := now.UTC().Format("20060102")
	nb := make([]byte, 16)
	_, _ = rand.Read(nb)
	nonce := hex.EncodeToString(nb)

	headers := map[string]string{}
	for k, v := range base {
		headers[strings.ToLower(k)] = v
	}
	headers["x-jdcloud-date"] = jdDate
	headers["x-jdcloud-nonce"] = nonce

	// 参与签名的头:含 host,排除 authorization 与 user-agent,小写、压缩空白、按键排序
	type kv struct{ k, v string }
	var entries []kv
	add := func(k, v string) {
		k = strings.ToLower(k)
		if k == "authorization" || k == "user-agent" {
			return
		}
		entries = append(entries, kv{k, strings.Join(strings.Fields(v), " ")})
	}
	for k, v := range headers {
		add(k, v)
	}
	add("host", u.Host)
	sort.Slice(entries, func(i, j int) bool { return entries[i].k < entries[j].k })

	var canonHeaders strings.Builder
	var signedKeys []string
	for _, e := range entries {
		canonHeaders.WriteString(e.k + ":" + e.v + "\n")
		signedKeys = append(signedKeys, e.k)
	}
	signedHeaders := strings.Join(signedKeys, ";")
	payloadHash := sha256Hex(body)

	canonicalRequest := strings.Join([]string{
		method,
		canonicalPath(u.Path),
		"", // batchSend 无 query
		canonHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := date + "/" + cfg.region() + "/" + service + "/jdcloud2_request"
	stringToSign := strings.Join([]string{
		"JDCLOUD2-HMAC-SHA256",
		jdDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := jdcloudSigningKey(cfg.AccessKeySecret, date, cfg.region(), service)
	signature := hmacHex(signingKey, stringToSign)

	headers["authorization"] = "JDCLOUD2-HMAC-SHA256 Credential=" + cfg.AccessKeyID + "/" + credentialScope +
		", SignedHeaders=" + signedHeaders + ", Signature=" + signature
	return headers
}

func jdcloudSigningKey(secret, date, region, service string) []byte {
	kDate := hmacRaw([]byte("JDCLOUD2"+secret), date)
	kRegion := hmacRaw(kDate, region)
	kService := hmacRaw(kRegion, service)
	return hmacRaw(kService, "jdcloud2_request")
}

func accepted(status, code, message string) bool {
	statusOk := status == "" || in(strings.ToLower(status), "true", "success", "ok", "passed")
	codeOk := code == "" || in(strings.ToUpper(code), "0", "200", "OK", "SUCCESS")
	msgOk := message != "" && in(strings.ToLower(message), "执行成功", "success", "ok")
	return statusOk && (codeOk || msgOk)
}

// canonicalPath 对每段做 JDCloud 百分号编码后用 / 连接。
func canonicalPath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = percentEncode(s)
	}
	return strings.Join(parts, "/")
}

// percentEncode 等价 encodeURIComponent + 额外编码 !'()*。
func percentEncode(s string) string {
	var b strings.Builder
	for _, r := range []byte(s) {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' || r == '~' {
			b.WriteByte(r)
		} else {
			b.WriteString(fmt.Sprintf("%%%02X", r))
		}
	}
	return b.String()
}

func sha256Hex(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func hmacRaw(key []byte, msg string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(msg))
	return m.Sum(nil)
}

func hmacHex(key []byte, msg string) string { return hex.EncodeToString(hmacRaw(key, msg)) }

func asRecord(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func firstString(vals ...any) string {
	for _, v := range vals {
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return strings.TrimSpace(t)
			}
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64) // 整数状态码不渲染成科学计数法
		case bool:
			return fmt.Sprintf("%v", t)
		}
	}
	return ""
}

func in(s string, set ...string) bool {
	for _, x := range set {
		if s == x {
			return true
		}
	}
	return false
}
