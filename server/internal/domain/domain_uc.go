package domain

import (
	"context"

	"github.com/jackc/pgx/v5"

	"m5755/server/internal/result"
	"m5755/server/internal/store"
)

// 用户中心面(/api/uc/v2,ADR-0010)的 domain 逻辑:平台对玩家、以主账户为核心。
// 鉴权仅凭 platformToken(SPA 不持 accountId/gameId),失效统一 platform_account_invalid。

// UCSubAccount 是用户中心回显的当前游戏小号。
type UCSubAccount struct {
	Account string `json:"account"`
	Label   string `json:"label"`
}

// UCProfileData 是 GET /api/uc/v2/profile 的响应数据(06a §3)。
type UCProfileData struct {
	Nickname          string        `json:"nickname"`
	MaskedPhone       string        `json:"maskedPhone"`
	AvatarURL         string        `json:"avatarUrl,omitempty"`
	RealNameStatus    string        `json:"realNameStatus"` // verified | unverified
	CurrentSubAccount *UCSubAccount `json:"currentSubAccount,omitempty"`
}

// GetUserCenterProfile 凭 platformToken 返回主账户身份 + 实名状态 + 当前小号。
// 任一失效路径返回 401 + platform_account_invalid,SPA 据此 postAccountAction("session_invalid")。
func (svc *Service) GetUserCenterProfile(ctx context.Context, platformToken string) (*UCProfileData, *Fault) {
	if platformToken == "" {
		return nil, fault(401, result.ReasonPlatformAccountInvalid, "缺少登录令牌")
	}
	sess, valid, err := svc.store.LookupAccountSessionByToken(ctx, platformToken)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "平台服务暂不可用")
	}
	if !valid {
		return nil, fault(401, result.ReasonPlatformAccountInvalid, "主账户登录态已失效")
	}

	acc, err := svc.store.GetPlatformAccount(ctx, sess.PlatformAccountID)
	if err == pgx.ErrNoRows {
		return nil, fault(401, result.ReasonPlatformAccountInvalid, "主账户不存在")
	}
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "平台服务暂不可用")
	}

	nickname := acc.DisplayName
	if nickname == "" {
		nickname = "5755玩家" // 缺省口径同 07 §5
	}
	status := "unverified"
	if acc.RealNameVerified {
		status = "verified"
	}
	data := &UCProfileData{
		Nickname:       nickname,
		MaskedPhone:    maskPhone(acc.LoginAccount),
		RealNameStatus: status,
	}

	if sub, ok, serr := svc.store.CurrentSubaccount(ctx, sess.PlatformAccountID, sess.GameID); serr == nil && ok {
		data.CurrentSubAccount = &UCSubAccount{Account: sub.Account, Label: sub.Label}
	}
	return data, nil
}

// UCOrderItem 是用户中心订单列表行(06a §3:真实货币,不出现平台币/代金券)。
type UCOrderItem struct {
	OrderID     string `json:"orderId"`
	ProductName string `json:"productName"`
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	CreatedAt   string `json:"createdAt"`
	Status      string `json:"status"`
}

// UCOrdersData 是 GET /api/uc/v2/orders 的响应数据(游标分页,06a §3)。
type UCOrdersData struct {
	Orders     []UCOrderItem `json:"orders"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

// GetUserCenterOrders 凭 platformToken 返回主账户充值订单(游标分页)。金额一律真实货币 CNY。
func (svc *Service) GetUserCenterOrders(ctx context.Context, platformToken, cursor string) (*UCOrdersData, *Fault) {
	if platformToken == "" {
		return nil, fault(401, result.ReasonPlatformAccountInvalid, "缺少登录令牌")
	}
	sess, valid, err := svc.store.LookupAccountSessionByToken(ctx, platformToken)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "平台服务暂不可用")
	}
	if !valid {
		return nil, fault(401, result.ReasonPlatformAccountInvalid, "主账户登录态已失效")
	}
	rows, next, err := svc.store.ListOrders(ctx, sess.PlatformAccountID, cursor, 20)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "平台服务暂不可用")
	}
	data := &UCOrdersData{Orders: []UCOrderItem{}, NextCursor: next}
	for _, o := range rows {
		data.Orders = append(data.Orders, UCOrderItem{
			OrderID:     o.OrderID,
			ProductName: o.ProductName,
			Amount:      o.Amount,
			Currency:    "CNY",
			CreatedAt:   o.CreatedAt.Format("2006-01-02 15:04"),
			Status:      o.Status,
		})
	}
	return data, nil
}

// ucResolve 凭 platformToken 反解主账户会话(账户/游戏 ID);失效统一 401 platform_account_invalid。
func (svc *Service) ucResolve(ctx context.Context, token string) (platformAccountID, gameID string, f *Fault) {
	if token == "" {
		return "", "", fault(401, result.ReasonPlatformAccountInvalid, "缺少登录令牌")
	}
	sess, valid, err := svc.store.LookupAccountSessionByToken(ctx, token)
	if err != nil {
		return "", "", fault(503, result.ReasonPlatformUnavailable, "平台服务暂不可用")
	}
	if !valid {
		return "", "", fault(401, result.ReasonPlatformAccountInvalid, "主账户登录态已失效")
	}
	return sess.PlatformAccountID, sess.GameID, nil
}

// consumeOrFault 把 SMS 消费结果转成 Fault(OK→nil)。
func consumeOrFault(consume store.SmsConsume, err error) *Fault {
	switch {
	case err != nil:
		return fault(503, result.ReasonPlatformUnavailable, "平台服务暂不可用")
	case consume == store.SmsConsumeExpired:
		return fault(400, result.ReasonParamInvalid, "验证码已过期")
	case consume != store.SmsConsumeOK:
		return fault(400, result.ReasonParamInvalid, "验证码错误")
	}
	return nil
}

// UCSendPhoneSms 换绑:向新手机号发验证码(复用 RequestSmsCode:11 位校验 + 限流 + dev devCode)。
func (svc *Service) UCSendPhoneSms(ctx context.Context, token, newPhone string) (*SmsData, *Fault) {
	_, gameID, f := svc.ucResolve(ctx, token)
	if f != nil {
		return nil, f
	}
	return svc.RequestSmsCode(ctx, gameID, newPhone)
}

// UCRebindPhone 提交换绑:验码 + 改 login_account;成功不登出(06a §49)。号被占用 → 409。
func (svc *Service) UCRebindPhone(ctx context.Context, token, newPhone, smsCode string) *Fault {
	paID, gameID, f := svc.ucResolve(ctx, token)
	if f != nil {
		return f
	}
	if !phoneRe.MatchString(newPhone) {
		return fault(400, result.ReasonParamInvalid, "手机号格式不正确")
	}
	if f := consumeOrFault(svc.store.ConsumeSmsCode(ctx, gameID, newPhone, smsCode)); f != nil {
		return f
	}
	ok, err := svc.store.UpdateLoginAccount(ctx, paID, newPhone)
	if err != nil {
		return fault(503, result.ReasonPlatformUnavailable, "平台服务暂不可用")
	}
	if !ok {
		return fault(409, result.ReasonParamInvalid, "该手机号已被占用")
	}
	return nil
}

// UCSendPasswordSms 改密:向**已绑手机**发验证码(短信验证身份)。
func (svc *Service) UCSendPasswordSms(ctx context.Context, token string) (*SmsData, *Fault) {
	paID, gameID, f := svc.ucResolve(ctx, token)
	if f != nil {
		return nil, f
	}
	acc, err := svc.store.GetPlatformAccount(ctx, paID)
	if err != nil {
		return nil, fault(401, result.ReasonPlatformAccountInvalid, "主账户不存在")
	}
	return svc.RequestSmsCode(ctx, gameID, acc.LoginAccount)
}

// UCChangePassword 提交改密:验码 + bcrypt 改密 + 作废全部会话(强制处处重登,06a §48)。
func (svc *Service) UCChangePassword(ctx context.Context, token, smsCode, newPassword string) *Fault {
	paID, gameID, f := svc.ucResolve(ctx, token)
	if f != nil {
		return f
	}
	if len(newPassword) < 6 || len(newPassword) > 64 {
		return fault(400, result.ReasonParamInvalid, "密码长度须 6-64")
	}
	acc, err := svc.store.GetPlatformAccount(ctx, paID)
	if err != nil {
		return fault(401, result.ReasonPlatformAccountInvalid, "主账户不存在")
	}
	if f := consumeOrFault(svc.store.ConsumeSmsCode(ctx, gameID, acc.LoginAccount, smsCode)); f != nil {
		return f
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return fault(500, result.ReasonPlatformUnavailable, "密码处理失败")
	}
	if err := svc.store.UpdatePasswordHash(ctx, paID, hash); err != nil {
		return fault(503, result.ReasonPlatformUnavailable, "平台服务暂不可用")
	}
	if err := svc.store.RevokeAllAccountSessions(ctx, paID); err != nil {
		return fault(503, result.ReasonPlatformUnavailable, "平台服务暂不可用")
	}
	return nil
}
