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

    /** 初始化:配置拉取,成功后才允许后续业务 API(01/03)。 */
    public static void init(final Activity activity, final Options options, final Listener listener) {
        final PlatformConfig cfg = AssetPlatformConfig.load(activity);
        android.util.Log.i("M5755Sdk", "init platformEnv=" + cfg.platformEnv
                + " baseHost=" + cfg.baseUrl.replaceFirst("^https?://", "")
                + " gameId=" + options.getGameId()
                + " artifactType=" + cfg.artifactType);
        gameId = options.getGameId();
        storage = new SharedPrefsStorage(activity);
        ui = new SdkUi(activity, background);
        controller = new ColdStartController(new HttpPlatformGateway(cfg), storage, ui, dispatchUserListener());
        // 渠道两源解析(M4 渠道三件套):诊断型,异常回退 default 不阻断;真实值进 login/config 请求
        com.m5755.operate.core.channel.ChannelRules.Result ch =
                com.m5755.operate.core.channel.ChannelResolver.resolve(activity);
        controller.setChannel(ch.resolved, ch.source);
        ui.setController(controller);
        warnIfNoConfigChanges(activity); // #44 接入自检(诊断型):未声明 configChanges 旋转会重建丢浮层
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

    /**
     * #44 接入自检(诊断型,不阻断):游戏 Activity 未声明 android:configChanges="orientation|screenSize" 时,
     * 旋转会重建 Activity、SDK 浮层与输入/勾选/倒计时态会丢失。锁定方向的游戏可忽略(ADR-0009:开屏即定、
     * 不实现旋转实时切换);允许旋转的游戏应声明该属性。
     */
    private static void warnIfNoConfigChanges(Activity activity) {
        try {
            android.content.pm.ActivityInfo ai = activity.getPackageManager()
                    .getActivityInfo(activity.getComponentName(), 0);
            int cc = ai.configChanges;
            boolean orient = (cc & android.content.pm.ActivityInfo.CONFIG_ORIENTATION) != 0;
            boolean size = (cc & android.content.pm.ActivityInfo.CONFIG_SCREEN_SIZE) != 0;
            if (!orient || !size) {
                android.util.Log.w("M5755Sdk", "接入自检:游戏 Activity 未声明 android:configChanges=\"orientation|screenSize\""
                        + ";旋转会重建 Activity、SDK 浮层与表单态可能丢失。若游戏允许旋转请声明该属性(ADR-0009;锁定方向的游戏可忽略)。");
            }
        } catch (Exception e) {
            // 诊断型:取不到 ActivityInfo 忽略,不阻断 init
        }
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

    /** #27 角色上报。 */
    public static void sendRoleInfo(RoleInfo roleInfo) {
        sendRoleInfo(roleInfo, null);
    }

    public static void sendRoleInfo(final RoleInfo roleInfo, final Listener listener) {
        if (controller == null) {
            if (listener != null) {
                listener.onResult(false, OperateCode.NOT_INITIALIZED, "未初始化");
            }
            return;
        }
        background.execute(new Runnable() {
            public void run() {
                controller.reportRole(roleInfo, listener);
            }
        });
    }

    /** #28 支付。 */
    public static void recharge(Activity activity, final Order order, final Listener listener) {
        if (controller == null) {
            if (listener != null) {
                listener.onResult(false, OperateCode.NOT_INITIALIZED, "未初始化");
            }
            return;
        }
        background.execute(new Runnable() {
            public void run() {
                controller.recharge(order, listener);
            }
        });
    }

    /** #30 退出游戏确认。 */
    public static void shouldQuitGame(Activity activity, final OnQuitGameListener listener) {
        if (ui == null) {
            if (listener != null) {
                listener.onQuit();
            }
            return;
        }
        ui.showQuitConfirm(new Runnable() {
            public void run() {
                if (listener != null) {
                    listener.onQuit();
                }
            }
        }, new Runnable() {
            public void run() {
                if (listener != null) {
                    listener.onCancel();
                }
            }
        });
    }

    /** #30 销毁:释放 SDK 资源,不改变账号状态(03 §6)。 */
    public static void destroy(Activity activity) {
        if (ui != null) {
            ui.hideFloatBall();
        }
        inited = false;
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
