package com.m5755.operate.core.flow;

import com.m5755.operate.core.gateway.Results;

/**
 * 冷启动状态机驱动的 UI 回调边界。业务配套 UI(sdk-ui)实现本接口;
 * 核心状态机只通过它表达「该展示什么」,不直接依赖 Android 视图。
 */
public interface FlowUi {

    /** 初始化失败(阻断型):展示诊断并阻断进入。 */
    void showInitError(String reason, String message);

    /** 维护门禁阻断:展示维护提示并阻断进入流程(不触发账号变化)。 */
    void showMaintenance(String message);

    /** 协议告知:新安装或协议版本升级时展示。 */
    void showProtocol(String protocolVersion);

    /** 到达 5755 账户登录窗口。 */
    void showLoginWindow();

    /** 验证码已请求(dev/mock 下可据 devCode 联调提示;不记日志)。 */
    void onSmsRequested(Results.Sms result);

    /** 登录失败:按 reason 提示并允许重试。 */
    void showLoginError(String reason, String message);

    /** 5755 账户登录成功(里程碑 1 终点;下游实名/小号选择属后续)。 */
    void onLoginSuccess(Results.Login result);

    /** 协议被拒绝:阻断进入流程(不杀进程、不触发账号变化)。 */
    void onEntryBlockedByProtocolReject();
}
