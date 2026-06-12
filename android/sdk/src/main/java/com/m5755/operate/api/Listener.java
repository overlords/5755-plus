package com.m5755.operate.api;

/** 通用结果监听(公开 API 白名单)。 */
public interface Listener {

    void onResult(boolean success, int code, String message);
}
