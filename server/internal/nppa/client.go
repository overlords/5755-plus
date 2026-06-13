package nppa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Creds 单个接入游戏的 NPPA 凭据(per-game,接入者授权,见 ADR-0007)。
type Creds struct {
	AppID     string
	BizID     string
	SecretKey string
}

// Ready 凭据是否齐备(任一缺失则该游戏无法真核验)。
func (c Creds) Ready() bool {
	return c.AppID != "" && c.BizID != "" && c.SecretKey != ""
}

// 默认线上地址(规范 V2.1);测试用 BaseURLs 覆盖。
const (
	DefaultCheckURL = "https://api.wlc.nppa.gov.cn/idcard/authentication/check"
	DefaultQueryURL = "http://api2.wlc.nppa.gov.cn/idcard/authentication/query"
)

// Status 实名认证结果。
const (
	StatusSuccess = 0 // 认证成功
	StatusPending = 1 // 认证中(异步,48h 内查询)
	StatusFailed  = 2 // 认证失败
)

// CheckResult 认证/查询接口的结果对象。
type CheckResult struct {
	Status int
	PI     string
}

type respEnvelope struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
	Data    struct {
		Result struct {
			Status int    `json:"status"`
			PI     string `json:"pi"`
		} `json:"result"`
	} `json:"data"`
}

type checkPlain struct {
	AI    string `json:"ai"`
	Name  string `json:"name"`
	IDNum string `json:"idNum"`
}

// Check 实名认证接口(一):POST,加密 body + 签名头。
func Check(ctx context.Context, client *http.Client, creds Creds, checkURL, ai, name, idNum string, now time.Time) (*CheckResult, error) {
	if !creds.Ready() {
		return nil, errors.New("NPPA 凭据不全")
	}
	plain, _ := json.Marshal(checkPlain{AI: ai, Name: name, IDNum: idNum})
	enc, err := Encrypt(creds.SecretKey, string(plain))
	if err != nil {
		return nil, err
	}
	body := `{"data":"` + enc + `"}`
	ts := strconv.FormatInt(now.UnixMilli(), 10)
	sign := Sign(creds.SecretKey, map[string]string{"appId": creds.AppID, "bizId": creds.BizID, "timestamps": ts}, body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, checkURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	setAuthHeaders(req, creds, ts, sign)
	return do(client, req)
}

// Query 实名认证结果查询接口(二):GET,ai 拼 URL,无 body。
func Query(ctx context.Context, client *http.Client, creds Creds, queryURL, ai string, now time.Time) (*CheckResult, error) {
	if !creds.Ready() {
		return nil, errors.New("NPPA 凭据不全")
	}
	ts := strconv.FormatInt(now.UnixMilli(), 10)
	sign := Sign(creds.SecretKey, map[string]string{"ai": ai, "appId": creds.AppID, "bizId": creds.BizID, "timestamps": ts}, "")
	full := queryURL + "?ai=" + url.QueryEscape(ai)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	setAuthHeaders(req, creds, ts, sign)
	return do(client, req)
}

func setAuthHeaders(req *http.Request, creds Creds, ts, sign string) {
	req.Header.Set("appId", creds.AppID)
	req.Header.Set("bizId", creds.BizID)
	req.Header.Set("timestamps", ts)
	req.Header.Set("sign", sign)
}

func do(client *http.Client, req *http.Request) (*CheckResult, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env respEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("NPPA 响应解析失败 http=%d", resp.StatusCode)
	}
	if env.Errcode != 0 {
		return nil, fmt.Errorf("NPPA 业务异常 errcode=%d errmsg=%s", env.Errcode, env.Errmsg)
	}
	return &CheckResult{Status: env.Data.Result.Status, PI: env.Data.Result.PI}, nil
}
