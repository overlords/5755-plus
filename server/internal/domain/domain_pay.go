package domain

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"m5755/server/internal/paychannel"
	"m5755/server/internal/result"
	"m5755/server/internal/store"
)

// ---------- #60 入站支付:收银台 + 预下单 + 渠道回调接收端 ----------

// PaymentChannels 持有可用渠道签名器;nil 表示该渠道未配置(预下单/验签 fail-closed)。
type PaymentChannels struct {
	Wechat *paychannel.WechatSigner
	Alipay *paychannel.AlipaySigner
}

// WechatEnabled / AlipayEnabled 供收银台页决定展示哪些支付方式。
func (svc *Service) WechatEnabled() bool {
	return svc.channels.Wechat != nil
}

func (svc *Service) AlipayEnabled() bool {
	return svc.channels.Alipay != nil
}

// CashierOrder 收银台渲染所需的订单视图(只暴露展示必需字段,不含归属/账户敏感面)。
type CashierOrder struct {
	PlatformOrderID string
	Amount          string // 元,两位小数
	Commodity       string
	PaymentStatus   string
	WechatEnabled   bool
	AlipayEnabled   bool
}

// GetCashierOrder 读订单供收银台渲染。订单不存在返回 Fault。
func (svc *Service) GetCashierOrder(ctx context.Context, platformOrderID string) (*CashierOrder, *Fault) {
	o, err := svc.store.GetOrder(ctx, platformOrderID)
	if err == store.ErrNotFound {
		return nil, fault(404, result.ReasonOrderInvalid, "订单不存在")
	}
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "订单读取失败")
	}
	return &CashierOrder{
		PlatformOrderID: o.PlatformOrderID,
		Amount:          o.Amount,
		Commodity:       o.Commodity,
		PaymentStatus:   o.PaymentStatus,
		WechatEnabled:   svc.WechatEnabled(),
		AlipayEnabled:   svc.AlipayEnabled(),
	}, nil
}

// PrepayResult 预下单结果:Kind 区分 SDK/收银台如何拉起。
//   - "url":直接把 RedirectURL 加载进 WebView(支付宝 wap / 微信 H5)。
//   - "jsapi":返回 JSAPIPayParams(收银台内 WeixinJSBridge 调起),需 openid。
type PrepayResult struct {
	Kind           string
	RedirectURL    string
	JSAPIPayParams map[string]string
}

// BeginPayment 按收银台所选方式预下单(A2)。method ∈ {wechat, alipay}。
// 反欺诈与发放走渠道回调,本步只换取拉起参数;失败 fail-closed(渠道未配置即拒绝)。
func (svc *Service) BeginPayment(ctx context.Context, platformOrderID, method, payerIP string) (*PrepayResult, *Fault) {
	o, err := svc.store.GetOrder(ctx, platformOrderID)
	if err == store.ErrNotFound {
		return nil, fault(404, result.ReasonOrderInvalid, "订单不存在")
	}
	if err != nil {
		return nil, fault(503, result.ReasonPlatformUnavailable, "订单读取失败")
	}
	if o.PaymentStatus != "待支付" {
		return nil, fault(409, result.ReasonOrderInvalid, "订单已非待支付状态")
	}
	fen, ferr := yuanStringToFen(o.Amount)
	if ferr != nil {
		return nil, fault(500, result.ReasonPlatformUnavailable, "订单金额异常")
	}

	switch method {
	case "wechat":
		if svc.channels.Wechat == nil {
			return nil, fault(503, result.ReasonPlatformUnavailable, "微信支付未配置")
		}
		// H5 预下单(收银台为平台 H5,玩家在 WebView 内);拿 prepay 后由收银台拉起。
		body, berr := svc.channels.Wechat.BuildH5PrepayBody(paychannel.WechatPrepayInput{
			OutTradeNo: o.PlatformOrderID, Description: o.Commodity, TotalFen: fen, PayerIP: payerIP,
		})
		if berr != nil {
			return nil, fault(400, result.ReasonOrderInvalid, "微信预下单参数非法")
		}
		// 实际 HTTP 调用微信 /v3/pay/transactions/h5 需真实商户资质(见 #60 业务前置);
		// 此处把签名头与请求体备好,留待资质就绪后接线真实出网调用。
		_ = body
		if err := svc.store.SetOrderPaymentMethod(ctx, o.PlatformOrderID, "wechat"); err != nil {
			svc.log().Warn("set_payment_method_failed", "platformOrderId", o.PlatformOrderID, "method", "wechat", "err", err.Error())
		}
		return nil, fault(503, result.ReasonPlatformUnavailable, "微信支付待商户资质接线")
	case "alipay":
		if svc.channels.Alipay == nil {
			return nil, fault(503, result.ReasonPlatformUnavailable, "支付宝未配置")
		}
		payURL, aerr := svc.channels.Alipay.BuildWapPayURL(paychannel.AlipayWapInput{
			OutTradeNo: o.PlatformOrderID, Subject: o.Commodity, TotalAmount: o.Amount,
		})
		if aerr != nil {
			return nil, fault(400, result.ReasonOrderInvalid, "支付宝预下单参数非法")
		}
		if err := svc.store.SetOrderPaymentMethod(ctx, o.PlatformOrderID, "alipay"); err != nil {
			svc.log().Warn("set_payment_method_failed", "platformOrderId", o.PlatformOrderID, "method", "alipay", "err", err.Error())
		}
		return &PrepayResult{Kind: "url", RedirectURL: payURL}, nil
	default:
		return nil, fault(400, result.ReasonParamInvalid, "未知支付方式")
	}
}

// ---------- 渠道回调接收端编排(B) ----------

// notifyAck 渠道要求的 ACK 体;handler 直接回写。
type NotifyOutcome struct {
	OK      bool   // true=已正确受理(回成功 ACK 止重推);false=拒绝(回失败,渠道可重推)
	Message string // 诊断
}

// HandleWechatNotify 处理微信 APIv3 回调:验签 → 解密 → 反欺诈 → 幂等 → CompletePayment。
// rawBody 为原始请求体;headers 为 Wechatpay-Timestamp/Nonce/Signature。
func (svc *Service) HandleWechatNotify(ctx context.Context, rawBody []byte, timestamp, nonce, signature string) NotifyOutcome {
	if svc.channels.Wechat == nil {
		svc.log().Warn("wxnotify_unconfigured")
		return NotifyOutcome{OK: false, Message: "微信支付未配置"}
	}
	body := string(rawBody)
	if err := svc.channels.Wechat.VerifyNotifySignature(timestamp, nonce, body, signature); err != nil {
		svc.log().Warn("wxnotify_sign_invalid", "err", err.Error())
		return NotifyOutcome{OK: false, Message: "验签失败"}
	}
	env, err := paychannel.ParseWechatEnvelope(rawBody)
	if err != nil {
		svc.log().Warn("wxnotify_envelope_invalid", "err", err.Error())
		return NotifyOutcome{OK: false, Message: "信封非法"}
	}
	txn, err := svc.channels.Wechat.DecryptNotifyResource(env)
	if err != nil {
		svc.log().Warn("wxnotify_decrypt_failed", "err", err.Error())
		return NotifyOutcome{OK: false, Message: "资源解密失败"}
	}
	if txn.TradeState != "SUCCESS" {
		// 非成功态:受理(回成功止重推)但不发放。
		svc.log().Info("wxnotify_non_success", "platformOrderId", txn.OutTradeNo, "tradeState", txn.TradeState)
		return NotifyOutcome{OK: true, Message: "非成功态,不发放"}
	}
	expectFen, derr := svc.expectedOrderFen(ctx, txn.OutTradeNo)
	if derr != nil {
		return svc.notifyOrderFault(ctx, "wechat", txn.OutTradeNo, derr)
	}
	if txn.Amount.Currency != "CNY" || txn.Amount.Total != expectFen {
		svc.log().Warn("wxnotify_amount_mismatch", "platformOrderId", txn.OutTradeNo,
			"channelFen", txn.Amount.Total, "channelCurrency", txn.Amount.Currency, "expectFen", expectFen)
		return NotifyOutcome{OK: false, Message: "金额/币种不符"}
	}
	return svc.settleChannelNotify(ctx, "wechat", txn.OutTradeNo, txn.TransactionID)
}

// HandleAlipayNotify 处理支付宝异步通知:验签 → 反欺诈 → 幂等 → CompletePayment。
// params 为 POST form 解析出的键值(含 sign/sign_type)。
func (svc *Service) HandleAlipayNotify(ctx context.Context, params map[string]string) NotifyOutcome {
	if svc.channels.Alipay == nil {
		svc.log().Warn("alinotify_unconfigured")
		return NotifyOutcome{OK: false, Message: "支付宝未配置"}
	}
	if err := svc.channels.Alipay.VerifyNotifySign(params); err != nil {
		svc.log().Warn("alinotify_sign_invalid", "err", err.Error())
		return NotifyOutcome{OK: false, Message: "验签失败"}
	}
	outTradeNo := params["out_trade_no"]
	tradeStatus := params["trade_status"]
	if tradeStatus != "TRADE_SUCCESS" && tradeStatus != "TRADE_FINISHED" {
		svc.log().Info("alinotify_non_success", "platformOrderId", outTradeNo, "tradeStatus", tradeStatus)
		return NotifyOutcome{OK: true, Message: "非成功态,不发放"}
	}
	expectFen, derr := svc.expectedOrderFen(ctx, outTradeNo)
	if derr != nil {
		return svc.notifyOrderFault(ctx, "alipay", outTradeNo, derr)
	}
	gotFen, perr := yuanStringToFen(params["total_amount"])
	if perr != nil || gotFen != expectFen {
		svc.log().Warn("alinotify_amount_mismatch", "platformOrderId", outTradeNo,
			"channelAmount", params["total_amount"], "expectFen", expectFen)
		return NotifyOutcome{OK: false, Message: "金额不符"}
	}
	return svc.settleChannelNotify(ctx, "alipay", outTradeNo, params["trade_no"])
}

// errOrderNotPending 区分"订单不存在/读取失败"与"订单非待支付"。
var (
	errOrderNotFound   = errors.New("order not found")
	errOrderReadFailed = errors.New("order read failed")
	errOrderNotPending = errors.New("order not pending")
)

// expectedOrderFen 取订单应收金额(分);并校验订单存在 + 状态=待支付(反欺诈)。
func (svc *Service) expectedOrderFen(ctx context.Context, platformOrderID string) (int, error) {
	o, err := svc.store.GetOrder(ctx, platformOrderID)
	if err == store.ErrNotFound {
		return 0, errOrderNotFound
	}
	if err != nil {
		return 0, errOrderReadFailed
	}
	if o.PaymentStatus != "待支付" {
		return 0, errOrderNotPending
	}
	return yuanStringToFen(o.Amount)
}

// notifyOrderFault 把订单层错误翻成 NotifyOutcome。
//   - 不存在 → 拒绝(可能伪造,回失败让渠道知道)。
//   - 读取失败 → 拒绝(回失败,渠道重推时再试)。
//   - 非待支付 → 已被先前回调处理(幂等),回成功止重推。
func (svc *Service) notifyOrderFault(ctx context.Context, channel, platformOrderID string, err error) NotifyOutcome {
	switch {
	case errors.Is(err, errOrderNotPending):
		svc.log().Info("notify_order_already_settled", "channel", channel, "platformOrderId", platformOrderID)
		return NotifyOutcome{OK: true, Message: "订单已结算"}
	case errors.Is(err, errOrderNotFound):
		svc.log().Warn("notify_order_not_found", "channel", channel, "platformOrderId", platformOrderID)
		return NotifyOutcome{OK: false, Message: "订单不存在"}
	default:
		svc.log().Warn("notify_order_read_failed", "channel", channel, "platformOrderId", platformOrderID)
		return NotifyOutcome{OK: false, Message: "订单读取失败"}
	}
}

// releaseClaim 回滚幂等认领;回滚失败必须显式告警——否则认领行残留会让后续 notify 命中
// "已处理"而永久漏发且无人知晓(blocker 巡检重投也只扫 已支付 订单,认领残留不在其内)。
func (svc *Service) releaseClaim(ctx context.Context, channel, platformOrderID string) {
	if err := svc.store.ReleasePaymentNotification(ctx, channel, platformOrderID); err != nil {
		svc.log().Warn("notify_release_failed", "channel", channel, "platformOrderId", platformOrderID, "err", err.Error())
	}
}

// settleChannelNotify 幂等认领 → CompletePayment(触发既有充值回调投递)。
func (svc *Service) settleChannelNotify(ctx context.Context, channel, platformOrderID, channelTxnID string) NotifyOutcome {
	claimErr := svc.store.ClaimPaymentNotification(ctx, channel, platformOrderID, channelTxnID)
	if errors.Is(claimErr, store.ErrNotifyAlreadyProcessed) {
		svc.log().Info("notify_idempotent_hit", "channel", channel, "platformOrderId", platformOrderID)
		return NotifyOutcome{OK: true, Message: "重复回调,已处理"}
	}
	if claimErr != nil {
		svc.log().Warn("notify_claim_failed", "channel", channel, "platformOrderId", platformOrderID, "err", claimErr.Error())
		return NotifyOutcome{OK: false, Message: "幂等认领失败"}
	}
	// 取 gameId(回调只带 out_trade_no=platformOrderId)。
	o, err := svc.store.GetOrder(ctx, platformOrderID)
	if err != nil {
		svc.releaseClaim(ctx, channel, platformOrderID)
		svc.log().Warn("notify_order_read_failed_after_claim", "channel", channel, "platformOrderId", platformOrderID)
		return NotifyOutcome{OK: false, Message: "订单读取失败"}
	}
	if f := svc.CompletePayment(ctx, o.GameID, platformOrderID, "成功"); f != nil {
		svc.releaseClaim(ctx, channel, platformOrderID)
		svc.log().Warn("notify_complete_payment_failed", "channel", channel, "platformOrderId", platformOrderID, "reason", f.Reason)
		return NotifyOutcome{OK: false, Message: "发放编排失败"}
	}
	svc.log().Info("notify_settled", "channel", channel, "platformOrderId", platformOrderID, "channelTxn", channelTxnID)
	return NotifyOutcome{OK: true, Message: "已发放"}
}

// ---------- 金额换算 ----------

// yuanStringToFen 把 "328.00"(元,两位小数)转为整数分;非法格式报错。
func yuanStringToFen(yuan string) (int, error) {
	yuan = strings.TrimSpace(yuan)
	if yuan == "" {
		return 0, errors.New("空金额")
	}
	f, err := strconv.ParseFloat(yuan, 64)
	if err != nil {
		return 0, fmt.Errorf("金额格式非法: %w", err)
	}
	if f < 0 {
		return 0, errors.New("金额为负")
	}
	// 四舍五入到分,避免浮点尾差(如 328.00*100=32799.999...)。
	return int(math.Round(f * 100)), nil
}
