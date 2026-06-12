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

    /** 5755 账户登录成功(进入实名/小号阶段)。 */
    void onLoginSuccess(Results.Login result);

    /** 协议被拒绝:阻断进入流程(不杀进程、不触发账号变化)。 */
    void onEntryBlockedByProtocolReject();

    // ===== 里程碑 2(#16-#18) =====

    /** 实名认证页(07 §7):未实名账户必经。 */
    void showRealName();

    /** 实名提交失败提示(格式错误等,可重试)。 */
    void showRealNameError(String reason, String message);

    /** 防沉迷进入游戏门禁阻断:仅提示,不触发账号变化(03 §2.7)。 */
    void showAntiAddictionBlocked(String message);

    /** 游戏小号选择页(07 §5)。 */
    void showSubaccountPicker(Results.SubaccountList list, String accountNickname, boolean switchFlow);

    /** 默认小号自动进入提示(07 §6,1800ms)。 */
    void showAutoEnterPrompt(String account, String displayName);

    /** 选择页内操作的轻提示(上限/失败等)。 */
    void showPickerNotice(String message);

    /** 登录态校验弹窗(07 §10):展示真实(脱敏)令牌。 */
    void showSessionCheck(String account, String maskedToken);

    /** 登录链路不可恢复阻断(如小号列表为空的平台侧异常):提示 + 诊断。 */
    void showFlowBlocked(String reason, String message);
}
