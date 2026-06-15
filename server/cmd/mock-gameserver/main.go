// Command mock-gameserver 是一个最小的"游戏服务端"模拟器,用于支付宝沙箱端到端联调,
// 兼作 #59 outbound 充值回调签名的参考实现(receiver 侧)。
//
// 它做且只做四件事:
//  1. 监听 HTTP,接收平台(5755)发来的充值回调 POST(application/json,单层对象,sign 平级)。
//  2. 用与平台 dispatchCallback **逐字节一致**的方式验签:
//     算法 HMAC-SHA256(ADR-0016)、密钥(HMAC key)来自 -secret(默认 dev 口径 m5755-dev-callback-secret-v1)、
//     待签串 = 除 sign 外全部键按字典序 `k=v&...`(末尾对仍带 &;secret 作 HMAC 密钥、不拼进串)(详见 signCallback)。
//  3. 验签通过 → 幂等"发放"并回游戏侧确认体 {"code":200,"msg":"success"}(平台据此判"已确认")。
//     验签失败 → 回 4xx 并记录(平台据此判"投递失败",会进入重投巡检)。
//  4. 每笔落结构化日志(订单号、CP 订单号、金额、验签结果、幂等命中)。
//
// 设计取舍:本程序**只用标准库**,不 import internal/domain —— 因为 internal/domain
// 会传递性拖入 pgx/gin/bcrypt 等 DB/Web 依赖,与"独立可部署的 mock 游戏服务端"相悖。
// signCallback 是 internal/domain/domain_m3.go callbackSign 的逐字节复刻
// (同算法 HMAC-SHA256、同排序 sort.Strings、同 `k=v&` 逐对拼接含末尾 &、secret 同作 HMAC 密钥),
// 两端用同一字节构造,故签名可被本 mock 复算验通。平台出站签名口径若再变,
// 只需同步改这一个函数与平台 callbackSign 即可。
//
// 用法:
//
//	go run ./cmd/mock-gameserver -addr :18080 -secret m5755-dev-callback-secret-v1
//	# 然后把联调 gameId 的 callback_url 指向 http://<本机或穿透域名>:18080/callback
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func main() {
	addr := flag.String("addr", ":18080", "监听地址,如 :18080")
	path := flag.String("path", "/callback", "充值回调接收路径(与 games.callback_url 末段一致)")
	secret := flag.String("secret", envOr("CALLBACK_SECRET", "m5755-dev-callback-secret-v1"),
		"回调验签密钥(须与平台 CALLBACK_SECRET 完全一致;dev 默认 m5755-dev-callback-secret-v1)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	srv := &mockGameServer{secret: *secret, log: logger, delivered: map[string]string{}}

	mux := http.NewServeMux()
	mux.HandleFunc(*path, srv.handleCallback)
	// 健康检查,联调时确认 mock 公网可达 / 穿透生效。
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	logger.Info("mock_gameserver_start", "addr", *addr, "callbackPath", *path,
		"secretLen", len(*secret), "signAlgo", "HMAC-SHA256(secret, k=v&...&)")
	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := httpSrv.ListenAndServe(); err != nil {
		logger.Error("mock_gameserver_exit", "err", err.Error())
		os.Exit(1)
	}
}

type mockGameServer struct {
	secret string
	log    *slog.Logger

	mu        sync.Mutex
	delivered map[string]string // 幂等账本:平台订单号 -> 首次发放结果(重复回调只确认、不重复交付)
}

// handleCallback 接收平台充值回调:验签 → 反欺诈口径外的最小校验 → 幂等"发放" → ACK。
func (s *mockGameServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.log.Warn("callback_method_rejected", "method", r.Method)
		s.writeFail(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.log.Warn("callback_read_failed", "err", err.Error())
		s.writeFail(w, http.StatusBadRequest, "read_failed")
		return
	}

	// 平台 dispatchCallback 发的是单层 JSON 对象,所有值皆 string(domain_m3.go:251-257)。
	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		s.log.Warn("callback_unmarshal_failed", "err", err.Error(), "rawLen", len(body))
		s.writeFail(w, http.StatusBadRequest, "bad_json")
		return
	}

	platformOrderID := payload["platformOrderId"]
	cpOrderID := payload["cpOrderId"]
	amount := payload["amount"]
	account := payload["account"]

	// 1) 验签:逐字节复刻平台 callbackSign。失败必须回 4xx,平台据此判"投递失败"并重投。
	expected := signCallback(payload, s.secret)
	got := payload["sign"]
	if !constTimeEqual(got, expected) {
		s.log.Warn("callback_sign_invalid",
			"platformOrderId", platformOrderID, "cpOrderId", cpOrderID,
			"account", maskAccount(account), "amount", amount, "signOK", false)
		s.writeFail(w, http.StatusUnauthorized, "sign_invalid")
		return
	}

	// 2) 最小契约校验(订单号/金额必须随回调带到,游戏侧据此归属与对账;真实游戏服务端
	//    还应做金额与本地订单一致、account+cpOrderId 归属校验,此处仅演示骨架)。
	if platformOrderID == "" || cpOrderID == "" || amount == "" {
		s.log.Warn("callback_missing_fields",
			"platformOrderId", platformOrderID, "cpOrderId", cpOrderID, "amount", amount, "signOK", true)
		s.writeFail(w, http.StatusBadRequest, "missing_fields")
		return
	}

	// 3) 幂等"发放":同一笔(以平台订单号为幂等键)只发放一次,重复回调只确认、不重复交付
	//    (04 §4 / 05 §3.4)。平台重试/超时重投/巡检补偿都会重复发同字节回调。
	s.mu.Lock()
	prev, repeated := s.delivered[platformOrderID]
	if !repeated {
		s.delivered[platformOrderID] = "granted@" + time.Now().Format(time.RFC3339)
	}
	s.mu.Unlock()

	if repeated {
		s.log.Info("callback_idempotent_repeat",
			"platformOrderId", platformOrderID, "cpOrderId", cpOrderID,
			"account", maskAccount(account), "amount", amount, "signOK", true,
			"action", "ack_only", "firstGrant", prev)
	} else {
		// 真实游戏服务端在此处发放物品;mock 仅记账。
		s.log.Info("callback_granted",
			"platformOrderId", platformOrderID, "cpOrderId", cpOrderID,
			"account", maskAccount(account), "amount", amount, "money", payload["money"],
			"payMoney", payload["pay_money"], "commodity", payload["commodity"],
			"serverId", payload["serverId"], "signOK", true, "action", "grant")
	}

	// 4) 游戏侧确认体:平台 postCallback 仅当 HTTP 200 且 body {code:200,msg:"success"} 才判"已确认"
	//    (domain_m3.go:291-300)。两者皆需,缺一即被平台判失败。
	s.writeAck(w)
}

func (s *mockGameServer) writeAck(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"code":200,"msg":"success"}`))
}

func (s *mockGameServer) writeFail(w http.ResponseWriter, status int, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"code":` + itoa(status) + `,"msg":"` + reason + `"}`))
}

// signCallback 是 internal/domain/domain_m3.go callbackSign 的逐字节复刻(ADR-0016 HMAC-SHA256)。
//
// 待签串构造(必须与平台完全一致,否则验不过):
//  1. 取 params 中除 "sign" 外的全部键;
//  2. 按键名用 Go sort.Strings(UTF-8 字节序,大写/下划线先于小写)升序排列;
//  3. 对每个键拼接 `key=value&`(每对后都带 `&`,**包括最后一对**);
//  4. 以 secret 为 HMAC 密钥对该串 UTF-8 字节做 HMAC-SHA256,输出十六进制小写
//     (注意:secret 是 HMAC 密钥参数、**不拼进串**;旧 MD5 口径末尾的 `key=<secret>` 已移除)。
//
// 例:HMAC-SHA256(m5755-dev-callback-secret-v1, "account=a&amount=6.00&...&serverName=星河一区&") → hex。
func signCallback(params map[string]string, secret string) string {
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

// constTimeEqual 定长比较签名,避免时序泄漏(mock 非生产,但作为参考实现保持口径)。
func constTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// maskAccount 对 account 做确定性脱敏(只在日志里,避免明文小号 ID 入日志)。
// 与平台 maskAccount 不必逐字节一致,仅保证日志可对账且不泄全量。
func maskAccount(a string) string {
	if a == "" {
		return ""
	}
	if len(a) <= 4 {
		return "****"
	}
	return a[:2] + "****" + a[len(a)-2:]
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
