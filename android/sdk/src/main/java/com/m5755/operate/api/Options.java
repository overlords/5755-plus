package com.m5755.operate.api;

/**
 * 初始化选项(公开 API 白名单)。最小字段集:游戏 ID。
 * 不提供 OAID/AndroidID 透传 key、环境切换或任何透传扩展(01 §3/§4 排除项)。
 */
public final class Options {

    private final String gameId;

    public Options(String gameId) {
        this.gameId = gameId;
    }

    public String getGameId() {
        return gameId;
    }
}
