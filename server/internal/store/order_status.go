package store

// 订单状态机的合法取值(closed set):中文显示串即机器状态(04 §2.9.2)。
// 值保持中文不变(不改 wire 契约、不迁存量行);此处是全仓唯一常量源——
// domain/api 引用这些常量而非裸字面量,store 是最底层(domain/api 都 import store,
// store 不能反向 import 它们),故自愈巡检 WHERE 也只能用本包常量。
// DB 侧 CHECK 约束(migration 0013)与此列举严格同步,二者改动须一并修订。
const (
	// payment_status:支付态。
	PaymentPending = "待支付" // 订单已建,尚未支付
	PaymentPaid    = "已支付" // 渠道确认收款
	PaymentFailed  = "支付失败" // 支付明确失败

	// callback_status:出站充值回调投递态(仅诊断,不入 04 物品发放判定)。
	CallbackPending    = "未投递"   // 尚未触发投递(支付失败/初始)
	CallbackDelivering = "投递中"   // 正在投递,结果未定
	CallbackConfirmed  = "已确认"   // 游戏服务端已确认收款
	CallbackFailed     = "投递失败"  // 投递未获确认,待巡检重投
	CallbackNoURL      = "无回调地址" // 游戏未配回调地址,无法投递
)
