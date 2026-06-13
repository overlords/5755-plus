// Package nppa 实现国家新闻出版署「网络游戏防沉迷实名认证系统」开放接口的
// 密码学与签名(规范 V2.1):请求体 AES-128-GCM + BASE64,签名 SHA256。
// 凭据(appId/bizId/secretKey)按接入游戏 per-game 由接入者授权(见 ADR-0007)。
// 本文件仅密码学/签名纯函数,可用规范示例黄金向量验证;HTTP 调用与流程在上层。
package nppa

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
)

// Sign 规范 §四:除 sign 外的系统/业务参数按 key 字典序拼成 Key-Value,
// 请求体拼在最后,secretKey 拼在最前,SHA256 取十六进制。
// 认证接口 params={appId,bizId,timestamps}+body;查询接口 params={ai,appId,bizId,timestamps}+空 body。
func Sign(secretKey string, params map[string]string, body string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s := secretKey
	for _, k := range keys {
		s += k + params[k]
	}
	s += body
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// aesKey:secretKey 为 32 位 hex,解码为 16 字节 AES-128 密钥。
func aesKey(secretKey string) ([]byte, error) {
	k, err := hex.DecodeString(secretKey)
	if err != nil || len(k) != 16 {
		return nil, errors.New("secretKey 须为 32 位 hex(AES-128 需 16 字节)")
	}
	return k, nil
}

// Encrypt 请求体加密:AES-128-GCM + BASE64,输出 base64(IV(12) || 密文 || tag(16))。
func Encrypt(secretKey, plaintext string) (string, error) {
	key, err := aesKey(secretKey)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, iv, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(append(iv, sealed...)), nil
}

// Decrypt 逆 Encrypt;也用于校验本端布局与 NPPA 一致(对官方密文解出官方明文)。
func Decrypt(secretKey, b64 string) (string, error) {
	key, err := aesKey(secretKey)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("密文长度不足")
	}
	pt, err := gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
