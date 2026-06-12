package com.m5755.operate.core.net;

/**
 * 平台接口运行配置,来自 AAR 包内 {@code assets/m5755-sdk-platform.properties}(04 §6),
 * 由构建/发布配置固定,接入游戏不能运行时切换。{@code signatureSecret} 在 dev 为公开测试密钥;
 * 生产由发布配置注入,不随源码明文交付。
 */
public final class PlatformConfig {

    public final String baseUrl;       // 含 scheme,如 https://sdk-dev.xingninghuyu.com
    public final String artifactType;  // integration / production / local
    public final String platformEnv;   // dev / prod / local
    public final String keyId;
    public final String signatureSecret;
    public final String sdkVersion;

    public PlatformConfig(String baseUrl, String artifactType, String platformEnv,
                          String keyId, String signatureSecret, String sdkVersion) {
        this.baseUrl = baseUrl;
        this.artifactType = artifactType;
        this.platformEnv = platformEnv;
        this.keyId = keyId;
        this.signatureSecret = signatureSecret;
        this.sdkVersion = sdkVersion;
    }
}
