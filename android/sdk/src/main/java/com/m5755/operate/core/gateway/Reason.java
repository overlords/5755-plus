package com.m5755.operate.core.gateway;

/**
 * 平台服务端机器可读失败原因(04 §1.2.1),与 Go 端 internal/result 一一对应。
 * SDK 状态机据此分流回退动作,不解析 message 文本;不扩大公开 OperateCode。
 */
public final class Reason {

    public static final String MAINTENANCE = "maintenance";
    public static final String CREDENTIAL_INVALID = "credential_invalid";
    public static final String SMS_CODE_INVALID = "sms_code_invalid";
    public static final String SMS_CODE_EXPIRED = "sms_code_expired";
    public static final String SMS_RATE_LIMITED = "sms_rate_limited";
    public static final String PLATFORM_ACCOUNT_INVALID = "platform_account_invalid";
    public static final String SUBACCOUNT_INVALID = "subaccount_invalid";
    public static final String REAL_NAME_REQUIRED = "real_name_required";
    public static final String ANTI_ADDICTION_ENTRY_BLOCKED = "anti_addiction_entry_blocked";
    public static final String ANTI_ADDICTION_PAYMENT_BLOCKED = "anti_addiction_payment_blocked";
    public static final String SUBACCOUNT_LIMIT_REACHED = "subaccount_limit_reached";
    public static final String ORDER_INVALID = "order_invalid";
    public static final String PARAM_INVALID = "param_invalid";
    public static final String SIGNATURE_INVALID = "signature_invalid";
    public static final String TIMESTAMP_EXPIRED = "timestamp_expired";
    public static final String PLATFORM_UNAVAILABLE = "platform_unavailable";
    public static final String DEVICE_VERIFICATION_REQUIRED = "device_verification_required";

    private Reason() {
    }
}
