package com.m5755.operate.core.flow;

import com.m5755.operate.api.UserListener;
import com.m5755.operate.core.gateway.PlatformGateway;
import com.m5755.operate.core.gateway.Reason;
import com.m5755.operate.core.gateway.Results;
import com.m5755.operate.core.store.Storage;

/**
 * 登录链路状态机(03 全链,里程碑 2):
 * init(config) → 维护门禁 → 协议告知 → 5755 账户登录/自动登录(账户有效检查)
 * → 实名认证 → 防沉迷进入门禁 → 小号选择/默认自动进入 → 小号登录 → 返回 account+token。
 *
 * <p>关键不变量(JVM 单测验证):
 * <ul>
 *   <li>维护/协议拒绝/防沉迷进入门禁阻断 → 绝不触发 {@link UserListener#onLogout()};
 *   <li>账户失效(自动登录校验不过/踢号)→ 清会话回登录窗 + 账号变化(03 §3);
 *   <li>小号失效 → 回小号选择页,不回登录窗(03 §3,与账户失效分流);
 *   <li>平台不可用 → 阻断提示,不用本地态放行也不误判失效;
 *   <li>小号列表为空 = 平台侧异常 → 阻断并诊断,不本地伪造(B4 反面);
 *   <li>account/token 只来自小号登录接口(04 §2.7)。
 * </ul>
 * 网关调用阻塞,由上层在后台线程驱动;本类只表达状态迁移。
 */
public final class ColdStartController {

    /** 登录链路终局回调(facade 翻译为公开 DataListener)。 */
    public interface FlowListener {
        void onFlowSuccess(String account, String token);

        /** 玩家在登录链路中关闭/取消(进入未完成)。 */
        void onFlowCanceled();

        /** 链路被阻断(维护/门禁/平台不可用),reason 供诊断。 */
        void onFlowBlocked(String reason, String message);
    }

    private static final FlowListener NOOP_FLOW = new FlowListener() {
        public void onFlowSuccess(String account, String token) {
        }

        public void onFlowCanceled() {
        }

        public void onFlowBlocked(String reason, String message) {
        }
    };

    private final PlatformGateway gateway;
    private final Storage storage;
    private final FlowUi ui;
    private final UserListener listener;
    private FlowListener flow = NOOP_FLOW;

    private String gameId;
    private String channelId = "default";
    private String channelSource = "manifest";
    private String protocolVersion;
    private String displayName = "";

    public ColdStartController(PlatformGateway gateway, Storage storage, FlowUi ui, UserListener listener) {
        this.gateway = gateway;
        this.storage = storage;
        this.ui = ui;
        this.listener = listener;
    }

    public void setFlowListener(FlowListener flowListener) {
        this.flow = flowListener == null ? NOOP_FLOW : flowListener;
    }

    public void setChannel(String channelId, String channelSource) {
        this.channelId = channelId;
        this.channelSource = channelSource;
    }

    // ===== init / login(03 §1:init 是 login 的前置,维护门禁在 init 后、协议前判断)=====

    private Results.Config config;

    /** init:配置拉取(03 §2.1)。不弹任何业务 UI;结果由 facade 翻译给接入方 Listener。 */
    public Results.Config init(String gameId, String sdkVersion, String packageName) {
        this.gameId = gameId;
        Results.Config cfg = gateway.fetchConfig(gameId, sdkVersion, packageName, channelId, channelSource);
        if (cfg.ok) {
            this.config = cfg;
            this.protocolVersion = cfg.protocolVersion;
        }
        return cfg;
    }

    public boolean isInited() {
        return config != null;
    }

    /** login:由游戏调用触发(03 §1)。init 未成功不得进入协议告知或登录。 */
    public void login() {
        if (config == null) {
            ui.showInitError(Reason.PARAM_INVALID, "init 未成功,不能调用 login");
            flow.onFlowBlocked(Reason.PARAM_INVALID, "init 未成功");
            return;
        }
        if (config.maintenanceEnabled) {
            ui.showMaintenance(config.maintenanceMessage); // 阻断进入,不触发账号变化(03 §2.2)
            flow.onFlowBlocked(Reason.MAINTENANCE, config.maintenanceMessage);
            return;
        }
        if (!storage.isProtocolConsented(protocolVersion)) {
            ui.showProtocol(protocolVersion);
            return;
        }
        autoLoginOrWindow();
    }

    /** 兼容入口(里程碑 1 形态):init + login 一步。 */
    public void start(String gameId, String sdkVersion, String packageName) {
        Results.Config cfg = init(gameId, sdkVersion, packageName);
        if (!cfg.ok) {
            ui.showInitError(cfg.reason, cfg.message);
            flow.onFlowBlocked(cfg.reason, cfg.message);
            return;
        }
        login();
    }

    public void onProtocolConsented() {
        storage.setProtocolConsented(protocolVersion);
        autoLoginOrWindow();
    }

    public void onProtocolRejected() {
        ui.onEntryBlockedByProtocolReject(); // 不杀进程、不触发账号变化
        flow.onFlowBlocked("protocol_rejected", "玩家拒绝协议");
    }

    /** 自动登录(#15):本地会话只用于发起校验,有效性由服务端判定(B2 规格)。 */
    private void autoLoginOrWindow() {
        if (!storage.hasSession()) {
            ui.showLoginWindow();
            return;
        }
        Results.AccountCheck chk = gateway.checkAccount(gameId, storage.getPlatformAccountId(), storage.getPlatformToken());
        if (!chk.ok) {
            // 平台不可用:阻断,不放行不误判失效
            ui.showFlowBlocked(chk.reason, chk.message);
            flow.onFlowBlocked(chk.reason, chk.message);
            return;
        }
        if (!chk.valid) {
            boolean hadAccount = storage.getAccount() != null;
            storage.clearSession();
            if (hadAccount) {
                listener.onLogout(); // 账户失效且需清理小号 → 账号变化(03 §2.5)
            }
            ui.showLoginWindow();
            return;
        }
        this.displayName = chk.displayName == null ? "" : chk.displayName;
        ui.showAutoLoginPrompt(this.displayName); // #6:自动登录有指示,不静默跳门禁
        enterRealNameStage();
    }

    // ===== 账户登录 =====

    public void requestCode(String phone) {
        Results.Sms r = gateway.requestSms(gameId, phone);
        if (!r.ok) {
            ui.showLoginError(r.reason, r.message);
            return;
        }
        ui.onSmsRequested(r);
    }

    public void submitLogin(String phone, String code) {
        Results.Login r = gateway.login(gameId, phone, code, channelId, channelSource);
        afterAccountLogin(r);
    }

    /** #29 密码登录提交。 */
    private String pendingPasswordAccount;
    private String pendingPassword;

    public void submitPasswordLogin(String account, String password) {
        this.pendingPasswordAccount = account;
        this.pendingPassword = password;
        Results.Login r = gateway.loginPassword(gameId, account, password, storage.getOrCreateDeviceId(), null, channelId, channelSource);
        if (Reason.DEVICE_VERIFICATION_REQUIRED.equals(r.reason)) {
            ui.showDeviceVerify(account); // 设备首次密码登录 → 设备安全验证页
            return;
        }
        afterAccountLogin(r);
    }

    /** 设备验证页提交验证码 → 带码续登。 */
    public void submitDeviceVerify(String code) {
        Results.Login r = gateway.loginPassword(gameId, pendingPasswordAccount, pendingPassword,
                storage.getOrCreateDeviceId(), code, channelId, channelSource);
        afterAccountLogin(r);
    }

    private void afterAccountLogin(Results.Login r) {
        if (!r.ok) {
            ui.showLoginError(r.reason, r.message);
            return;
        }
        storage.saveSession(r.platformAccountId, r.platformToken, null);
        this.displayName = r.displayName == null ? "" : r.displayName;
        ui.onLoginSuccess(r);
        enterRealNameStage();
    }

    // ===== 实名 + 防沉迷进入门禁(#16) =====

    private void enterRealNameStage() {
        Results.RealName rn = gateway.getRealName(gameId, storage.getPlatformAccountId(), storage.getPlatformToken());
        if (!rn.ok) {
            routeAccountFailure(rn.reason, rn.message);
            return;
        }
        if (!rn.verified) {
            ui.showRealName();
            return;
        }
        gateCheck(rn);
    }

    public void submitRealName(String realName, String idNumber) {
        Results.RealName rn = gateway.submitRealName(gameId, storage.getPlatformAccountId(), storage.getPlatformToken(),
                realName, idNumber);
        if (!rn.ok) {
            if (Reason.PARAM_INVALID.equals(rn.reason)) {
                ui.showRealNameError(rn.reason, rn.message); // 留在实名页可重试
            } else {
                routeAccountFailure(rn.reason, rn.message);
            }
            return;
        }
        gateCheck(rn); // 实名通过流程继续,玩家不重登(03 §2.6)
    }

    private void gateCheck(Results.RealName rn) {
        if (rn.entryBlocked) {
            ui.showAntiAddictionBlocked(rn.message); // 仅提示,不触发账号变化(03 §2.7)
            flow.onFlowBlocked(Reason.ANTI_ADDICTION_ENTRY_BLOCKED, rn.message);
            return;
        }
        enterSubaccountStage(false);
    }

    // ===== 小号阶段(#17) =====

    private Results.SubaccountList lastList;

    private void enterSubaccountStage(boolean switchFlow) {
        Results.SubaccountList list = gateway.listSubaccounts(gameId, storage.getPlatformAccountId(), storage.getPlatformToken());
        if (!list.ok) {
            routeAccountFailure(list.reason, list.message);
            return;
        }
        if (list.items.isEmpty()) {
            // 平台侧异常:阻断并诊断,不本地伪造(B4 反面)
            ui.showFlowBlocked("subaccount_list_empty", "游戏小号列表为空(平台侧异常)");
            flow.onFlowBlocked("subaccount_list_empty", "游戏小号列表为空");
            return;
        }
        this.lastList = list;
        if (!switchFlow && list.defaultAccount != null && !list.defaultAccount.isEmpty()) {
            String name = "";
            for (Results.SubaccountList.Item it : list.items) {
                if (it.account.equals(list.defaultAccount)) {
                    name = it.displayName;
                }
            }
            ui.showAutoEnterPrompt(list.defaultAccount, name);
            return;
        }
        ui.showSubaccountPicker(list, displayName, switchFlow);
    }

    /** 自动进入提示倒计时结束未操作 → 返回默认小号。 */
    public void onAutoEnterElapsed(String account) {
        loginSubaccount(account, false);
    }

    /** 自动进入提示点「切换」→ 完整选择页(切换链路:取消保持默认,03 §2.8)。 */
    public void onAutoEnterSwitch() {
        enterSubaccountStage(true);
    }

    public void onSubaccountChosen(String account, boolean switchFlow) {
        loginSubaccount(account, switchFlow);
    }

    public void onAddSubaccount(boolean switchFlow) {
        Results.SubaccountOp op = gateway.createSubaccount(gameId, storage.getPlatformAccountId(), storage.getPlatformToken());
        if (!op.ok) {
            if (Reason.SUBACCOUNT_LIMIT_REACHED.equals(op.reason)) {
                ui.showPickerNotice("已达小号上限(10 个)");
            } else if (isAccountInvalid(op.reason)) {
                routeAccountFailure(op.reason, op.message);
            } else {
                ui.showPickerNotice("添加失败,请稍后重试");
            }
            return;
        }
        refreshPicker(switchFlow); // 刷新选择页;不自动进入不自动设默认(03 §4.2),已在页内不跳自动进入
    }

    public void onSetDefault(String account, boolean switchFlow) {
        Results.SubaccountOp op = gateway.setDefaultSubaccount(gameId, storage.getPlatformAccountId(), storage.getPlatformToken(), account);
        if (!op.ok) {
            if (isAccountInvalid(op.reason)) {
                routeAccountFailure(op.reason, op.message);
            } else {
                ui.showPickerNotice("设置默认失败");
            }
            return;
        }
        refreshPicker(switchFlow); // 立即展示勾选状态(03 §4.3):刷新选择页本身,不跳自动进入
    }

    /** 选择页内操作后原地刷新(始终展示选择页,不触发自动进入提示)。 */
    private void refreshPicker(boolean switchFlow) {
        Results.SubaccountList list = gateway.listSubaccounts(gameId, storage.getPlatformAccountId(), storage.getPlatformToken());
        if (!list.ok) {
            routeAccountFailure(list.reason, list.message);
            return;
        }
        if (list.items.isEmpty()) {
            ui.showFlowBlocked("subaccount_list_empty", "游戏小号列表为空(平台侧异常)");
            flow.onFlowBlocked("subaccount_list_empty", "游戏小号列表为空");
            return;
        }
        this.lastList = list;
        ui.showSubaccountPicker(list, displayName, switchFlow);
    }

    /** 关闭选择页:登录链路=进入未完成;切换链路=取消并保持当前小号(03 §4.4)。 */
    public void onPickerClosed(boolean switchFlow) {
        if (switchFlow) {
            flow.onFlowCanceled();
        } else {
            flow.onFlowCanceled();
        }
    }

    // ===== 小号登录(#18) =====

    private void loginSubaccount(String account, boolean switchFlow) {
        Results.SubaccountLogin r = gateway.loginSubaccount(gameId, storage.getPlatformAccountId(), storage.getPlatformToken(), account);
        if (!r.ok) {
            if (Reason.SUBACCOUNT_INVALID.equals(r.reason)) {
                // 小号失效 → 回小号选择页,不回登录窗(03 §3 分流硬规则);强制选择页,不跳自动进入
                listener.onLogout(); // 游戏小号失效属账号变化
                ui.showPickerNotice("游戏小号已失效,请重新选择");
                refreshPicker(switchFlow);
            } else if (isAccountInvalid(r.reason)) {
                routeAccountFailure(r.reason, r.message);
            } else {
                ui.showFlowBlocked(r.reason, r.message);
                flow.onFlowBlocked(r.reason, r.message);
            }
            return;
        }
        boolean isSwitch = switchFlow && storage.getAccount() != null;
        storage.saveSubaccount(r.account, r.token);
        if (isSwitch) {
            listener.onLogout(); // 切换小号收敛到账号变化(03 §5)
        }
        ui.showSessionCheck(r.account, maskToken(r.token));
        // #5:用户中心 = 平台 H5,URL 经 /config 下发,带 platformToken
        ui.showFloatBall(r.account, config != null ? config.userCenterUrl : "", storage.getPlatformToken());
        flow.onFlowSuccess(r.account, r.token);
    }

    // ===== 登出 / 切换(公开 API 支撑,#18) =====

    /** 退出登录:清理并回登录窗,不重复协议(03 §6)。 */
    public void logout() {
        storage.clearSession();
        listener.onLogout();
        ui.showLoginWindow();
    }

    /** 切换小号(changeUser):没有当前小号不得隐式触发(03 §4.5)。 */
    public boolean changeUser() {
        if (storage.getAccount() == null || !storage.hasSession()) {
            return false;
        }
        enterSubaccountStage(true);
        return true;
    }

    // ===== #27 角色上报 =====

    public void reportRole(com.m5755.operate.api.RoleInfo info, com.m5755.operate.api.Listener cb) {
        String account = storage.getAccount();
        String token = storage.getSubaccountToken();
        if (account == null || token == null) {
            done(cb, false, com.m5755.operate.provider.OperateCode.NOT_INITIALIZED, "未登录或登录令牌为空");
            return;
        }
        // 客户端校验(05 §1.3)
        String err = validateRole(info);
        if (err != null) {
            done(cb, false, com.m5755.operate.provider.OperateCode.PARAM_ERROR, err);
            ui.showRoleResult(false, "param_invalid", roleFields(info));
            return;
        }
        Results.RoleReport r = gateway.reportRole(gameId, account, token, roleFields(info));
        ui.showRoleResult(r.ok, r.reason, roleFields(info));
        done(cb, r.ok, r.ok ? 0 : com.m5755.operate.provider.OperateCode.FAILURE, r.ok ? "上报成功" : r.message);
    }

    private static String validateRole(com.m5755.operate.api.RoleInfo i) {
        if (isBlank(i.getServerId()) || isBlank(i.getServerName()) || isBlank(i.getRoleName()) || isBlank(i.getRoleLevel())) {
            return "角色字段不完整";
        }
        if (isBlank(i.getRoleId()) || "-1".equals(i.getRoleId())) {
            return "roleId 必须为唯一角色 ID";
        }
        String amt = i.getRoleRechargeAmount();
        if (amt != null && !"-1".equals(amt) && !amt.matches("\\d+\\.\\d{2}")) {
            return "累计充值金额须为 -1 或两位小数";
        }
        return null;
    }

    private static java.util.Map<String, String> roleFields(com.m5755.operate.api.RoleInfo i) {
        java.util.Map<String, String> m = new java.util.LinkedHashMap<String, String>();
        m.put("serverId", nv(i.getServerId()));
        m.put("serverName", nv(i.getServerName()));
        m.put("roleId", nv(i.getRoleId()));
        m.put("roleName", nv(i.getRoleName()));
        m.put("roleLevel", nv(i.getRoleLevel()));
        m.put("roleCe", nv(i.getRoleCe()));
        m.put("roleStage", nv(i.getRoleStage()));
        m.put("roleRechargeAmount", nv(i.getRoleRechargeAmount()));
        m.put("roleGuild", nv(i.getRoleGuild()));
        return m;
    }

    // ===== #28 支付 =====

    /** 当前挂起的支付客户端回调:cb 下沉到容器终态单次 fire(05 §3.1),由 onPayContainerClosed 消费。 */
    private com.m5755.operate.api.Listener pendingPayCb;

    public void recharge(com.m5755.operate.api.Order order, com.m5755.operate.api.Listener cb) {
        String account = storage.getAccount();
        String token = storage.getSubaccountToken();
        if (account == null || token == null) {
            android.util.Log.w("M5755Sdk", "recharge_failed reason=not_logged_in");
            done(cb, false, com.m5755.operate.provider.OperateCode.NOT_INITIALIZED, "未登录或登录令牌为空");
            return;
        }
        String err = validateOrder(order);
        if (err != null) {
            android.util.Log.w("M5755Sdk", "recharge_failed reason=param_invalid detail=" + err);
            done(cb, false, com.m5755.operate.provider.OperateCode.PARAM_ERROR, err); // 无演示订单兜底
            return;
        }
        java.util.Map<String, Object> body = new java.util.LinkedHashMap<String, Object>();
        body.put("amount", order.getAmount());
        body.put("cpOrderId", order.getCpOrderId());
        body.put("commodity", order.getCommodity());
        body.put("serverId", order.getServerId());
        body.put("serverName", order.getServerName());
        body.put("roleId", order.getRoleId());
        body.put("roleName", order.getRoleName());
        body.put("roleLevel", order.getRoleLevel());
        Results.OrderCreate r = gateway.createOrder(gameId, account, token, body);
        if (!r.ok) {
            // 诊断:区分防沉迷门禁/小号失效/平台失败(05 §4 可采集诊断),客服据此区分而非靠 message
            android.util.Log.w("M5755Sdk", "recharge_failed reason=" + r.reason);
            if (Reason.SUBACCOUNT_INVALID.equals(r.reason)) {
                // 失效分流硬规则:小号失效 → 失败本次支付 + 账号变化 + 回小号选择,不收敛为普通失败
                done(cb, false, com.m5755.operate.provider.OperateCode.FAILURE, r.message);
                listener.onLogout();
                ui.showPickerNotice("游戏小号已失效,请重新选择");
                refreshPicker(false);
                return;
            }
            // 防沉迷支付门禁等仅失败本次支付,不触发账号变化(03 §3)
            done(cb, false, com.m5755.operate.provider.OperateCode.FAILURE, r.message);
            return;
        }
        // paymentUrl 校验:非空入口必须是 http(s) 支付台(05 §2.5),非法即失败、不展示 mock 收银台
        if (r.paymentUrl != null && !r.paymentUrl.isEmpty()
                && !r.paymentUrl.startsWith("http://") && !r.paymentUrl.startsWith("https://")) {
            android.util.Log.w("M5755Sdk", "recharge_failed reason=invalid_payment_url");
            done(cb, false, com.m5755.operate.provider.OperateCode.FAILURE, "支付入口非法");
            return;
        }
        android.util.Log.i("M5755Sdk", "recharge order_created platformOrderId=" + r.platformOrderId);
        // 展示支付容器(订单显示取自 Order 入参,05 §2.3)
        java.util.Map<String, String> display = new java.util.LinkedHashMap<String, String>();
        display.put("商品", order.getCommodity());
        display.put("金额", "￥" + String.format(java.util.Locale.ROOT, "%.2f", order.getAmount()));
        display.put("小号", account);
        display.put("区服", order.getServerName());
        display.put("角色", order.getRoleName());
        display.put("订单号", r.platformOrderId);
        // 客户端支付回调下沉到容器终态:暂存 cb,由 onPayContainerClosed 在收银台/订单抽屉关闭时单次 fire。
        // (旧实现在此处下单即报"已交接"——玩家还没进收银台就假报,已移除;05 §3.1 三态只在客户端流程终点回调)
        if (pendingPayCb != null) {
            done(pendingPayCb, false, com.m5755.operate.provider.OperateCode.CANCELED, "未完成"); // 兜底:上一笔容器未正常关闭,防 cb 泄漏
        }
        pendingPayCb = cb;
        ui.showPayDrawer(display, r.paymentUrl);
    }

    /**
     * 支付容器终态(收银台 / 订单确认抽屉关闭):客户端支付回调单次 fire(05 §3.1)。
     * handed=true→已交接(SUCCESS),否则→未完成(CANCELED)。"处理中"由容器打开态本身承载、不经 cb。
     * 取出即置 null,保证一次 recharge 只回调一次(吞掉返回键与未来 sentinel 的竞合);与 recharge 同在
     * background 单线程,无需额外同步。注:已交接仅 UI 口径、不表示到账,发货唯一依据是充值回调;当前无
     * 收银台结果信号时一律保守判未完成、绝不假报已交接(已交接/未完成的明确区分靠 #60 收银台 return-URL)。
     */
    public void onPayContainerClosed(boolean handed) {
        com.m5755.operate.api.Listener cb = pendingPayCb;
        pendingPayCb = null;
        if (cb == null) {
            return;
        }
        if (handed) {
            done(cb, true, com.m5755.operate.provider.OperateCode.SUCCESS, "已交接");
        } else {
            done(cb, false, com.m5755.operate.provider.OperateCode.CANCELED, "未完成");
        }
    }

    private static String validateOrder(com.m5755.operate.api.Order o) {
        if (!(o.getAmount() > 0) || Double.isNaN(o.getAmount()) || Double.isInfinite(o.getAmount())) {
            return "金额必须大于 0";
        }
        if (isBlank(o.getCpOrderId()) || o.getCpOrderId().length() > 128) {
            return "CP 订单号非法";
        }
        if (isBlank(o.getCommodity()) || isBlank(o.getServerId()) || isBlank(o.getServerName())
                || isBlank(o.getRoleId()) || isBlank(o.getRoleName())) {
            return "订单归属字段缺失";
        }
        return null;
    }

    // ===== 用户中心动作(#26) =====

    /** 用户中心账号动作收敛(06 §4):switch_account/logout/session_invalid;非法值忽略。 */
    public void onUserCenterAction(String action) {
        if ("logout".equals(action)) {
            logout();
        } else if ("switch_account".equals(action)) {
            changeUser();
        } else if ("session_invalid".equals(action)) {
            storage.clearSession();
            listener.onLogout();
            ui.showLoginWindow();
        }
        // 其他(unknown)忽略
    }

    public String currentAccount() {
        return storage.getAccount();
    }

    // ===== 内部 =====

    private static void done(com.m5755.operate.api.Listener cb, boolean ok, int code, String msg) {
        if (cb != null) {
            cb.onResult(ok, code, msg);
        }
    }

    private static boolean isBlank(String s) {
        return s == null || s.isEmpty();
    }

    private static String nv(String s) {
        return s == null ? "-1" : s;
    }


    /** 主账户失效类失败统一路由:清会话 + 账号变化 + 回登录窗(03 §3)。 */
    private void routeAccountFailure(String reason, String message) {
        if (isAccountInvalid(reason)) {
            storage.clearSession();
            listener.onLogout();
            ui.showLoginWindow();
        } else {
            ui.showFlowBlocked(reason, message);
            flow.onFlowBlocked(reason, message);
        }
    }

    private static boolean isAccountInvalid(String reason) {
        return Reason.PLATFORM_ACCOUNT_INVALID.equals(reason);
    }

    static String maskToken(String token) {
        if (token == null || token.length() < 8) {
            return "****";
        }
        return token.substring(0, 5) + "****" + token.substring(token.length() - 4);
    }
}
