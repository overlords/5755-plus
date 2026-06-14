package domain

import (
	"bytes"
	"context"
	"crypto/md5"
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
	GameID, Account, Token                               string
	ServerID, ServerName, RoleID, RoleName, RoleLevel    string
	RoleCE, RoleStage, RoleRechargeAmount, RoleGuild     string
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
	GameID, Account, Token                            string
	Amount                                            float64
	CPOrderID, Commodity, ServerID, ServerName        string
	RoleID, RoleName, RoleLevel                       string
}

type OrderCreateData struct {
	PlatformOrderID string `json:"platformOrderId"`
	OrderID         string `json:"orderId"`
	PaymentURL      string `json:"paymentUrl"`
	Account         string `json:"account"`
	CPOrderID       string `json:"cpOrderId"`
	Amount          string `json:"amount"`
	Commodity       string `json:"commodity"`
	ServerID        string `json:"serverId"`
	ServerName      string `json:"serverName"`
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

	platformOrderID := "P5755" + strconv.FormatInt(svc.now().UnixNano(), 10)
	amount := fmt.Sprintf("%.2f", in.Amount)
	err = svc.store.CreateOrder(ctx, store.Order{
		PlatformOrderID: platformOrderID, CPOrderID: in.CPOrderID, Account: account, GameID: in.GameID,
		PlatformAccountID: platformAccountID, Amount: amount, Commodity: in.Commodity,
		ServerID: in.ServerID, ServerName: in.ServerName, RoleID: in.RoleID, RoleName: in.RoleName, RoleLevel: in.RoleLevel,
	})
	if err != nil {
		svc.log().Error("order_create_failed", "cpOrderId", in.CPOrderID,
			"account", maskAccount(account), "gameId", in.GameID, "err", err)
		return nil, fault(503, result.ReasonPlatformUnavailable, "订单创建失败")
	}
	svc.log().Info("order_created", "platformOrderId", platformOrderID, "cpOrderId", in.CPOrderID,
		"account", maskAccount(account), "gameId", in.GameID, "amount", amount)
	return &OrderCreateData{
		PlatformOrderID: platformOrderID, OrderID: platformOrderID,
		PaymentURL: baseURL + "/pay/" + platformOrderID,
		Account:    account, CPOrderID: in.CPOrderID, Amount: amount,
		Commodity: in.Commodity, ServerID: in.ServerID, ServerName: in.ServerName,
	}, nil
}

type OrderQueryData struct {
	PaymentStatus  string `json:"paymentStatus"`
	CallbackStatus string `json:"callbackStatus"`
}

func (svc *Service) QueryOrder(ctx context.Context, gameID, account, token, platformOrderID string) (*OrderQueryData, *Fault) {
	owner, _, ok, err := svc.store.SubaccountByToken(ctx, token, gameID)
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "鉴权失败")
	}
	if !ok || owner != account {
		return nil, fault(401, result.ReasonSubaccountInvalid, "登录态无效或归属不符")
	}
	o, err := svc.store.GetOrderForGame(ctx, platformOrderID, gameID)
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
func (svc *Service) CompletePayment(ctx context.Context, gameID, platformOrderID, mode string) *Fault {
	o, err := svc.store.GetOrderForGame(ctx, platformOrderID, gameID)
	if err == store.ErrNotFound {
		return fault(404, result.ReasonOrderInvalid, "订单不存在")
	}
	if err != nil {
		return fault(503, result.ReasonPlatformUnavailable, "订单读取失败")
	}
	if mode == "失败" {
		_ = svc.store.UpdateOrderStatus(ctx, platformOrderID, "支付失败", "未投递")
		svc.log().Info("payment_completed", "platformOrderId", platformOrderID,
			"gameId", gameID, "account", maskAccount(o.Account), "result", "支付失败")
		return nil
	}
	_ = svc.store.UpdateOrderStatus(ctx, platformOrderID, "已支付", "投递中")
	callbackURL, err := svc.store.GetCallbackURL(ctx, gameID)
	if err != nil || callbackURL == "" {
		_ = svc.store.UpdateOrderStatus(ctx, platformOrderID, "已支付", "无回调地址")
		svc.log().Warn("callback_skipped", "platformOrderId", platformOrderID,
			"gameId", gameID, "reason", "无回调地址")
		return nil
	}
	confirmed := svc.dispatchCallback(callbackURL, o)
	if mode == "超时" {
		// 重复推送语义:再投一次(同笔字段一致)
		confirmed = svc.dispatchCallback(callbackURL, o) || confirmed
	}
	status := "已确认"
	if !confirmed {
		status = "投递失败"
	}
	_ = svc.store.UpdateOrderStatus(ctx, platformOrderID, "已支付", status)
	svc.log().Info("callback_settled", "platformOrderId", platformOrderID, "gameId", gameID,
		"account", maskAccount(o.Account), "cpOrderId", o.CPOrderID, "callbackStatus", status, "mode", mode)
	return nil
}

// dispatchCallback 向游戏服务端推送充值回调,有限重试;游戏侧 {code:200,msg:success} 视为确认。
func (svc *Service) dispatchCallback(url string, o *store.Order) bool {
	payload := map[string]string{
		"account": o.Account, "platformOrderId": o.PlatformOrderID, "cpOrderId": o.CPOrderID,
		"amount": o.Amount, "commodity": o.Commodity, "serverId": o.ServerID, "serverName": o.ServerName,
		"order_id": o.PlatformOrderID, "cp_order_id": o.CPOrderID, "money": o.Amount, "pay_money": o.Amount,
	}
	payload["sign"] = callbackSign(payload, svc.callbackSecret)
	body, _ := json.Marshal(payload)
	host := callbackHost(url)
	for attempt := 0; attempt < 3; attempt++ {
		ok := svc.postCallback(url, body)
		svc.log().Info("callback_attempt", "platformOrderId", o.PlatformOrderID,
			"cpOrderId", o.CPOrderID, "host", host, "attempt", attempt+1, "ok", ok)
		if ok {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
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

// callbackSign:对参数(除 sign)按键字典序拼接 + 密钥,MD5(平台回调签名规则)。
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
	sb.WriteString("key=")
	sb.WriteString(secret)
	sum := md5.Sum([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
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
