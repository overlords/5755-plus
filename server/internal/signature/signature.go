// Package signature 实现 04 §1.3 入站验签:HMAC-SHA256 + 时间戳防重放窗口。
package signature

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"m5755/server/internal/result"
)

const (
	HeaderTimestamp = "X-M5755-Timestamp"
	HeaderKeyID     = "X-M5755-Key-Id"
	HeaderSignature = "X-M5755-Signature"
	WindowSeconds   = 300
	// ContextKeyPrincipal:验签通过后存入 gin context 的调用主体类型('sdk'/'server',ADR-0016),供端点授权。
	ContextKeyPrincipal = "keyPrincipal"
)

// KeyLookup 按 keyId 取签名密钥与主体类型('sdk'/'server',ADR-0016);ok=false 表示未知 keyId。
type KeyLookup func(ctx context.Context, keyID string) (secret, principal string, ok bool, err error)

// Canonical 构造签名原文:方法\n路径\n字典序query\n时间戳\n请求体(GET 体为空串)。
// 规范化 query 在原始 token(k=v)层面做字典序排序,避免 encode/decode 歧义,两端一致。
func Canonical(method, path, rawQuery, timestamp string, body []byte) string {
	q := ""
	if rawQuery != "" {
		parts := strings.Split(rawQuery, "&")
		sort.Strings(parts)
		q = strings.Join(parts, "&")
	}
	return strings.ToUpper(method) + "\n" + path + "\n" + q + "\n" + timestamp + "\n" + string(body)
}

// Compute 计算 HMAC-SHA256 十六进制小写签名。
func Compute(secret, canonical string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(canonical))
	return hex.EncodeToString(m.Sum(nil))
}

// Sign 供测试与 SDK 侧构造签名头(同一算法)。
func Sign(secret, keyID, method, path, rawQuery string, body []byte, ts int64) map[string]string {
	tss := strconv.FormatInt(ts, 10)
	return map[string]string{
		HeaderTimestamp: tss,
		HeaderKeyID:     keyID,
		HeaderSignature: Compute(secret, Canonical(method, path, rawQuery, tss, body)),
	}
}

// Middleware 返回 gin 验签中间件:全端点强制,失败写 ApiResult 并中断。
func Middleware(lookup KeyLookup, now func() time.Time) gin.HandlerFunc {
	if now == nil {
		now = time.Now
	}
	return func(c *gin.Context) {
		ts := c.GetHeader(HeaderTimestamp)
		keyID := c.GetHeader(HeaderKeyID)
		sig := c.GetHeader(HeaderSignature)
		if ts == "" || keyID == "" || sig == "" {
			result.WriteFail(c, 401, result.ReasonSignatureInvalid, "缺少签名头")
			c.Abort()
			return
		}

		tsInt, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			result.WriteFail(c, 401, result.ReasonSignatureInvalid, "时间戳格式非法")
			c.Abort()
			return
		}
		if delta := now().Unix() - tsInt; delta > WindowSeconds || delta < -WindowSeconds {
			result.WriteFail(c, 401, result.ReasonTimestampExpired, "时间戳超出允许窗口")
			c.Abort()
			return
		}

		secret, principal, ok, err := lookup(c.Request.Context(), keyID)
		if err != nil {
			result.WriteFail(c, 503, result.ReasonPlatformUnavailable, "验签密钥查询失败")
			c.Abort()
			return
		}
		if !ok {
			result.WriteFail(c, 401, result.ReasonSignatureInvalid, "未知 keyId")
			c.Abort()
			return
		}
		c.Set(ContextKeyPrincipal, principal) // 供端点授权(#86)区分 SDK / 游戏服务端调用者

		var body []byte
		if c.Request.Body != nil {
			body, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(body)) // 复位供 handler 再读
		}

		expected := Compute(secret, Canonical(c.Request.Method, c.Request.URL.Path, c.Request.URL.RawQuery, ts, body))
		if !hmac.Equal([]byte(expected), []byte(sig)) {
			result.WriteFail(c, 401, result.ReasonSignatureInvalid, "签名不匹配")
			c.Abort()
			return
		}

		c.Next()
	}
}
