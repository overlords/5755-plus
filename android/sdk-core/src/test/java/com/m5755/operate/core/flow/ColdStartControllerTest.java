package com.m5755.operate.core.flow;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNull;
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
 * 冷启动状态机的纯逻辑验证(无 Android 依赖),覆盖 #7/#8/#9 的关键规则。
 * 上线阻断回归仍以仪器化测试为准(ADR-0002);此处为快速反馈的开发期单测。
 */
public class ColdStartControllerTest {

    // ---- 维护门禁阻断不触发账号变化(#9 核心) ----
    @Test
    public void maintenanceBlocksEntryWithoutAccountChange() {
        FakeGateway gw = new FakeGateway();
        gw.config.ok = true;
        gw.config.maintenanceEnabled = true;
        gw.config.maintenanceMessage = "维护中";
        RecordingUi ui = new RecordingUi();
        RecordingListener listener = new RecordingListener();
        ColdStartController c = new ColdStartController(gw, new FakeStorage(), ui, listener);

        c.start("m5755-demo", "1.0.0", "com.x");

        assertTrue("应展示维护提示", ui.calls.contains("showMaintenance:维护中"));
        assertFalse("维护时不应进入协议", ui.calls.contains("showProtocol"));
        assertFalse("维护时不应进入登录窗口", ui.calls.contains("showLoginWindow"));
        assertFalse("维护阻断绝不触发账号变化", listener.logoutCalled);
    }

    // ---- init 阻断型失败时不进入协议或登录 ----
    @Test
    public void initFailureBlocksBeforeProtocol() {
        FakeGateway gw = new FakeGateway();
        gw.config.ok = false;
        gw.config.reason = "param_invalid";
        RecordingUi ui = new RecordingUi();
        ColdStartController c = new ColdStartController(gw, new FakeStorage(), ui, new RecordingListener());

        c.start("nope", "1.0.0", "com.x");

        assertTrue(ui.calls.contains("showInitError:param_invalid"));
        assertFalse(ui.calls.contains("showProtocol"));
        assertFalse(ui.calls.contains("showLoginWindow"));
    }

    // ---- 全新安装:展示协议,同意后进入登录窗口 ----
    @Test
    public void freshInstallShowsProtocolThenLogin() {
        FakeGateway gw = new FakeGateway();
        gw.config.ok = true;
        gw.config.protocolVersion = "1";
        RecordingUi ui = new RecordingUi();
        FakeStorage store = new FakeStorage();
        ColdStartController c = new ColdStartController(gw, store, ui, new RecordingListener());

        c.start("m5755-demo", "1.0.0", "com.x");
        assertTrue(ui.calls.contains("showProtocol:1"));
        assertFalse("未同意前不进登录窗口", ui.calls.contains("showLoginWindow"));

        c.onProtocolConsented();
        assertTrue("已同意应记录", store.isProtocolConsented("1"));
        assertTrue(ui.calls.contains("showLoginWindow"));
    }

    // ---- 已同意协议:直接到达登录窗口,不重复展示协议 ----
    @Test
    public void consentedInstallReachesLoginDirectly() {
        FakeGateway gw = new FakeGateway();
        gw.config.ok = true;
        gw.config.protocolVersion = "1";
        FakeStorage store = new FakeStorage();
        store.setProtocolConsented("1");
        RecordingUi ui = new RecordingUi();
        ColdStartController c = new ColdStartController(gw, store, ui, new RecordingListener());

        c.start("m5755-demo", "1.0.0", "com.x");

        assertFalse("同一安装内不重复协议告知", ui.calls.contains("showProtocol:1"));
        assertTrue(ui.calls.contains("showLoginWindow"));
    }

    // ---- 协议拒绝:阻断进入,不触发账号变化 ----
    @Test
    public void protocolRejectDoesNotFireAccountChange() {
        RecordingUi ui = new RecordingUi();
        RecordingListener listener = new RecordingListener();
        ColdStartController c = new ColdStartController(new FakeGateway(), new FakeStorage(), ui, listener);

        c.onProtocolRejected();

        assertTrue(ui.calls.contains("onEntryBlockedByProtocolReject"));
        assertFalse("协议拒绝不触发账号变化", listener.logoutCalled);
    }

    // ---- devCode 登录成功:保存会话并回调成功(#8) ----
    @Test
    public void loginSuccessSavesSession() {
        FakeGateway gw = new FakeGateway();
        gw.login.ok = true;
        gw.login.platformAccountId = "pa_1";
        gw.login.platformToken = "pt_1";
        gw.login.firstAccount = "sub_1";
        RecordingUi ui = new RecordingUi();
        FakeStorage store = new FakeStorage();
        ColdStartController c = new ColdStartController(gw, store, ui, new RecordingListener());

        c.submitLogin("13800000000", "123456");

        assertTrue(ui.calls.contains("onLoginSuccess"));
        assertTrue("登录成功应保存会话", store.hasSession());
    }

    // ---- 验证码错误:提示并允许重试,不保存会话(#8) ----
    @Test
    public void invalidCodeShowsErrorNoSession() {
        FakeGateway gw = new FakeGateway();
        gw.login.ok = false;
        gw.login.reason = "sms_code_invalid";
        RecordingUi ui = new RecordingUi();
        FakeStorage store = new FakeStorage();
        ColdStartController c = new ColdStartController(gw, store, ui, new RecordingListener());

        c.submitLogin("13800000000", "000000");

        assertTrue(ui.calls.contains("showLoginError:sms_code_invalid"));
        assertFalse("登录失败不应保存会话", store.hasSession());
    }

    // ===== 内存假实现 =====

    static final class FakeGateway implements PlatformGateway {
        final Results.Config config = new Results.Config();
        final Results.Sms sms = new Results.Sms();
        final Results.Login login = new Results.Login();

        @Override
        public Results.Config fetchConfig(String g, String v, String p, String c, String s) {
            return config;
        }

        @Override
        public Results.Sms requestSms(String g, String a) {
            return sms;
        }

        @Override
        public Results.Login login(String g, String a, String cr, String c, String s) {
            return login;
        }
    }

    static final class FakeStorage implements Storage {
        private final Set<String> consented = new HashSet<>();
        private boolean session;

        @Override
        public boolean isProtocolConsented(String v) {
            return consented.contains(v);
        }

        @Override
        public void setProtocolConsented(String v) {
            consented.add(v);
        }

        @Override
        public boolean hasSession() {
            return session;
        }

        @Override
        public void saveSession(String pa, String pt, String account) {
            session = true;
        }

        @Override
        public void clearSession() {
            session = false;
        }
    }

    static final class RecordingUi implements FlowUi {
        final List<String> calls = new ArrayList<>();

        @Override
        public void showInitError(String reason, String message) {
            calls.add("showInitError:" + reason);
        }

        @Override
        public void showMaintenance(String message) {
            calls.add("showMaintenance:" + message);
        }

        @Override
        public void showProtocol(String protocolVersion) {
            calls.add("showProtocol:" + protocolVersion);
        }

        @Override
        public void showLoginWindow() {
            calls.add("showLoginWindow");
        }

        @Override
        public void onSmsRequested(Results.Sms result) {
            calls.add("onSmsRequested");
        }

        @Override
        public void showLoginError(String reason, String message) {
            calls.add("showLoginError:" + reason);
        }

        @Override
        public void onLoginSuccess(Results.Login result) {
            calls.add("onLoginSuccess");
        }

        @Override
        public void onEntryBlockedByProtocolReject() {
            calls.add("onEntryBlockedByProtocolReject");
        }
    }

    static final class RecordingListener implements UserListener {
        boolean logoutCalled;

        @Override
        public void onLogout() {
            logoutCalled = true;
        }
    }
}
