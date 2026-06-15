package domain

import (
	"bytes"
	"context"
	"crypto/hmac"
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

	"golang.org/x/crypto/bcrypt"

	"m5755/server/internal/result"
	"m5755/server/internal/store"
)

// ---------- #21 角色上报 ----------

var amountRe = regexp.MustCompile(`^\d+\.\d{2}$`)

type RoleInput struct {
	GameID, Account, Token                            string
	ServerID, ServerName, RoleID, RoleName, RoleLevel string
	RoleCE, RoleStage, RoleRechargeAmount, RoleGuild  string
}

func (svc *Service) ReportRole(ctx context.Context, in RoleInput) (*Fault, map[string]bool) {
	account, _, ok, err := svc.store.SubaccountByToken(ctx, in.Token, in.GameID)
	if err != nil {
		return fault(503, result.ReasonPlatformUnavailable, "鉴权失败"), nil
	}
	if !ok || account != in.Account {
		return fault(401, result.ReasonSubaccountInvalid, "游戏小号登录态无效或归属不符"), nil
	}
	// 05 §1 校验:全字段必填;roleId 拒绝 "-1";金额接受 "-1" 或两位小数。
	if in.ServerID == "" || in.ServerName == "" || in.RoleName == "" || in.RoleLevel == "" {
		return fault(400, result.ReasonParamInvalid, "角色字段缺失"), nil
	}
	if in.RoleID == "" || in.RoleID == "-1" {
		return fault(400, result.ReasonParamInvalid, "roleId 必须为可区分的唯一角色 ID"), nil
	}
	if in.RoleRechargeAmount != "-1" && !amountRe.MatchString(in.RoleRechargeAmount) {
		return fault(400, result.ReasonParamInvalid, "累计充值金额须为 -1 或两位小数"), nil
	}
	err = svc.store.UpsertRole(ctx, store.RoleSnapshot{
		Account: in.Account, GameID: in.GameID, ServerID: in.ServerID, ServerName: in.ServerName,
		RoleID: in.RoleID, RoleName: in.RoleName, RoleLevel: in.RoleLevel,
		RoleCE: dft(in.RoleCE), RoleStage: dft(in.RoleStage), RoleRechargeAmount: dft(in.RoleRechargeAmount), RoleGuild: dft(in.RoleGuild),
	})
	if err != nil {
		return fault(503, result.ReasonPlatformUnavailable, "角色上报失败"), nil
	}
	return nil, map[string]bool{"reported": true}
}

// ---------- #22 支付创建 / 订单查询 ----------

type OrderInput struct {
	GameID, Account, Token                     string
	Amount                                     float64
	CPOrderID, Commodity, ServerID, ServerName string
	RoleID, RoleName, RoleLevel                string
}

type OrderCreateData struct {
	OrderID    string `json:"orderId"`
	PaymentURL string `json:"paymentUrl"`
	Account    string `json:"account"`
	CPOrderID  string `json:"cpOrderId"`
	Amount     string `json:"amount"`
	Commodity  string `json:"commodity"`
	ServerID   string `json:"serverId"`
	ServerName string `json:"serverName"`
}

func (svc *Service) CreateOrder(ctx context.Context, in OrderInput, baseURL string) (*OrderCreateData, *Fault) {
	account, platformAccountID, ok, err := svc.store.SubaccountByToken(ctx, in.Token, in.GameID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "鉴权失败")
	}
	if !ok || account != in.Account {
		return nil, fault(401, result.ReasonSubaccountInvalid, "游戏小号登录态无效或归属不符")
	}
	// 字段校验(05 §2):无演示订单兜底
	if in.Amount <= 0 || in.Amount > 1e9 {
		return nil, fault(400, result.ReasonOrderInvalid, "金额非法")
	}
	if in.CPOrderID == "" || len(in.CPOrderID) > 128 {
		return nil, fault(400, result.ReasonOrderInvalid, "CP 订单号非法")
	}
	if in.Commodity == "" || in.ServerID == "" || in.ServerName == "" || in.RoleID == "" || in.RoleName == "" {
		return nil, fault(400, result.ReasonOrderInvalid, "订单归属字段缺失")
	}
	// 服务端复核合规门禁(D3)
	rn, err := svc.store.GetRealName(ctx, platformAccountID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "门禁判定失败")
	}
	gates, err := svc.realNameGates(ctx, in.GameID, platformAccountID, rn)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "门禁判定失败")
	}
	if !rn.Verified {
		return nil, fault(403, result.ReasonRealNameRequired, "需先完成实名认证")
	}
	if gates.AntiAddictionPaymentBlocked {
		return nil, fault(403, result.ReasonAntiAddictionPayBlocked, "防沉迷支付门禁限制")
	}

	orderID := "P5755" + strconv.FormatInt(svc.now().UnixNano(), 10)
	amount := fmt.Sprintf("%.2f", in.Amount)
	err = svc.store.CreateOrder(ctx, store.Order{
		OrderID: orderID, CPOrderID: in.CPOrderID, Account: account, GameID: in.GameID,
		PlatformAccountID: platformAccountID, Amount: amount, Commodity: in.Commodity,
		ServerID: in.ServerID, ServerName: in.ServerName, RoleID: in.RoleID, RoleName: in.RoleName, RoleLevel: in.RoleLevel,
	})
	if err != nil {
		svc.log().Error("order_create_failed", "cpOrderId", in.CPOrderID,
			"account", maskAccount(account), "gameId", in.GameID, "err", err)
		return nil, fault(503, result.ReasonPlatformUnavailable, "订单创建失败")
	}
	svc.log().Info("order_created", "orderId", orderID, "cpOrderId", in.CPOrderID,
		"account", maskAccount(account), "gameId", in.GameID, "amount", amount)
	return &OrderCreateData{
		OrderID:    orderID,
		PaymentURL: baseURL + "/pay/" + orderID,
		Account:    account, CPOrderID: in.CPOrderID, Amount: amount,
		Commodity: in.Commodity, ServerID: in.ServerID, ServerName: in.ServerName,
	}, nil
}

type OrderQueryData struct {
	PaymentStatus  string `json:"paymentStatus"`
	CallbackStatus string `json:"callbackStatus"`
}

func (svc *Service) QueryOrder(ctx context.Context, gameID, account, token, orderID string) (*OrderQueryData, *Fault) {
	owner, _, ok, err := svc.store.SubaccountByToken(ctx, token, gameID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "鉴权失败")
	}
	if !ok || owner != account {
		return nil, fault(401, result.ReasonSubaccountInvalid, "登录态无效或归属不符")
	}
	o, err := svc.store.GetOrderForGame(ctx, orderID, gameID)
	if err == store.ErrNotFound {
		return nil, fault(404, result.ReasonOrderInvalid, "订单不存在")
	}
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "订单查询失败")
	}
	if o.Account != account {
		return nil, fault(403, result.ReasonOrderInvalid, "订单不归属当前小号")
	}
	return &OrderQueryData{PaymentStatus: o.PaymentStatus, CallbackStatus: o.CallbackStatus}, nil
}

// ---------- #23 充值回调出站 ----------

// CompletePayment 推进订单并触发回调投递(complete-payment 驱动)。mode:成功/失败/超时。
func (svc *Service) CompletePayment(ctx context.Context, gameID, orderID, mode string) *Fault {
	o, err := svc.store.GetOrderForGame(ctx, orderID, gameID)
	if err == store.ErrNotFound {
		return fault(404, result.ReasonOrderInvalid, "订单不存在")
	}
	if err != nil {
		return fault(503, result.ReasonPlatformUnavailable, "订单读取失败")
	}
	if mode == "失败" {
		_ = svc.store.UpdateOrderStatus(ctx, orderID, "支付失败", "未投递")
		svc.log().Info("payment_completed", "orderId", orderID,
			"gameId", gameID, "account", maskAccount(o.Account), "result", "支付失败")
		return nil
	}
	_ = svc.store.UpdateOrderStatus(ctx, orderID, "已支付", "投递中")
	callbackURL, err := svc.store.GetCallbackURL(ctx, gameID)
	if err != nil || callbackURL == "" {
		_ = svc.store.UpdateOrderStatus(ctx, orderID, "已支付", "无回调地址")
		svc.log().Warn("callback_skipped", "orderId", orderID,
			"gameId", gameID, "reason", "无回调地址")
		return nil
	}
	confirmed := svc.dispatchCallback(ctx, callbackURL, o)
	if mode == "超时" {
		// 重复推送语义:再投一次(同笔字段一致)
		confirmed = svc.dispatchCallback(ctx, callbackURL, o) || confirmed
	}
	status := "已确认"
	if !confirmed {
		status = "投递失败"
	}
	_ = svc.store.UpdateOrderStatus(ctx, orderID, "已支付", status)
	svc.log().Info("callback_settled", "orderId", orderID, "gameId", gameID,
		"account", maskAccount(o.Account), "cpOrderId", o.CPOrderID, "callbackStatus", status, "mode", mode)
	return nil
}

// RedeliverPendingCallbacks 平台侧充值回调重投巡检:对"已支付但出站未送达(投递失败/投递中)"的订单重投。
// 这是漏发自愈的关键——渠道确认支付后即 ACK 止重推(渠道不会再推同笔),出站充值回调的最终送达不靠渠道重推,
// 由本巡检补偿;游戏服务端对同笔回调幂等(04 §4),重投安全。返回本轮尝试数与确认数。
func (svc *Service) RedeliverPendingCallbacks(ctx context.Context) (attempted, confirmed int) {
	orders, err := svc.store.ListUndeliveredPaidOrders(ctx, 100)
	if err != nil {
		svc.log().Warn("callback_redeliver_list_failed", "err", err.Error())
		return 0, 0
	}
	for i := range orders {
		o := orders[i]
		callbackURL, err := svc.store.GetCallbackURL(ctx, o.GameID)
		if err != nil || callbackURL == "" {
			continue // 无回调地址无法重投(订单已落"无回调地址"或游戏缺配),非本巡检可补偿
		}
		attempted++
		if svc.dispatchCallback(ctx, callbackURL, &o) {
			_ = svc.store.UpdateOrderStatus(ctx, o.OrderID, "已支付", "已确认")
			confirmed++
			svc.log().Info("callback_redelivered", "orderId", o.OrderID, "gameId", o.GameID)
		}
	}
	if attempted > 0 {
		svc.log().Info("callback_redeliver_sweep", "attempted", attempted, "confirmed", confirmed)
	}
	return attempted, confirmed
}

// RunCallbackRetryLoop 后台定时重投巡检,直至 ctx 取消(进程退出)。
func (svc *Service) RunCallbackRetryLoop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			svc.RedeliverPendingCallbacks(ctx)
		}
	}
}

// devServerKeyID 是 dev 路径的固定 serverKeyId(migration 0010 seed 的 'dev-server-key',
// 其 secret 与 callbackSecret 一致 'm5755-dev-callback-secret-v1')。游戏未配 per-game serverKey 时
// 出站签名回退此默认;已配则据该游戏 active serverKey 记录选 keyId/secret 签。
const devServerKeyID = "dev-server-key"

// dispatchCallback 向游戏服务端推送充值回调,有限重试;游戏侧 {code:200,msg:success} 视为确认。
// 回调体定形(ADR-0016,10 字段全 camelCase):
//
//	account · orderId · cpOrderId · amount · payAmount · commodity · serverId · serverName · serverKeyId · sign
//
// serverKeyId 是非密的密钥标识,进签名串;接收方据它选对应 serverSecret 验签(为优雅轮换留路)。
// payAmount 恒等 amount(v2 无折扣/券/余额,前向兼容缝,ADR-0012)。
func (svc *Service) dispatchCallback(ctx context.Context, url string, o *store.Order) bool {
	serverKeyID, serverSecret := svc.signingServerKey(ctx, o.GameID)
	payload := map[string]string{
		"account": o.Account, "orderId": o.OrderID, "cpOrderId": o.CPOrderID,
		"amount": o.Amount, "payAmount": o.Amount, "commodity": o.Commodity,
		"serverId": o.ServerID, "serverName": o.ServerName, "serverKeyId": serverKeyID,
	}
	// serverKeyId 已在 payload 中,callbackSign 签全字段(除 sign),天然纳入签名串。
	payload["sign"] = callbackSign(payload, serverSecret)
	body, _ := json.Marshal(payload)
	host := callbackHost(url)
	for attempt := 0; attempt < 3; attempt++ {
		ok := svc.postCallback(url, body)
		svc.log().Info("callback_attempt", "orderId", o.OrderID,
			"cpOrderId", o.CPOrderID, "host", host, "attempt", attempt+1, "ok", ok)
		if ok {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// signingServerKey 返回该游戏出站签名(充值回调)用的 (serverKeyId, serverSecret):
// 取该游戏最新 active serverKey(principal='server',ServerKeyForGame);查到则用其 keyId/secret 签,
// 未配 per-game serverKey(或查询失败)则回退 dev 默认(dev-server-key + callbackSecret,二者 secret 一致)。
// 仅出站签名用途;入站验签据 header keyId 走 LookupSigningKey(认任一 active),勿混用。
func (svc *Service) signingServerKey(ctx context.Context, gameID string) (serverKeyID, serverSecret string) {
	if svc.store != nil {
		if keyID, secret, ok, err := svc.store.ServerKeyForGame(ctx, gameID); err == nil && ok {
			return keyID, secret
		}
	}
	return devServerKeyID, svc.callbackSecret
}

// callbackHost 提取回调地址的主机用于日志(不记完整 URL/查询,避免潜在参数入日志)。
func callbackHost(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return "invalid"
}

func (svc *Service) postCallback(url string, body []byte) bool {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != 200 {
		return false
	}
	var ack struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(rb, &ack)
	return ack.Code == 200 && ack.Msg == "success"
}

// callbackSign:对参数(除 sign)按键字典序逐对拼 `键=值&`(含最后一对),
// 以 serverSecret 为密钥对该串做 HMAC-SHA256,hex 小写(04 §4 / ADR-0016)。
// 密钥是 HMAC 参数、不拼进串(旧 MD5 口径末尾的 `key=<密钥>` 已移除)。
func callbackSign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "sign" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(params[k])
		sb.WriteString("&")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sb.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyCallbackSign 供测试接收端复用,验证签名一致。
func VerifyCallbackSign(params map[string]string, secret string) bool {
	return params["sign"] == callbackSign(params, secret)
}

// ---------- 辅助 ----------

func dft(v string) string {
	if v == "" {
		return "-1"
	}
	return v
}

// HashPassword / checkPassword:bcrypt(#25)。
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
