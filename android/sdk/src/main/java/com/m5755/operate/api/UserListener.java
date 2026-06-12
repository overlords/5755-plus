package com.m5755.operate.api;

/**
 * 账号变化监听(公开 API 白名单)。退出登录、踢号、账户失效、切换游戏小号统一收敛到此。
 * 维护门禁、协议拒绝、防沉迷门禁阻断都不触发本回调。
 */
public interface UserListener {

    /** 账号登出/失效收敛入口。 */
    void onLogout();
}
