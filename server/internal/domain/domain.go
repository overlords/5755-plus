// Package domain 承载 SDK 网关面的业务编排:配置、短信、5755 账户登录与首个小号保障。
// 业务规则以 04 契约为准;失败以 Fault 携带机器可读 reason 向上传递。
package domain

import (
	"context"
	"crypto/rand"
	"math/big"
	"regexp"
	"time"

	"m5755/server/internal/result"
	"m5755/server/internal/store"
)

// Fault 是带 reason 的业务失败,handler 据此写 ApiResult。
type Fault struct {
	HTTPStatus int
	Reason     string
	Message    string
}

func (f *Fault) Error() string { return f.Message }

func fault(status int, reason, msg string) *Fault { return &Fault{HTTPStatus: status, Reason: reason, Message: msg} }

var phoneRe = regexp.MustCompile(`^1\d{10}$`)

const (
	smsTTL          = 5 * time.Minute
	sessionTTL      = 24 * time.Hour
	smsRateWindow   = 60 * time.Second
	smsRateMaxInWin = 5
)

type Service struct {
	store          *store.Store
	now            func() time.Time
	callbackSecret string
	realNameMock   bool // dev=true(格式校验+mock 通过);prod=false(未配置真实 provider 时 fail-closed)
}

// Options 注入生产/联调差异(M4-S3:密钥环境注入,不再源码常量)。
type Options struct {
	CallbackSecret string
	RealNameMock   bool
}

func New(s *store.Store) *Service {
	// 兼容旧测试入口:dev 公开测试密钥 + mock 实名(联调口径)。
	return NewWith(s, Options{CallbackSecret: "m5755-dev-callback-secret-v1", RealNameMock: true})
}

func NewWith(s *store.Store, opt Options) *Service {
	return &Service{store: s, now: time.Now, callbackSecret: opt.CallbackSecret, realNameMock: opt.RealNameMock}
}

// CallbackSecret 供测试接收端验证签名。
func (svc *Service) CallbackSecret() string { return svc.callbackSecret }

// ---------- 配置 ----------

type Maintenance struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}

type ConfigData struct {
	GameID                      string      `json:"gameId"`
	GameName                    string      `json:"gameName"`
	Maintenance                 Maintenance `json:"maintenance"`
	AntiAddictionEntryBlocked   bool        `json:"antiAddictionEntryBlocked"`
	AntiAddictionPaymentBlocked bool        `json:"antiAddictionPaymentBlocked"`
	ProtocolVersion             string      `json:"protocolVersion"`
	RequestID                   string      `json:"requestId"`
	ConfigVersion               string      `json:"configVersion"`
	SDKLatestVersion            string      `json:"sdkLatestVersion"`
	SDKMinVersion               string      `json:"sdkMinVersion"`
	UpdateRequired              bool        `json:"updateRequired"`
	LoginDomain                 string      `json:"loginDomain"`
	PaymentDomain               string      `json:"paymentDomain"`
}

// GetConfig 返回初始化配置;gameId 缺失或游戏不存在为阻断型失败。
func (svc *Service) GetConfig(ctx context.Context, gameID, sdkVersion, requestID string) (*ConfigData, *Fault) {
	if gameID == "" {
		return nil, fault(400, result.ReasonParamInvalid, "缺少 gameId")
	}
	g, err := svc.store.GetGameConfig(ctx, gameID)
	if err == store.ErrNotFound {
		return nil, fault(404, result.ReasonParamInvalid, "游戏不存在")
	}
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "配置读取失败")
	}
	return &ConfigData{
		GameID:                      g.GameID,
		GameName:                    g.GameName,
		Maintenance:                 Maintenance{Enabled: g.MaintenanceEnabled, Message: g.MaintenanceMessage},
		AntiAddictionEntryBlocked:   g.AntiAddictionEntryBlocked,
		AntiAddictionPaymentBlocked: g.AntiAddictionPaymentBlocked,
		ProtocolVersion:             g.ProtocolVersion,
		RequestID:                   requestID,
		ConfigVersion:               g.ConfigVersion,
		SDKLatestVersion:            g.SDKLatestVersion,
		SDKMinVersion:               g.SDKMinVersion,
		UpdateRequired:              compareVersion(sdkVersion, g.SDKMinVersion) < 0,
		LoginDomain:                 g.LoginDomain,
		PaymentDomain:               g.PaymentDomain,
	}, nil
}

// ---------- 短信验证码 ----------

type SmsData struct {
	CodeID             string `json:"codeId"`
	LoginAccountMasked string `json:"loginAccountMasked"`
	ExpiresAt          string `json:"expiresAt"`
	ProviderMode       string `json:"providerMode"`
	ProviderStatus     string `json:"providerStatus"`
	DevCode            string `json:"devCode,omitempty"`
}

func (svc *Service) RequestSmsCode(ctx context.Context, gameID, loginAccount string) (*SmsData, *Fault) {
	if gameID == "" || loginAccount == "" {
		return nil, fault(400, result.ReasonParamInvalid, "缺少 gameId 或 loginAccount")
	}
	if !phoneRe.MatchString(loginAccount) {
		return nil, fault(400, result.ReasonParamInvalid, "loginAccount 必须是手机号")
	}
	n, err := svc.store.CountRecentSmsCodes(ctx, gameID, loginAccount, smsRateWindow)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "短信请求失败")
	}
	if n >= smsRateMaxInWin {
		return nil, fault(429, result.ReasonSmsRateLimited, "验证码请求过于频繁")
	}
	code := genNumericCode(6)
	sc, err := svc.store.CreateSmsCode(ctx, gameID, loginAccount, code, smsTTL)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "短信签发失败")
	}
	return &SmsData{
		CodeID:             sc.CodeID,
		LoginAccountMasked: maskPhone(loginAccount),
		ExpiresAt:          sc.ExpiresAt.UTC().Format(time.RFC3339),
		ProviderMode:       "mock",
		ProviderStatus:     "accepted",
		DevCode:            code, // mock 模式允许返回;不进诊断/日志
	}, nil
}

// ---------- 5755 账户登录 ----------

type CreatedSubaccount struct {
	Account           string `json:"account"`
	GameID            string `json:"gameId"`
	PlatformAccountID string `json:"platformAccountId"`
	DisplayName       string `json:"displayName"`
	IsDefault         bool   `json:"isDefault"`
}

type GameEntry struct {
	IsNewGameUser     bool               `json:"isNewGameUser"`
	CreatedSubaccount *CreatedSubaccount `json:"createdSubaccount,omitempty"`
}

type LoginData struct {
	PlatformAccountID string    `json:"platformAccountId"`
	PlatformToken     string    `json:"platformToken"`
	DisplayName       string    `json:"displayName"`
	GameEntry         GameEntry `json:"gameEntry"`
	ExpiresAt         string    `json:"expiresAt"`
}

type LoginInput struct {
	GameID          string
	LoginMethod     string
	LoginAccount    string
	Credential      string
	ChannelID       string
	ChannelSource   string
	DeviceID        string // #25:安装级随机 ID,非硬件标识
	DeviceVerifyCode string
}

func (svc *Service) Login(ctx context.Context, in LoginInput) (*LoginData, *Fault) {
	if in.GameID == "" || in.LoginAccount == "" || in.Credential == "" || in.LoginMethod == "" {
		return nil, fault(400, result.ReasonParamInvalid, "缺少登录必填字段")
	}
	g, err := svc.store.GetGameConfig(ctx, in.GameID)
	if err == store.ErrNotFound {
		return nil, fault(404, result.ReasonParamInvalid, "游戏不存在")
	}
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "登录失败")
	}
	if g.MaintenanceEnabled {
		return nil, fault(503, result.ReasonMaintenance, "平台维护中")
	}

	// 解析账户:sms 可建新账户;password 必须已存在并通过密码+设备校验(#25)。
	var accountID, displayName string
	var isNew bool

	switch in.LoginMethod {
	case "sms":
		if !phoneRe.MatchString(in.LoginAccount) {
			return nil, fault(400, result.ReasonParamInvalid, "loginAccount 必须是手机号")
		}
		res, err := svc.store.ConsumeSmsCode(ctx, in.GameID, in.LoginAccount, in.Credential)
		if err != nil {
			return nil, fault(503, result.ReasonPlatformUnavailable, "验证码校验失败")
		}
		switch res {
		case store.SmsConsumeExpired:
			return nil, fault(401, result.ReasonSmsCodeExpired, "验证码已过期")
		case store.SmsConsumeInvalid:
			return nil, fault(401, result.ReasonSmsCodeInvalid, "验证码错误")
		}
		acc, isNewAcc, err := svc.store.FindOrCreateAccount(ctx, in.LoginAccount, in.ChannelID, in.ChannelSource)
		if err != nil {
			return nil, fault(503, result.ReasonPlatformUnavailable, "账户处理失败")
		}
		accountID, displayName, isNew = acc.PlatformAccountID, acc.DisplayName, isNewAcc
	case "password":
		paID, fp := svc.authenticatePassword(ctx, in)
		if fp != nil {
			return nil, fp
		}
		accountID, displayName, isNew = paID, "", false
	default:
		return nil, fault(400, result.ReasonParamInvalid, "未知 loginMethod")
	}

	token, exp, err := svc.store.CreateSession(ctx, accountID, in.GameID, sessionTTL)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "会话签发失败")
	}
	_, created, err := svc.store.EnsureFirstSubaccount(ctx, accountID, in.GameID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "首个小号建档失败")
	}

	out := &LoginData{
		PlatformAccountID: accountID,
		PlatformToken:     token,
		DisplayName:       displayName,
		ExpiresAt:         exp.UTC().Format(time.RFC3339),
		GameEntry:         GameEntry{IsNewGameUser: isNew},
	}
	if created != nil {
		out.GameEntry.CreatedSubaccount = &CreatedSubaccount{
			Account:           created.Account,
			GameID:            in.GameID,
			PlatformAccountID: accountID,
			DisplayName:       created.DisplayName,
			IsDefault:         false,
		}
	}
	return out, nil
}

// authenticatePassword 校验密码 + 设备信任(#25)。返回 platformAccountId。
func (svc *Service) authenticatePassword(ctx context.Context, in LoginInput) (string, *Fault) {
	paID, hash, _, ok, err := svc.store.FindAccountByLogin(ctx, in.LoginAccount)
	if err != nil {
		return "", fault(503, result.ReasonPlatformUnavailable, "账户读取失败")
	}
	if !ok || hash == "" || !checkPassword(hash, in.Credential) {
		return "", fault(401, result.ReasonCredentialInvalid, "账号或密码错误")
	}
	// 设备信任:未提供 deviceId 时视为已信任(兼容无设备 ID 的调用);提供则按设备判定。
	if in.DeviceID != "" {
		trusted, err := svc.store.IsDeviceTrusted(ctx, paID, in.DeviceID)
		if err != nil {
			return "", fault(503, result.ReasonPlatformUnavailable, "设备校验失败")
		}
		if !trusted {
			if in.DeviceVerifyCode == "" {
				return "", fault(401, result.ReasonDeviceVerificationRequired, "设备需短信验证")
			}
			res, err := svc.store.ConsumeDeviceCode(ctx, in.GameID, in.LoginAccount, in.DeviceVerifyCode)
			if err != nil {
				return "", fault(503, result.ReasonPlatformUnavailable, "设备验证码校验失败")
			}
			if res == store.SmsConsumeExpired {
				return "", fault(401, result.ReasonSmsCodeExpired, "验证码已过期")
			}
			if res == store.SmsConsumeInvalid {
				return "", fault(401, result.ReasonSmsCodeInvalid, "验证码错误")
			}
			if err := svc.store.TrustDevice(ctx, paID, in.DeviceID); err != nil {
				return "", fault(503, result.ReasonPlatformUnavailable, "设备信任写入失败")
			}
		}
	}
	return paID, nil
}

// ---------- 辅助 ----------

func maskPhone(p string) string {
	if len(p) != 11 {
		return "***"
	}
	return p[:3] + "****" + p[7:]
}

func genNumericCode(n int) string {
	const digits = "0123456789"
	out := make([]byte, n)
	for i := range out {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		out[i] = digits[idx.Int64()]
	}
	return string(out)
}

// compareVersion 比较点分版本:a<b 返回 -1,a==b 返回 0,a>b 返回 1。缺位补 0。
func compareVersion(a, b string) int {
	as := splitVer(a)
	bs := splitVer(b)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

func splitVer(v string) []int {
	if v == "" {
		return nil
	}
	var out []int
	cur := 0
	has := false
	for _, r := range v {
		if r >= '0' && r <= '9' {
			cur = cur*10 + int(r-'0')
			has = true
		} else if r == '.' {
			out = append(out, cur)
			cur = 0
			has = false
		}
		// 忽略非数字非点字符
	}
	if has || len(out) > 0 {
		out = append(out, cur)
	}
	return out
}
