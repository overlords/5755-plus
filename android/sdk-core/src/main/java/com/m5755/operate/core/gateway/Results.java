package com.m5755.operate.core.gateway;

/**
 * 平台网关返回的最小结果模型(里程碑 1)。失败时 {@code reason} 携带 04 §1.2.1 机器可读枚举,
 * SDK 状态机据此分流,不解析 message。
 */
public final class Results {

    private Results() {
    }

    /** 初始化配置结果。 */
    public static final class Config {
        public boolean ok;
        public String reason;
        public String message;

        public boolean maintenanceEnabled;
        public String maintenanceMessage;
        public boolean antiAddictionEntryBlocked;
        public boolean antiAddictionPaymentBlocked;
        public String protocolVersion;
        public boolean updateRequired;
        public String configVersion;
        public String requestId;
    }

    /** 短信验证码请求结果。 */
    public static final class Sms {
        public boolean ok;
        public String reason;
        public String message;
        public String codeId;
        public String loginAccountMasked;
        /** 仅 mock 模式返回,供联调提示;不得进入诊断或日志。 */
        public String devCode;
    }

    /** 5755 账户登录结果。 */
    public static final class Login {
        public boolean ok;
        public String reason;
        public String message;
        public String platformAccountId;
        public String platformToken;
        public String displayName;
        public boolean isNewGameUser;
        /** 平台保障建档的首个游戏小号 ID(新用户)。 */
        public String firstAccount;
    }

    /** 账户有效检查结果(#15)。ok=接口成功;valid=登录态是否有效(语义分层)。 */
    public static final class AccountCheck {
        public boolean ok;
        public String reason;
        public String message;
        public boolean valid;
        public String displayName;
    }

    /** 实名状态/提交结果(#16)。 */
    public static final class RealName {
        public boolean ok;
        public String reason;
        public String message;
        public boolean verified;
        public boolean adult;
        public boolean entryBlocked;
        public boolean paymentBlocked;
    }

    /** 游戏小号列表(#17)。 */
    public static final class SubaccountList {
        public boolean ok;
        public String reason;
        public String message;
        public String defaultAccount;
        public java.util.List<Item> items = new java.util.ArrayList<Item>();

        public static final class Item {
            public String account;
            public String displayName;
            public boolean isDefault;
        }
    }

    /** 添加小号 / 设默认结果(#17)。 */
    public static final class SubaccountOp {
        public boolean ok;
        public String reason;
        public String message;
        public String account;
        public String displayName;
    }

    /** 小号登录结果(#18):account/token 只来自此处。 */
    public static final class SubaccountLogin {
        public boolean ok;
        public String reason;
        public String message;
        public String account;
        public String token;
    }
}
