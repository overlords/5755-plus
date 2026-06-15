package com.m5755.operate.core.flow;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import com.m5755.operate.api.UserListener;
import com.m5755.operate.core.gateway.PlatformGateway;
import com.m5755.operate.core.gateway.Results;
import com.m5755.operate.core.store.Storage;

import org.junit.Test;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

/**
 * 登录链路状态机纯逻辑验证(无 Android 依赖),覆盖 03 §3 阻断/回退矩阵的可单测部分。
 */
public class ColdStartControllerTest {

    // ===== 里程碑 1 既有规则 =====

    @Test
    public void maintenanceBlocksEntryWithoutAccountChange() {
        Fixture f = new Fixture();
        f.gw.config.ok = true;
        f.gw.config.maintenanceEnabled = true;
        f.gw.config.maintenanceMessage = "维护中";
        f.c.start("g", "1.0.0", "p");
        assertTrue(f.ui.calls.contains("showMaintenance:维护中"));
        assertFalse(f.ui.calls.contains("showLoginWindow"));
        assertFalse("维护阻断绝不触发账号变化", f.listener.logoutCalled);
    }

    @Test
    public void initFailureBlocksBeforeProtocol() {
        Fixture f = new Fixture();
        f.gw.config.ok = false;
        f.gw.config.reason = "param_invalid";
        f.c.start("g", "1.0.0", "p");
        assertTrue(f.ui.calls.contains("showInitError:param_invalid"));
        assertFalse(f.ui.calls.contains("showProtocol"));
    }

    @Test
    public void freshInstallShowsProtocolThenLogin() {
        Fixture f = new Fixture();
        f.gw.config.ok = true;
        f.gw.config.protocolVersion = "1";
        f.c.start("g", "1.0.0", "p");
        assertTrue(f.ui.calls.contains("showProtocol:1"));
        f.c.onProtocolConsented();
        assertTrue(f.store.isProtocolConsented("1"));
        assertTrue(f.ui.calls.contains("showLoginWindow"));
    }

    @Test
    public void protocolRejectDoesNotFireAccountChange() {
        Fixture f = new Fixture();
        f.c.onProtocolRejected();
        assertTrue(f.ui.calls.contains("onEntryBlockedByProtocolReject"));
        assertFalse(f.listener.logoutCalled);
    }

    @Test
    public void loginFailureShowsErrorNoSession() {
        Fixture f = new Fixture();
        f.gw.login.ok = false;
        f.gw.login.reason = "sms_code_invalid";
        f.c.submitLogin("13800000000", "000000");
        assertTrue(f.ui.calls.contains("showLoginError:sms_code_invalid"));
        assertFalse(f.store.hasSession());
    }

    // ===== #15 自动登录 =====

    @Test
    public void autoLoginValidSkipsLoginWindow() {
        Fixture f = consentedWithSession();
        f.gw.check.ok = true;
        f.gw.check.valid = true;
        f.gw.realName.ok = true;
        f.gw.realName.verified = true;
        f.gw.list.ok = true;
        f.gw.list.items.add(item("sub_1", "小号1", false));
        f.c.start("g", "1.0.0", "p");
        assertFalse("有效会话不应弹登录窗", f.ui.calls.contains("showLoginWindow"));
        assertTrue("应直达小号选择", f.ui.calls.contains("showSubaccountPicker"));
    }

    @Test
    public void autoLoginInvalidClearsAndShowsWindow() {
        Fixture f = consentedWithSession();
        f.store.saveSubaccount("sub_1", "st_1"); // 曾有当前小号
        f.gw.check.ok = true;
        f.gw.check.valid = false;
        f.c.start("g", "1.0.0", "p");
        assertTrue(f.ui.calls.contains("showLoginWindow"));
        assertFalse("会话应被清理", f.store.hasSession());
        assertTrue("账户失效且需清理小号 → 账号变化", f.listener.logoutCalled);
    }

    @Test
    public void autoLoginPlatformUnavailableBlocksWithoutMisjudging() {
        Fixture f = consentedWithSession();
        f.gw.check.ok = false;
        f.gw.check.reason = "platform_unavailable";
        f.c.start("g", "1.0.0", "p");
        assertTrue(f.ui.calls.contains("showFlowBlocked:platform_unavailable"));
        assertFalse("平台不可用不得误判失效", f.listener.logoutCalled);
        assertTrue("本地会话应保留", f.store.hasSession());
        assertFalse("不得本地放行", f.ui.calls.contains("showSubaccountPicker"));
    }

    // ===== #16 实名 + 门禁 =====

    @Test
    public void unverifiedAccountEntersRealName() {
        Fixture f = loggedIn();
        f.gw.realName.ok = true;
        f.gw.realName.verified = false;
        f.c.submitLogin("13800000000", "123456");
        assertTrue(f.ui.calls.contains("showRealName"));
    }

    @Test
    public void realNameSubmitContinuesWithoutRelogin() {
        Fixture f = loggedIn();
        f.gw.realNameSubmit.ok = true;
        f.gw.realNameSubmit.verified = true;
        f.gw.list.ok = true;
        f.gw.list.items.add(item("sub_1", "小号1", false));
        f.c.submitRealName("张三", "11010119900101001X");
        assertTrue("实名通过应继续小号流程", f.ui.calls.contains("showSubaccountPicker"));
        assertFalse("不应回登录窗", f.ui.calls.contains("showLoginWindow"));
    }

    @Test
    public void antiAddictionEntryBlockedNoAccountChange() {
        Fixture f = loggedIn();
        f.gw.realName.ok = true;
        f.gw.realName.verified = true;
        f.gw.realName.entryBlocked = true;
        f.c.submitLogin("13800000000", "123456");
        assertTrue(f.ui.calls.contains("showAntiAddictionBlocked"));
        assertFalse("门禁阻断不触发账号变化", f.listener.logoutCalled);
        assertFalse(f.ui.calls.contains("showLoginWindow"));
    }

    // ===== #17 小号分支 =====

    @Test
    public void emptyListBlocksWithoutFabrication() {
        Fixture f = loggedIn();
        f.gw.realName.ok = true;
        f.gw.realName.verified = true;
        f.gw.list.ok = true; // 空列表
        f.c.submitLogin("13800000000", "123456");
        assertTrue(f.ui.calls.contains("showFlowBlocked:subaccount_list_empty"));
        assertFalse(f.ui.calls.contains("showSubaccountPicker"));
    }

    @Test
    public void defaultAccountShowsAutoEnterPrompt() {
        Fixture f = loggedIn();
        f.gw.realName.ok = true;
        f.gw.realName.verified = true;
        f.gw.list.ok = true;
        f.gw.list.defaultAccount = "sub_1";
        f.gw.list.items.add(item("sub_1", "小号1", true));
        f.c.submitLogin("13800000000", "123456");
        assertTrue(f.ui.calls.contains("showAutoEnterPrompt:sub_1"));
        assertFalse("有默认不展示完整选择页", f.ui.calls.contains("showSubaccountPicker"));
    }

    @Test
    public void switchFlowNeverShowsAutoEnter() {
        Fixture f = loggedIn();
        f.store.saveSubaccount("sub_1", "st_1");
        f.gw.list.ok = true;
        f.gw.list.defaultAccount = "sub_1";
        f.gw.list.items.add(item("sub_1", "小号1", true));
        assertTrue(f.c.changeUser());
        assertTrue("切换链路必须进完整选择页", f.ui.calls.contains("showSubaccountPicker"));
        assertFalse("切换不展示自动进入提示", f.ui.calls.contains("showAutoEnterPrompt:sub_1"));
    }

    @Test
    public void changeUserWithoutCurrentAccountRefused() {
        Fixture f = loggedIn(); // 有会话但无当前小号
        assertFalse("没有当前小号不得隐式触发切换", f.c.changeUser());
        assertFalse(f.ui.calls.contains("showSubaccountPicker"));
    }

    // ===== #18 小号登录与分流 =====

    @Test
    public void subaccountLoginSuccessDeliversUser() {
        Fixture f = loggedIn();
        f.gw.subLogin.ok = true;
        f.gw.subLogin.account = "sub_1";
        f.gw.subLogin.token = "st_token_123456";
        f.c.onSubaccountChosen("sub_1", false);
        assertEquals("sub_1", f.flow.account);
        assertEquals("st_token_123456", f.flow.token);
        assertTrue("应展示登录态校验弹窗", f.ui.calls.contains("showSessionCheck:sub_1"));
        assertEquals("sub_1", f.store.getAccount());
    }

    @Test
    public void subaccountInvalidRoutesBackToPicker() {
        Fixture f = loggedIn();
        f.gw.subLogin.ok = false;
        f.gw.subLogin.reason = "subaccount_invalid";
        f.gw.list.ok = true;
        f.gw.list.items.add(item("sub_2", "小号2", false));
        f.c.onSubaccountChosen("sub_1", false);
        assertTrue("小号失效进选择页", f.ui.calls.contains("showSubaccountPicker"));
        assertFalse("不回登录窗(03 §3 分流)", f.ui.calls.contains("showLoginWindow"));
        assertTrue("小号失效属账号变化", f.listener.logoutCalled);
    }

    @Test
    public void platformAccountInvalidRoutesToLoginWindow() {
        Fixture f = loggedIn();
        f.gw.subLogin.ok = false;
        f.gw.subLogin.reason = "platform_account_invalid";
        f.c.onSubaccountChosen("sub_1", false);
        assertTrue("账户失效回登录窗", f.ui.calls.contains("showLoginWindow"));
        assertFalse("不进选择页(03 §3 分流)", f.ui.calls.contains("showSubaccountPicker"));
        assertTrue(f.listener.logoutCalled);
        assertFalse(f.store.hasSession());
    }

    @Test
    public void logoutClearsAndShowsWindowWithoutProtocol() {
        Fixture f = loggedIn();
        f.store.setProtocolConsented("1");
        f.c.logout();
        assertTrue(f.ui.calls.contains("showLoginWindow"));
        assertTrue(f.listener.logoutCalled);
        assertFalse(f.store.hasSession());
        assertFalse("登出不重复协议告知", f.ui.calls.contains("showProtocol:1"));
    }

    @Test
    public void addSubaccountLimitShowsNotice() {
        Fixture f = loggedIn();
        f.gw.createOp.ok = false;
        f.gw.createOp.reason = "subaccount_limit_reached";
        f.c.onAddSubaccount(false);
        assertTrue(f.ui.calls.contains("showPickerNotice"));
        assertFalse(f.listener.logoutCalled);
    }

    // ===== #27 角色上报 =====

    @Test
    public void roleReportClientValidationBlocksBadFields() {
        Fixture f = loggedInWithSubaccount();
        com.m5755.operate.api.RoleInfo info = validRole();
        info.setRoleId("-1"); // 非法
        final boolean[] cb = {false, false};
        f.c.reportRole(info, (s, code, m) -> {
            cb[0] = true;
            cb[1] = s;
        });
        assertTrue("应回调", cb[0]);
        assertFalse("roleId=-1 应失败", cb[1]);
        assertTrue("展示失败结果", f.ui.calls.contains("showRoleResult:false"));
    }

    @Test
    public void roleReportNotLoggedInFailsWithoutRequest() {
        Fixture f = loggedIn(); // 有账户会话但无小号
        final boolean[] cb = {false};
        f.c.reportRole(validRole(), (s, code, m) -> cb[0] = s);
        assertFalse("无当前小号应失败", cb[0]);
        assertFalse("不应发起上报", f.ui.calls.contains("showRoleResult:true"));
    }

    @Test
    public void roleReportSuccessShowsRealFields() {
        Fixture f = loggedInWithSubaccount();
        f.gw.roleReport.ok = true;
        f.gw.roleReport.reported = true;
        f.c.reportRole(validRole(), null);
        assertTrue(f.ui.calls.contains("showRoleResult:true"));
        assertEquals("云起", f.ui.lastRoleFields.get("roleName"));
    }

    // ===== #28 支付 =====

    @Test
    public void rechargeInvalidOrderNoRequest() {
        Fixture f = loggedInWithSubaccount();
        com.m5755.operate.api.Order o = validOrder();
        o.setAmount(0); // 非法
        final boolean[] cb = {false};
        f.c.recharge(o, (s, code, m) -> cb[0] = s);
        assertFalse(cb[0]);
        assertFalse("无演示订单兜底,不应展示支付容器", f.ui.calls.contains("showPayDrawer"));
    }

    @Test
    public void rechargeOKShowsPayDrawerFromInput() {
        Fixture f = loggedInWithSubaccount();
        f.gw.orderCreate.ok = true;
        f.gw.orderCreate.orderId = "P5755x";
        f.gw.orderCreate.paymentUrl = "https://sdk-dev/pay/P5755x";
        f.c.recharge(validOrder(), null);
        assertTrue(f.ui.calls.contains("showPayDrawer"));
        assertEquals("648 元宝", f.ui.lastPayDisplay.get("商品"));
    }

    @Test
    public void rechargePaymentGateOnlyFailsPaymentNoAccountChange() {
        Fixture f = loggedInWithSubaccount();
        f.gw.orderCreate.ok = false;
        f.gw.orderCreate.reason = "anti_addiction_payment_blocked";
        final boolean[] cb = {true};
        f.c.recharge(validOrder(), (s, code, m) -> cb[0] = s);
        assertFalse("支付门禁仅失败本次支付", cb[0]);
        assertFalse("不触发账号变化", f.listener.logoutCalled);
    }

    @Test
    public void rechargeSubaccountInvalidRoutesWithAccountChange() {
        // 失效分流:小号失效(subaccount_invalid)→ 失败本次支付 + 触发账号变化(对照防沉迷门禁不触发)
        Fixture f = loggedInWithSubaccount();
        f.gw.orderCreate.ok = false;
        f.gw.orderCreate.reason = "subaccount_invalid";
        final boolean[] cb = {true};
        f.c.recharge(validOrder(), (s, code, m) -> cb[0] = s);
        assertFalse("支付失败本次", cb[0]);
        assertTrue("小号失效触发账号变化", f.listener.logoutCalled);
    }

    @Test
    public void rechargeOKDoesNotFireHandedOffOnOrderCreate() {
        // 口径漏洞回归:下单成功不得立即报"已交接"——cb 须挂起到容器终态(05 §3.1)
        Fixture f = loggedInWithSubaccount();
        f.gw.orderCreate.ok = true;
        f.gw.orderCreate.orderId = "P5755x";
        f.gw.orderCreate.paymentUrl = "https://sdk-dev/pay/P5755x";
        final int[] calls = {0};
        f.c.recharge(validOrder(), (s, code, m) -> calls[0]++);
        assertEquals("下单成功不应立即回调(已交接下沉到容器终态)", 0, calls[0]);
        assertTrue("应展示支付容器", f.ui.calls.contains("showPayDrawer"));
    }

    @Test
    public void payContainerClosedHandedFiresHandedOff() {
        Fixture f = loggedInWithSubaccount();
        f.gw.orderCreate.ok = true;
        f.gw.orderCreate.orderId = "P5755x";
        f.gw.orderCreate.paymentUrl = "https://sdk-dev/pay/P5755x";
        final int[] calls = {0};
        final boolean[] ok = {false};
        final int[] code = {-1};
        final String[] msg = {null};
        f.c.recharge(validOrder(), (s, c, m) -> { calls[0]++; ok[0] = s; code[0] = c; msg[0] = m; });
        f.c.onPayContainerClosed(true);
        assertEquals(1, calls[0]);
        assertTrue("已交接=成功", ok[0]);
        assertEquals(com.m5755.operate.provider.OperateCode.SUCCESS, code[0]);
        assertEquals("已交接", msg[0]);
    }

    @Test
    public void payContainerClosedNotHandedFiresNotCompleted() {
        Fixture f = loggedInWithSubaccount();
        f.gw.orderCreate.ok = true;
        f.gw.orderCreate.orderId = "P5755x";
        f.gw.orderCreate.paymentUrl = "https://sdk-dev/pay/P5755x";
        final int[] calls = {0};
        final boolean[] ok = {true};
        final int[] code = {-1};
        final String[] msg = {null};
        f.c.recharge(validOrder(), (s, c, m) -> { calls[0]++; ok[0] = s; code[0] = c; msg[0] = m; });
        f.c.onPayContainerClosed(false); // 无收银台信号→保守判未完成
        assertEquals(1, calls[0]);
        assertFalse("未完成=失败", ok[0]);
        assertEquals(com.m5755.operate.provider.OperateCode.CANCELED, code[0]);
        assertEquals("未完成", msg[0]);
    }

    @Test
    public void payContainerClosedFiresExactlyOnce() {
        // 单次触发:多次终态信号(返回键与未来 sentinel 竞合)只回调一次
        Fixture f = loggedInWithSubaccount();
        f.gw.orderCreate.ok = true;
        f.gw.orderCreate.orderId = "P5755x";
        f.gw.orderCreate.paymentUrl = "https://sdk-dev/pay/P5755x";
        final int[] calls = {0};
        f.c.recharge(validOrder(), (s, c, m) -> calls[0]++);
        f.c.onPayContainerClosed(false);
        f.c.onPayContainerClosed(false);
        f.c.onPayContainerClosed(true);
        assertEquals("一次 recharge 只回调一次客户端支付回调", 1, calls[0]);
    }

    // ===== #29 密码登录 / 设备验证 =====

    @Test
    public void passwordLoginDeviceVerificationFlow() {
        Fixture f = inited();
        f.gw.passwordLogin.reason = "device_verification_required";
        f.c.submitPasswordLogin("13900000000", "Test1234");
        assertTrue("应进设备验证页", f.ui.calls.contains("showDeviceVerify:13900000000"));
        // 带码续登成功
        f.gw.passwordLogin.ok = true;
        f.gw.passwordLogin.reason = null;
        f.gw.passwordLogin.platformAccountId = "pa_pwd";
        f.gw.passwordLogin.platformToken = "pt_pwd";
        f.gw.realName.ok = true;
        f.gw.realName.verified = true;
        f.gw.list.ok = true;
        f.gw.list.items.add(item("sub_p", "小号1", false));
        f.c.submitDeviceVerify("123456");
        assertEquals("续登应带验证码", "123456", f.gw.lastPasswordDeviceCode);
        assertTrue("验证后继续到小号选择", f.ui.calls.contains("showSubaccountPicker"));
    }

    // ===== #26 用户中心动作 =====

    @Test
    public void userCenterSwitchEntersPicker() {
        Fixture f = loggedInWithSubaccount();
        f.gw.list.ok = true;
        f.gw.list.items.add(item("sub_1", "小号1", false));
        f.c.onUserCenterAction("switch_account");
        assertTrue(f.ui.calls.contains("showSubaccountPicker"));
    }

    @Test
    public void userCenterLogoutClearsAndShowsWindow() {
        Fixture f = loggedInWithSubaccount();
        f.c.onUserCenterAction("logout");
        assertTrue(f.ui.calls.contains("showLoginWindow"));
        assertTrue(f.listener.logoutCalled);
        assertFalse(f.store.hasSession());
    }

    @Test
    public void userCenterUnknownActionIgnored() {
        Fixture f = loggedInWithSubaccount();
        f.c.onUserCenterAction("hack");
        assertFalse(f.listener.logoutCalled);
        assertFalse(f.ui.calls.contains("showLoginWindow"));
    }

    private static com.m5755.operate.api.RoleInfo validRole() {
        com.m5755.operate.api.RoleInfo i = new com.m5755.operate.api.RoleInfo();
        i.setServerId("s1");
        i.setServerName("星河一区");
        i.setRoleId("role_1");
        i.setRoleName("云起");
        i.setRoleLevel("68");
        i.setRoleRechargeAmount("328.00");
        return i;
    }

    private static com.m5755.operate.api.Order validOrder() {
        com.m5755.operate.api.Order o = new com.m5755.operate.api.Order();
        o.setAmount(328.0);
        o.setCpOrderId("cp_1");
        o.setCommodity("648 元宝");
        o.setServerId("s1");
        o.setServerName("星河一区");
        o.setRoleId("role_1");
        o.setRoleName("云起");
        o.setRoleLevel("68");
        return o;
    }

    private static Fixture inited() {
        Fixture f = new Fixture();
        f.gw.config.ok = true;
        f.gw.config.protocolVersion = "1";
        f.c.init("g", "1.0.0", "p");
        return f;
    }

    private static Fixture loggedInWithSubaccount() {
        Fixture f = loggedIn();
        f.store.saveSubaccount("sub_cur", "st_cur");
        return f;
    }

    // ===== 夹具 =====

    private static Results.SubaccountList.Item item(String account, String name, boolean dft) {
        Results.SubaccountList.Item it = new Results.SubaccountList.Item();
        it.account = account;
        it.displayName = name;
        it.isDefault = dft;
        return it;
    }

    /** 已同意协议 + 有本地会话(自动登录场景)。 */
    private static Fixture consentedWithSession() {
        Fixture f = new Fixture();
        f.gw.config.ok = true;
        f.gw.config.protocolVersion = "1";
        f.store.setProtocolConsented("1");
        f.store.saveSession("pa_1", "pt_1", null);
        return f;
    }

    /** 已 init + 已有账户会话(登录后阶段);login 网关默认成功。 */
    private static Fixture loggedIn() {
        Fixture f = new Fixture();
        f.gw.config.ok = true;
        f.gw.config.protocolVersion = "1";
        f.c.init("g", "1.0.0", "p");
        f.store.saveSession("pa_1", "pt_1", null);
        f.gw.login.ok = true;
        f.gw.login.platformAccountId = "pa_1";
        f.gw.login.platformToken = "pt_1";
        return f;
    }

    static final class Fixture {
        final FakeGateway gw = new FakeGateway();
        final FakeStorage store = new FakeStorage();
        final RecordingUi ui = new RecordingUi();
        final RecordingListener listener = new RecordingListener();
        final RecordingFlow flow = new RecordingFlow();
        final ColdStartController c;

        Fixture() {
            c = new ColdStartController(gw, store, ui, listener);
            c.setFlowListener(flow);
        }
    }

    static final class RecordingFlow implements ColdStartController.FlowListener {
        String account;
        String token;
        boolean canceled;
        String blockedReason;

        public void onFlowSuccess(String account, String token) {
            this.account = account;
            this.token = token;
        }

        public void onFlowCanceled() {
            canceled = true;
        }

        public void onFlowBlocked(String reason, String message) {
            blockedReason = reason;
        }
    }

    static final class FakeGateway implements PlatformGateway {
        final Results.Config config = new Results.Config();
        final Results.Sms sms = new Results.Sms();
        final Results.Login login = new Results.Login();
        final Results.AccountCheck check = new Results.AccountCheck();
        final Results.RealName realName = new Results.RealName();
        final Results.RealName realNameSubmit = new Results.RealName();
        final Results.SubaccountList list = new Results.SubaccountList();
        final Results.SubaccountOp createOp = new Results.SubaccountOp();
        final Results.SubaccountOp defaultOp = new Results.SubaccountOp();
        final Results.SubaccountLogin subLogin = new Results.SubaccountLogin();

        public Results.Config fetchConfig(String a, String b, String c, String d, String e) {
            return config;
        }

        public Results.Sms requestSms(String a, String b) {
            return sms;
        }

        public Results.Login login(String a, String b, String c, String d, String e) {
            return login;
        }

        public Results.AccountCheck checkAccount(String a, String b, String c) {
            return check;
        }

        public Results.RealName getRealName(String a, String b, String c) {
            return realName;
        }

        public Results.RealName submitRealName(String a, String b, String c, String d, String e) {
            return realNameSubmit;
        }

        public Results.SubaccountList listSubaccounts(String a, String b, String c) {
            return list;
        }

        public Results.SubaccountOp createSubaccount(String a, String b, String c) {
            return createOp;
        }

        public Results.SubaccountOp setDefaultSubaccount(String a, String b, String c, String d) {
            return defaultOp;
        }

        public Results.SubaccountLogin loginSubaccount(String a, String b, String c, String d) {
            return subLogin;
        }

        final Results.Login passwordLogin = new Results.Login();
        final Results.RoleReport roleReport = new Results.RoleReport();
        final Results.OrderCreate orderCreate = new Results.OrderCreate();
        String lastPasswordDeviceCode;

        public Results.Login loginPassword(String a, String b, String c, String dev, String code, String d, String e) {
            lastPasswordDeviceCode = code;
            return passwordLogin;
        }

        public Results.RoleReport reportRole(String a, String b, String c, java.util.Map<String, String> f) {
            return roleReport;
        }

        public Results.OrderCreate createOrder(String a, String b, String c, java.util.Map<String, Object> o) {
            return orderCreate;
        }
    }

    static final class FakeStorage implements Storage {
        private final Set<String> consented = new HashSet<>();
        private String paId;
        private String pToken;
        private String account;
        private String subToken;

        public boolean isProtocolConsented(String v) {
            return consented.contains(v);
        }

        public void setProtocolConsented(String v) {
            consented.add(v);
        }

        public boolean hasSession() {
            return pToken != null;
        }

        public void saveSession(String pa, String pt, String acc) {
            paId = pa;
            pToken = pt;
            account = acc;
        }

        public void clearSession() {
            paId = null;
            pToken = null;
            account = null;
            subToken = null;
        }

        public String getPlatformAccountId() {
            return paId;
        }

        public String getPlatformToken() {
            return pToken;
        }

        public String getAccount() {
            return account;
        }

        public void saveSubaccount(String acc, String st) {
            account = acc;
            subToken = st;
        }

        public String getSubaccountToken() {
            return subToken;
        }

        public String getOrCreateDeviceId() {
            return "dev_test";
        }
    }

    static final class RecordingUi implements FlowUi {
        final List<String> calls = new ArrayList<>();

        public void showInitError(String reason, String message) {
            calls.add("showInitError:" + reason);
        }

        public void showMaintenance(String message) {
            calls.add("showMaintenance:" + message);
        }

        public void showProtocol(String v) {
            calls.add("showProtocol:" + v);
        }

        public void showLoginWindow() {
            calls.add("showLoginWindow");
        }

        public void onSmsRequested(Results.Sms r) {
            calls.add("onSmsRequested");
        }

        public void showLoginError(String reason, String message) {
            calls.add("showLoginError:" + reason);
        }

        public void onLoginSuccess(Results.Login r) {
            calls.add("onLoginSuccess");
        }

        public void onEntryBlockedByProtocolReject() {
            calls.add("onEntryBlockedByProtocolReject");
        }

        public void showRealName() {
            calls.add("showRealName");
        }

        public void showRealNameError(String reason, String message) {
            calls.add("showRealNameError:" + reason);
        }

        public void showAntiAddictionBlocked(String message) {
            calls.add("showAntiAddictionBlocked");
        }

        public void showSubaccountPicker(Results.SubaccountList list, String nick, boolean sw) {
            calls.add("showSubaccountPicker");
        }

        public void showAutoEnterPrompt(String account, String name) {
            calls.add("showAutoEnterPrompt:" + account);
        }

        public void showAutoLoginPrompt(String displayName) {
            calls.add("showAutoLoginPrompt");
        }

        public void showPickerNotice(String message) {
            calls.add("showPickerNotice");
        }

        public void showSessionCheck(String account, String maskedToken) {
            calls.add("showSessionCheck:" + account);
        }

        public void showFlowBlocked(String reason, String message) {
            calls.add("showFlowBlocked:" + reason);
        }

        java.util.Map<String, String> lastRoleFields;
        java.util.Map<String, String> lastPayDisplay;

        public void showRoleResult(boolean success, String reason, java.util.Map<String, String> fields) {
            calls.add("showRoleResult:" + success);
            lastRoleFields = fields;
        }

        public void showPayDrawer(java.util.Map<String, String> orderDisplay, String paymentUrl) {
            calls.add("showPayDrawer");
            lastPayDisplay = orderDisplay;
        }

        public void showFloatBall(String account, String userCenterUrl, String platformToken) {
            calls.add("showFloatBall:" + account);
        }

        public void hideFloatBall() {
            calls.add("hideFloatBall");
        }

        public void showDeviceVerify(String loginAccount) {
            calls.add("showDeviceVerify:" + loginAccount);
        }
    }

    static final class RecordingListener implements UserListener {
        boolean logoutCalled;

        public void onLogout() {
            logoutCalled = true;
        }
    }
}
