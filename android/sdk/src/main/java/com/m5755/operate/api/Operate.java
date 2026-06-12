package com.m5755.operate.api;

import android.app.Activity;
import android.content.Context;

import com.m5755.operate.core.flow.ColdStartController;
import com.m5755.operate.core.gateway.HttpPlatformGateway;
import com.m5755.operate.core.net.AssetPlatformConfig;
import com.m5755.operate.core.net.PlatformConfig;
import com.m5755.operate.core.store.SharedPrefsStorage;
import com.m5755.operate.core.store.Storage;
import com.m5755.operate.provider.OperateCode;
import com.m5755.sdk.ui.SdkUi;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * 公开静态门面(01 §3 白名单;签名对齐旧实现)。业务配套 UI 由核心状态机驱动,
 * 接入方只通过本类触发流程。
 */
public final class Operate {

    private static final String VERSION = "2.0.0";

    private static final ExecutorService background = Executors.newSingleThreadExecutor();
    private static UserListener userListener;
    private static PlatformConfig configOverride;

    private static Storage storage;
    private static SdkUi ui;
    private static ColdStartController controller;
    private static String gameId;
    private static volatile boolean inited;
    private static volatile User currentUser;

    public static String getVersion() {
        return VERSION;
    }

    public static void onGameStart(Context context) {
        android.util.Log.i("M5755Sdk", "onGameStart");
    }

    public static void setUserListener(UserListener listener) {
        userListener = listener;
    }

    public static UserListener getUserListener() {
        return userListener;
    }

    /** 仅样例/测试:覆盖包内平台配置(指向本地服务端)。生产路径不调用。 */
    public static void setPlatformConfigOverride(PlatformConfig config) {
        configOverride = config;
    }

    /** 初始化:配置拉取,成功后才允许后续业务 API(01/03)。 */
    public static void init(final Activity activity, final Options options, final Listener listener) {
        final PlatformConfig cfg = configOverride != null ? configOverride : AssetPlatformConfig.load(activity);
        android.util.Log.i("M5755Sdk", "init platformEnv=" + cfg.platformEnv
                + " baseHost=" + cfg.baseUrl.replaceFirst("^https?://", "")
                + " gameId=" + options.getGameId()
                + " artifactType=" + cfg.artifactType);
        gameId = options.getGameId();
        storage = new SharedPrefsStorage(activity);
        ui = new SdkUi(activity, background);
        controller = new ColdStartController(new HttpPlatformGateway(cfg), storage, ui, dispatchUserListener());
        ui.setController(controller);
        background.execute(new Runnable() {
            public void run() {
                com.m5755.operate.core.gateway.Results.Config r =
                        controller.init(gameId, cfg.sdkVersion, activity.getPackageName());
                inited = r.ok;
                if (listener != null) {
                    listener.onResult(r.ok, r.ok ? OperateCode.SUCCESS : OperateCode.FAILURE,
                            r.ok ? "初始化成功" : safeMsg(r.message));
                }
            }
        });
    }

    /** 登录:SDK 内部处理协议/账户登录/实名/门禁/小号,最终回调 User(account+token)。 */
    public static void login(Activity activity, final DataListener<User> listener) {
        if (controller == null || !inited) {
            if (listener != null) {
                listener.onResult(false, OperateCode.NOT_INITIALIZED, "init 未成功", null);
            }
            return;
        }
        controller.setFlowListener(new ColdStartController.FlowListener() {
            public void onFlowSuccess(String account, String token) {
                currentUser = new User(token, account);
                if (listener != null) {
                    listener.onResult(true, OperateCode.SUCCESS, "登录成功", currentUser);
                }
            }

            public void onFlowCanceled() {
                if (listener != null) {
                    listener.onResult(false, OperateCode.CANCELED, "玩家取消", null);
                }
            }

            public void onFlowBlocked(String reason, String message) {
                if (listener != null) {
                    listener.onResult(false, OperateCode.FAILURE, safeMsg(message), null);
                }
            }
        });
        background.execute(new Runnable() {
            public void run() {
                controller.login();
            }
        });
    }

    public static boolean isLogin() {
        return currentUser != null;
    }

    public static User getUser() {
        return currentUser;
    }

    public static void logout() {
        if (controller == null) {
            return;
        }
        currentUser = null;
        background.execute(new Runnable() {
            public void run() {
                controller.logout();
            }
        });
    }

    /** 切换游戏小号:必经小号选择页显式选择;取消保持当前小号(03 §4.5)。 */
    public static void changeUser(Activity activity, final DataListener<User> listener) {
        if (controller == null || currentUser == null) {
            if (listener != null) {
                listener.onResult(false, OperateCode.FAILURE, "当前无游戏小号,不能切换", null);
            }
            return;
        }
        controller.setFlowListener(new ColdStartController.FlowListener() {
            public void onFlowSuccess(String account, String token) {
                currentUser = new User(token, account);
                if (listener != null) {
                    listener.onResult(true, OperateCode.SUCCESS, "切换成功", currentUser);
                }
            }

            public void onFlowCanceled() {
                if (listener != null) {
                    listener.onResult(false, OperateCode.CANCELED, "取消切换,保持当前小号", null);
                }
            }

            public void onFlowBlocked(String reason, String message) {
                if (listener != null) {
                    listener.onResult(false, OperateCode.FAILURE, safeMsg(message), null);
                }
            }
        });
        background.execute(new Runnable() {
            public void run() {
                controller.changeUser();
            }
        });
    }

    // ---- 内部 ----

    private static UserListener dispatchUserListener() {
        return new UserListener() {
            public void onLogout() {
                currentUser = null;
                UserListener l = userListener;
                if (l != null) {
                    l.onLogout();
                }
            }
        };
    }

    private static String safeMsg(String m) {
        return m == null ? "操作失败" : m;
    }

    private Operate() {
    }
}
