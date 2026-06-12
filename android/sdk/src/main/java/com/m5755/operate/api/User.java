package com.m5755.operate.api;

import java.io.Serializable;

/**
 * 公开用户对象:`account` = 当前游戏小号 ID,`token` = 游戏小号登录令牌(仅来自小号登录接口)。
 * 接入方用两者完成服务端登录态校验、角色上报与支付归属。
 */
public final class User implements Serializable {

    private static final long serialVersionUID = 57550001L;

    private final String token;
    private final String account;

    public User(String token, String account) {
        this.token = token;
        this.account = account;
    }

    public String getToken() {
        return token;
    }

    public String getAccount() {
        return account;
    }

    @Override
    public String toString() {
        return "User{account=" + account + "}";
    }
}
