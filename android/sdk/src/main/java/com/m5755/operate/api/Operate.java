package com.m5755.operate.api;

import android.app.Activity;

import com.m5755.operate.core.flow.ColdStartController;
import com.m5755.operate.core.gateway.HttpPlatformGateway;
import com.m5755.operate.core.gateway.PlatformGateway;
import com.m5755.operate.core.net.AssetPlatformConfig;
import com.m5755.operate.core.net.PlatformConfig;
import com.m5755.operate.core.store.SharedPrefsStorage;
import com.m5755.operate.core.store.Storage;
import com.m5755.sdk.ui.SdkUi;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * 公开门面(里程碑 1 子集)。装配平台网关、存储、业务配套 UI 与冷启动状态机,
 * 驱动 onGameStart → init → 维护门禁 → 协议告知 → 5755 账户登录窗口 → devCode 登录的切片。
 *
 * <p>里程碑 1 仅暴露 {@link #start} 与 {@link #setUserListener};完整公开 API 白名单
 * (init/login/getUser/recharge 等,01 §3)在后续里程碑补齐。
 */
public final class Operate {

    private static Operate instance;

    private final ExecutorService background = Executors.newSingleThreadExecutor();
    private UserListener userListener;
    private SdkUi ui;
    private ColdStartController controller;

    public static synchronized Operate get() {
        if (instance == null) {
            instance = new Operate();
        }
        return instance;
    }

    public void setUserListener(UserListener listener) {
        this.userListener = listener;
    }

    public UserListener getUserListener() {
        return userListener;
    }

    /** 以包内 assets 配置启动冷启动切片(联调默认对接 sdk-dev)。 */
    public void start(Activity activity, String gameId) {
        start(activity, gameId, AssetPlatformConfig.load(activity));
    }

    /** 以注入配置启动(测试/本地可指向 http://10.0.2.2:PORT 的本地服务端)。 */
    public void start(final Activity activity, final String gameId, final PlatformConfig config) {
        // 诊断快照(08 §2.2 验收口径):tag M5755Sdk;只输出非密字段,不输出令牌/验证码。
        android.util.Log.i("M5755Sdk", "init platformEnv=" + config.platformEnv
                + " baseHost=" + config.baseUrl.replaceFirst("^https?://", "")
                + " gameId=" + gameId
                + " artifactType=" + config.artifactType);
        PlatformGateway gateway = new HttpPlatformGateway(config);
        Storage storage = new SharedPrefsStorage(activity);
        this.ui = new SdkUi(activity, background);
        this.controller = new ColdStartController(gateway, storage, ui,
                userListener == null ? NOOP_LISTENER : userListener);
        ui.setController(controller);
        background.execute(new Runnable() {
            public void run() {
                controller.start(gameId, config.sdkVersion, activity.getPackageName());
            }
        });
    }

    private static final UserListener NOOP_LISTENER = new UserListener() {
        @Override
        public void onLogout() {
        }
    };

    private Operate() {
    }
}
