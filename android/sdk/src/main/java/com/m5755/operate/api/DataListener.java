package com.m5755.operate.api;

/** 带数据的结果监听(公开 API 白名单)。 */
public interface DataListener<Data> {

    void onResult(boolean success, int code, String message, Data data);
}
