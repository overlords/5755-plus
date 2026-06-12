package com.m5755.operate.core.store;

/**
 * SDK 轻量键值存储边界:只保存协议同意状态、登录态标记等必要运行状态。
 * 真实实现基于 SharedPreferences;测试注入内存假实现。
 */
public interface Storage {

    /** 同一安装内、对应协议版本是否已同意(协议告知按安装级 + 版本级触发)。 */
    boolean isProtocolConsented(String protocolVersion);

    void setProtocolConsented(String protocolVersion);

    /** 是否有可用于发起自动登录的本地会话。 */
    boolean hasSession();

    void saveSession(String platformAccountId, String platformToken, String account);

    void clearSession();
}
