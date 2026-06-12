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
}
