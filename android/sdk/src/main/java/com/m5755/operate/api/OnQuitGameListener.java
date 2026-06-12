package com.m5755.operate.api;

/** 退出游戏确认回调(公开 API 白名单)。 */
public interface OnQuitGameListener {

    /** 玩家确认退出游戏。 */
    void onQuit();

    /** 玩家取消退出。 */
    void onCancel();
}
