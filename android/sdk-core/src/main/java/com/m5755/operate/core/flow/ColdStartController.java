package com.m5755.operate.core.flow;

import com.m5755.operate.api.UserListener;
import com.m5755.operate.core.gateway.PlatformGateway;
import com.m5755.operate.core.gateway.Results;
import com.m5755.operate.core.store.Storage;

/**
 * 冷启动状态机(里程碑 1 切片):onGameStart → init → 维护门禁 → 协议告知 → 5755 账户登录窗口,
 * 并覆盖验证码请求与账户登录提交。状态顺序与阻断/回退规则以 03 文档为准。
 *
 * <p>关键不变量(可由 JVM 单测验证):
 * <ul>
 *   <li>维护门禁阻断只展示维护提示,<b>绝不</b>调用 {@link UserListener#onLogout()}(不触发账号变化,03 §2.2);
 *   <li>协议拒绝阻断进入流程,不杀进程、不触发账号变化(03 §2.3);
 *   <li>init 阻断型失败时不进入协议告知或登录;
 *   <li>登录失败按 {@code reason} 提示并允许重试,不保存会话。
 * </ul>
 * 网关调用为阻塞式,由上层 facade 在后台线程执行;本类只表达状态迁移。
 */
public final class ColdStartController {

    private final PlatformGateway gateway;
    private final Storage storage;
    private final FlowUi ui;
    private final UserListener listener;

    private String gameId;
    private String channelId = "default";
    private String channelSource = "manifest";
    private String protocolVersion;

    public ColdStartController(PlatformGateway gateway, Storage storage, FlowUi ui, UserListener listener) {
        this.gateway = gateway;
        this.storage = storage;
        this.ui = ui;
        this.listener = listener;
    }

    public void setChannel(String channelId, String channelSource) {
        this.channelId = channelId;
        this.channelSource = channelSource;
    }

    /** 冷启动入口。init 成功前不进入协议告知或登录。 */
    public void start(String gameId, String sdkVersion, String packageName) {
        this.gameId = gameId;
        Results.Config cfg = gateway.fetchConfig(gameId, sdkVersion, packageName, channelId, channelSource);
        if (!cfg.ok) {
            ui.showInitError(cfg.reason, cfg.message);
            return;
        }
        if (cfg.maintenanceEnabled) {
            // 维护阻断:展示提示并阻断进入;不触发账号变化(不调用 listener)。
            ui.showMaintenance(cfg.maintenanceMessage);
            return;
        }
        this.protocolVersion = cfg.protocolVersion;
        if (!storage.isProtocolConsented(cfg.protocolVersion)) {
            ui.showProtocol(cfg.protocolVersion);
            return;
        }
        // 里程碑 1:全新安装无本地会话,直接到达登录窗口。
        ui.showLoginWindow();
    }

    /** 玩家同意协议:记录同意并进入登录窗口。 */
    public void onProtocolConsented() {
        storage.setProtocolConsented(protocolVersion);
        ui.showLoginWindow();
    }

    /** 玩家拒绝协议:阻断进入流程,不杀进程、不触发账号变化。 */
    public void onProtocolRejected() {
        ui.onEntryBlockedByProtocolReject();
    }

    /** 登录窗口请求短信验证码。 */
    public void requestCode(String phone) {
        Results.Sms r = gateway.requestSms(gameId, phone);
        if (!r.ok) {
            ui.showLoginError(r.reason, r.message);
            return;
        }
        ui.onSmsRequested(r);
    }

    /** 提交验证码登录。成功保存会话并回调成功;失败按 reason 提示并允许重试,不保存会话。 */
    public void submitLogin(String phone, String code) {
        Results.Login r = gateway.login(gameId, phone, code, channelId, channelSource);
        if (!r.ok) {
            ui.showLoginError(r.reason, r.message);
            return;
        }
        storage.saveSession(r.platformAccountId, r.platformToken, r.firstAccount);
        ui.onLoginSuccess(r);
    }
}
