package com.m5755.operate.provider;

/**
 * 公开错误码白名单。SDK 对外只暴露这组粗粒度码;平台服务端的细分 reason 在 SDK 内部
 * 用于状态机分流,不扩大公开码集合(04 §1.2)。
 */
public final class OperateCode {

    /** 成功。 */
    public static final int SUCCESS = 0;

    /** 通用失败。 */
    public static final int FAILURE = 3;

    /** 未初始化或初始化未完成。 */
    public static final int NOT_INITIALIZED = 6;

    /** 网络错误。 */
    public static final int NETWORK_ERROR = 7;

    /** 请求超时。 */
    public static final int TIMEOUT = 8;

    /** 用户取消。 */
    public static final int CANCELED = 9;

    /** 入参非法。 */
    public static final int PARAM_ERROR = 10;

    private OperateCode() {
    }
}
