package api

import (
	"html"
	"strings"

	"m5755/server/internal/domain"
	"m5755/server/internal/store"
)

// renderCashierPage 渲染平台收银台 H5。展示金额/商品 + 微信|支付宝单选 + 确认支付。
// 终态 sentinel:取消按钮 → /pay/return?status=canceled;支付成功由渠道异步回调推进发货,
// 收银台在玩家从渠道返回后(无法可靠探知,故由"我已完成支付"按钮辅助)导航到 status=handed。
// status 仅驱动 SDK UI 口径,SDK 绝不据此发货(#60 评论 sentinel 契约第 4 条)。
func renderCashierPage(co *domain.CashierOrder) string {
	orderID := html.EscapeString(co.OrderID)
	amount := html.EscapeString(co.Amount)
	commodity := html.EscapeString(co.Commodity)

	if co.PaymentStatus != store.PaymentPending {
		// 已支付/失败订单不再展示收银,给无害终态。
		return cashierClosedPage(co.PaymentStatus)
	}

	var methods strings.Builder
	if co.WechatEnabled {
		methods.WriteString(methodRadio("wechat", "微信支付", true))
	}
	if co.AlipayEnabled {
		methods.WriteString(methodRadio("alipay", "支付宝", !co.WechatEnabled))
	}
	if !co.WechatEnabled && !co.AlipayEnabled {
		methods.WriteString(`<p style="color:#c0392b">暂无可用支付方式,请稍后再试。</p>`)
	}

	return `<!doctype html><html lang="zh"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>5755 支付</title>` +
		`<style>` + cashierCSS + `</style></head>` +
		`<body>` +
		`<div class="card">` +
		`<h2>订单支付</h2>` +
		`<div class="row"><span class="label">商品</span><span class="value">` + commodity + `</span></div>` +
		`<div class="row amount"><span class="label">金额</span><span class="value">¥` + amount + `</span></div>` +
		`<div class="row"><span class="label">订单号</span><span class="value mono">` + orderID + `</span></div>` +
		`<div class="methods">` + methods.String() + `</div>` +
		`<button id="pay" class="btn primary">确认支付</button>` +
		`<button id="cancel" class="btn ghost">取消</button>` +
		`<p id="hint" class="hint"></p>` +
		`</div>` +
		`<script>` + cashierJS(orderID) + `</script>` +
		`</body></html>`
}

func methodRadio(value, label string, checked bool) string {
	c := ""
	if checked {
		c = " checked"
	}
	return `<label class="method"><input type="radio" name="method" value="` + value + `"` + c + `><span>` + html.EscapeString(label) + `</span></label>`
}

const cashierCSS = `
*{box-sizing:border-box}
body{font-family:-apple-system,sans-serif;background:#f5f5f5;margin:0;padding:24px;color:#25272b}
.card{max-width:420px;margin:0 auto;background:#fff;border-radius:12px;padding:24px;box-shadow:0 2px 12px rgba(0,0,0,.06)}
h2{margin:0 0 20px;font-size:20px}
.row{display:flex;justify-content:space-between;padding:10px 0;border-bottom:1px solid #f0f0f0}
.label{color:#777b83}
.value{font-weight:600}
.mono{font-family:monospace;font-size:13px}
.amount .value{color:#e8541e;font-size:22px}
.methods{margin:20px 0}
.method{display:flex;align-items:center;gap:10px;padding:14px;border:1px solid #e5e5e5;border-radius:8px;margin-bottom:10px;cursor:pointer}
.method input{width:18px;height:18px}
.btn{width:100%;height:48px;border:0;border-radius:8px;font-size:16px;font-weight:700;cursor:pointer;margin-top:8px}
.btn.primary{background:#ffc936;color:#5d4300}
.btn.ghost{background:transparent;color:#777b83}
.hint{text-align:center;color:#777b83;min-height:20px;margin-top:12px}
`

// cashierJS 收银台交互:确认支付 → POST /pay/begin → 按 kind 拉起;取消 → sentinel canceled。
// 注:status=handed sentinel 由"我已完成支付"或从渠道返回后导航触发;真实拉起渠道 App 的外跳属 SDK #61。
func cashierJS(orderID string) string {
	return `
var ORDER=` + jsString(orderID) + `;
function sentinel(s){location.href='/pay/return?status='+s+'&orderId='+encodeURIComponent(ORDER);}
document.getElementById('cancel').onclick=function(){sentinel('canceled');};
document.getElementById('pay').onclick=function(){
  var m=document.querySelector('input[name=method]:checked');
  var hint=document.getElementById('hint');
  if(!m){hint.textContent='请选择支付方式';return;}
  hint.textContent='正在创建支付…';
  var fd=new URLSearchParams();fd.append('orderId',ORDER);fd.append('method',m.value);
  fetch('/pay/begin',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:fd.toString()})
   .then(function(r){return r.json();})
   .then(function(j){
     if(!j.success){hint.textContent=(j.message||'创建支付失败');return;}
     if(j.data&&j.data.kind==='url'&&j.data.redirectUrl){location.href=j.data.redirectUrl;return;}
     hint.textContent='已发起支付,完成后请返回游戏。';
   })
   .catch(function(){hint.textContent='网络异常,请重试。';});
};
`
}

// jsString 把字符串安全嵌入 JS 字面量(转义引号/反斜杠/换行)。
func jsString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\n", `\n`, "\r", `\r`, "<", `\x3c`, ">", `\x3e`)
	return "'" + r.Replace(s) + "'"
}

func cashierErrorPage(msg string) string {
	return `<!doctype html><html lang="zh"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1"><title>5755 支付</title></head>` +
		`<body style="font-family:sans-serif;background:#f5f5f5;margin:0;padding:32px;color:#25272b;text-align:center">` +
		`<p style="margin-top:48px">` + html.EscapeString(msg) + `</p></body></html>`
}

func cashierClosedPage(status string) string {
	msg := "订单状态:" + status + ",无需再次支付。"
	return cashierErrorPage(msg)
}
