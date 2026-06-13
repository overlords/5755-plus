package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"time"

	"m5755/server/internal/nppa"
	"m5755/server/internal/result"
	"m5755/server/internal/store"
)

// ---------- #11 账户有效检查 ----------

type AccountCheckData struct {
	Valid             bool   `json:"valid"`
	PlatformAccountID string `json:"platformAccountId,omitempty"`
	DisplayName       string `json:"displayName,omitempty"`
}

// CheckAccountSession 语义分层(04 §2.3.2):接口成功 success:true,有效性看 data.valid;
// 明确失效时 data.valid=false + reason=platform_account_invalid(handler 写入信封)。
func (svc *Service) CheckAccountSession(ctx context.Context, gameID, platformAccountID, platformToken string) (*AccountCheckData, *Fault) {
	if gameID == "" || platformAccountID == "" || platformToken == "" {
		return nil, fault(400, result.ReasonParamInvalid, "缺少 gameId/platformAccountId/凭据")
	}
	owner, valid, err := svc.store.ValidateAccountSession(ctx, platformToken, platformAccountID, gameID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "会话校验失败")
	}
	if !valid {
		return &AccountCheckData{Valid: false}, nil
	}
	return &AccountCheckData{Valid: true, PlatformAccountID: owner.PlatformAccountID, DisplayName: owner.DisplayName}, nil
}

// requirePlatformSession 实名/小号类端点的统一前置:主账户会话必须有效,失效即 platform_account_invalid。
func (svc *Service) requirePlatformSession(ctx context.Context, gameID, platformAccountID, platformToken string) *Fault {
	if gameID == "" || platformAccountID == "" || platformToken == "" {
		return fault(400, result.ReasonParamInvalid, "缺少 gameId/platformAccountId/凭据")
	}
	_, valid, err := svc.store.ValidateAccountSession(ctx, platformToken, platformAccountID, gameID)
	if err != nil {
		return fault(503, result.ReasonPlatformUnavailable, "会话校验失败")
	}
	if !valid {
		return fault(401, result.ReasonPlatformAccountInvalid, "5755 账户登录态已失效")
	}
	return nil
}

// ---------- #12 实名 ----------

var idNumberRe = regexp.MustCompile(`^\d{17}[\dXx]$`)

type RealNameData struct {
	Verified                    bool `json:"verified"`
	Adult                       bool `json:"adult"`
	Pending                     bool `json:"pending,omitempty"` // NPPA 认证中(异步,等查询接口出结果)
	AntiAddictionEntryBlocked   bool `json:"antiAddictionEntryBlocked"`
	AntiAddictionPaymentBlocked bool `json:"antiAddictionPaymentBlocked"`
}

// realNameGates 门禁最小推导(04 §2.4):未实名→阻进入+支付;已实名未成年→放行进入阻支付;dev 注入可覆盖。
// "认证中"按未实名对待(verified=false → 阻进入+支付),Pending 仅作信息透出。
func (svc *Service) realNameGates(ctx context.Context, gameID, platformAccountID string, st *store.RealNameState) (*RealNameData, error) {
	d := &RealNameData{Verified: st.Verified, Adult: st.Adult, Pending: st.Pending}
	if !st.Verified {
		d.AntiAddictionEntryBlocked = true
		d.AntiAddictionPaymentBlocked = true
	} else if !st.Adult {
		d.AntiAddictionPaymentBlocked = true
	}
	inj, err := svc.store.GetAccountInjection(ctx, gameID, platformAccountID)
	if err != nil {
		return nil, err
	}
	if inj.EntryBlocked != nil {
		d.AntiAddictionEntryBlocked = *inj.EntryBlocked
	}
	if inj.PaymentBlocked != nil {
		d.AntiAddictionPaymentBlocked = *inj.PaymentBlocked
	}
	return d, nil
}

func (svc *Service) GetRealName(ctx context.Context, gameID, platformAccountID, platformToken string) (*RealNameData, *Fault) {
	if f := svc.requirePlatformSession(ctx, gameID, platformAccountID, platformToken); f != nil {
		return nil, f
	}
	st, err := svc.store.GetRealName(ctx, platformAccountID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "实名状态读取失败")
	}
	// 懒查询(a):异步"认证中"的账户,本次读取顺带向 NPPA 查询接口拿结果并定案。
	if st.Pending && !svc.realNameMock {
		st = svc.resolvePending(ctx, gameID, platformAccountID, st)
	}
	d, err := svc.realNameGates(ctx, gameID, platformAccountID, st)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "门禁判定失败")
	}
	return d, nil
}

// resolvePending 经 NPPA 查询接口尝试为"认证中"账户定案;查询失败则保持 pending 留待下次。
func (svc *Service) resolvePending(ctx context.Context, gameID, platformAccountID string, st *store.RealNameState) *store.RealNameState {
	creds, err := svc.store.GetGameNppaCreds(ctx, gameID)
	if err != nil {
		return st
	}
	nc := nppa.Creds{AppID: creds.AppID, BizID: creds.BizID, SecretKey: creds.SecretKey}
	if !nc.Ready() {
		return st
	}
	res, qerr := nppa.Query(ctx, svc.httpClient, nc, svc.nppaQueryURL, st.AI, svc.now())
	if qerr != nil {
		return st
	}
	switch res.Status {
	case nppa.StatusSuccess:
		_ = svc.store.MarkRealNameVerified(ctx, platformAccountID, res.PI)
	case nppa.StatusFailed:
		_ = svc.store.MarkRealNameFailed(ctx, platformAccountID)
	default: // 仍认证中
		return st
	}
	if reloaded, rerr := svc.store.GetRealName(ctx, platformAccountID); rerr == nil {
		return reloaded
	}
	return st
}

// SubmitRealName dev=mock(格式 + 出生日期);生产=NPPA 真认证(per-game 凭据,ADR-0007),
// 未配置凭据 fail-closed;异步"认证中"按(a)懒查询定案。
func (svc *Service) SubmitRealName(ctx context.Context, gameID, platformAccountID, platformToken, realName, idNumber string) (*RealNameData, *Fault) {
	if f := svc.requirePlatformSession(ctx, gameID, platformAccountID, platformToken); f != nil {
		return nil, f
	}
	if realName == "" || !idNumberRe.MatchString(idNumber) {
		return nil, fault(400, result.ReasonParamInvalid, "姓名或身份证号格式非法")
	}
	birth, err := time.Parse("20060102", idNumber[6:14])
	if err != nil || birth.After(svc.now()) {
		return nil, fault(400, result.ReasonParamInvalid, "身份证出生日期非法")
	}
	adult := age(birth, svc.now()) >= 18

	// 已实名锁定:幂等返回当前态,不重复核验。
	if cur, gerr := svc.store.GetRealName(ctx, platformAccountID); gerr == nil && cur.Verified {
		return svc.gatesOrFault(ctx, gameID, platformAccountID, cur)
	}

	if svc.realNameMock {
		st, serr := svc.store.SubmitRealName(ctx, platformAccountID, maskName(realName), maskID(idNumber), adult)
		if serr != nil {
			return nil, fault(503, result.ReasonPlatformUnavailable, "实名提交失败")
		}
		return svc.gatesOrFault(ctx, gameID, platformAccountID, st)
	}

	// 生产 NPPA 路径
	creds, cerr := svc.store.GetGameNppaCreds(ctx, gameID)
	if cerr != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "实名核验服务未配置")
	}
	nc := nppa.Creds{AppID: creds.AppID, BizID: creds.BizID, SecretKey: creds.SecretKey}
	if !nc.Ready() {
		// 该游戏未提供 NPPA 凭据(接入者未授权)→ fail-closed。
		return nil, fault(503, result.ReasonPlatformUnavailable, "实名核验服务未配置(该游戏未提供 NPPA 凭据)")
	}
	ai := genHexID()
	if serr := svc.store.StageRealName(ctx, platformAccountID, ai, maskName(realName), maskID(idNumber), adult); serr != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "实名提交失败")
	}
	res, nerr := nppa.Check(ctx, svc.httpClient, nc, svc.nppaCheckURL, ai, realName, idNumber, svc.now())
	if nerr != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "实名核验失败")
	}
	switch res.Status {
	case nppa.StatusSuccess:
		if merr := svc.store.MarkRealNameVerified(ctx, platformAccountID, res.PI); merr != nil {
			return nil, fault(503, result.ReasonPlatformUnavailable, "实名提交失败")
		}
		return svc.gatesOrFault(ctx, gameID, platformAccountID, &store.RealNameState{Verified: true, Adult: adult, PI: res.PI})
	case nppa.StatusFailed:
		_ = svc.store.MarkRealNameFailed(ctx, platformAccountID)
		return nil, fault(400, result.ReasonParamInvalid, "实名认证未通过(姓名与身份证不匹配)")
	default: // StatusPending 认证中
		_ = svc.store.MarkRealNamePending(ctx, platformAccountID)
		return svc.gatesOrFault(ctx, gameID, platformAccountID, &store.RealNameState{Pending: true, Adult: adult})
	}
}

func (svc *Service) gatesOrFault(ctx context.Context, gameID, platformAccountID string, st *store.RealNameState) (*RealNameData, *Fault) {
	d, err := svc.realNameGates(ctx, gameID, platformAccountID, st)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "门禁判定失败")
	}
	return d, nil
}

// genHexID 生成 32 位 hex(NPPA ai:游戏内唯一标识,每次提交不可复用)。
func genHexID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- #13 小号 ----------

const subaccountLimit = 10

type SubaccountItem struct {
	Account     string `json:"account"`
	DisplayName string `json:"displayName"`
	IsDefault   bool   `json:"isDefault"`
}

type SubaccountListData struct {
	DefaultAccount string           `json:"defaultAccount"`
	Subaccounts    []SubaccountItem `json:"subaccounts"`
}

func (svc *Service) ListSubaccounts(ctx context.Context, gameID, platformAccountID, platformToken string) (*SubaccountListData, *Fault) {
	if f := svc.requirePlatformSession(ctx, gameID, platformAccountID, platformToken); f != nil {
		return nil, f
	}
	subs, err := svc.store.ListSubaccounts(ctx, platformAccountID, gameID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "小号列表读取失败")
	}
	out := &SubaccountListData{Subaccounts: make([]SubaccountItem, 0, len(subs))}
	for _, s := range subs {
		if s.IsDefault {
			out.DefaultAccount = s.Account
		}
		out.Subaccounts = append(out.Subaccounts, SubaccountItem{Account: s.Account, DisplayName: s.DisplayName, IsDefault: s.IsDefault})
	}
	return out, nil
}

func (svc *Service) CreateSubaccount(ctx context.Context, gameID, platformAccountID, platformToken string) (*SubaccountItem, *Fault) {
	if f := svc.requirePlatformSession(ctx, gameID, platformAccountID, platformToken); f != nil {
		return nil, f
	}
	n, err := svc.store.CountSubaccounts(ctx, platformAccountID, gameID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "小号数量读取失败")
	}
	if n >= subaccountLimit {
		return nil, fault(409, result.ReasonSubaccountLimitReached, "已达小号上限")
	}
	sub, err := svc.store.CreateSubaccount(ctx, platformAccountID, gameID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "小号创建失败")
	}
	return &SubaccountItem{Account: sub.Account, DisplayName: sub.DisplayName, IsDefault: false}, nil
}

type SetDefaultData struct {
	Account        string `json:"account"`
	DefaultAccount bool   `json:"defaultAccount"`
}

func (svc *Service) SetDefaultSubaccount(ctx context.Context, gameID, platformAccountID, platformToken, account string) (*SetDefaultData, *Fault) {
	if f := svc.requirePlatformSession(ctx, gameID, platformAccountID, platformToken); f != nil {
		return nil, f
	}
	if account == "" {
		return nil, fault(400, result.ReasonParamInvalid, "缺少 account")
	}
	err := svc.store.SetDefaultSubaccount(ctx, platformAccountID, gameID, account)
	if err == store.ErrNotFound {
		return nil, fault(404, result.ReasonSubaccountInvalid, "游戏小号不存在或已失效")
	}
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "设默认失败")
	}
	return &SetDefaultData{Account: account, DefaultAccount: true}, nil
}

// ---------- #14 小号会话 ----------

type SubaccountLoginData struct {
	Account   string `json:"account"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

// LoginSubaccount 失效分流硬规则(03 §3):主账户失效→platform_account_invalid;
// 小号不存在/停用/归属不符→subaccount_invalid;两者绝不混用。account 必填禁省略。
func (svc *Service) LoginSubaccount(ctx context.Context, gameID, platformAccountID, platformToken, account string) (*SubaccountLoginData, *Fault) {
	if account == "" {
		return nil, fault(400, result.ReasonParamInvalid, "缺少 account(不存在省略 account 的默认登录)")
	}
	if f := svc.requirePlatformSession(ctx, gameID, platformAccountID, platformToken); f != nil {
		return nil, f
	}
	sub, owner, active, err := svc.store.GetSubaccount(ctx, account)
	if err == store.ErrNotFound {
		return nil, fault(404, result.ReasonSubaccountInvalid, "游戏小号不存在")
	}
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "小号读取失败")
	}
	if !active || owner != platformAccountID || sub.GameID != gameID {
		return nil, fault(403, result.ReasonSubaccountInvalid, "游戏小号已失效或归属不符")
	}
	token, exp, err := svc.store.CreateSubaccountSession(ctx, account, platformAccountID, gameID, sessionTTL)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "小号会话签发失败")
	}
	return &SubaccountLoginData{Account: account, Token: token, ExpiresAt: exp.UTC().Format(time.RFC3339)}, nil
}

type SubaccountCheckData struct {
	Valid   bool   `json:"valid"`
	Account string `json:"account,omitempty"`
}

func (svc *Service) CheckSubaccountSession(ctx context.Context, gameID, account, token string) (*SubaccountCheckData, *Fault) {
	if gameID == "" || account == "" || token == "" {
		return nil, fault(400, result.ReasonParamInvalid, "缺少 gameId/account/凭据")
	}
	valid, err := svc.store.ValidateSubaccountSession(ctx, token, account, gameID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "登录态校验失败")
	}
	if !valid {
		return &SubaccountCheckData{Valid: false}, nil
	}
	return &SubaccountCheckData{Valid: true, Account: account}, nil
}

// ---------- 辅助 ----------

func age(birth, now time.Time) int {
	a := now.Year() - birth.Year()
	if now.YearDay() < birth.YearDay() {
		a--
	}
	return a
}

func maskName(name string) string {
	r := []rune(name)
	if len(r) <= 1 {
		return "*"
	}
	return string(r[0]) + "**"
}

func maskID(id string) string {
	if len(id) < 8 {
		return "****"
	}
	return id[:4] + "**********" + id[len(id)-4:]
}
