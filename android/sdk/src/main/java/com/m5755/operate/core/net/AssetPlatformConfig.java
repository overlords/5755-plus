package com.m5755.operate.core.net;

import android.content.Context;

import java.io.InputStream;
import java.util.Properties;

/**
 * 从 AAR 包内 {@code assets/m5755-sdk-platform.properties} 加载 {@link PlatformConfig}(04 §6)。
 * 裸 baseHost 运行时补 {@code https://};已含 scheme 则原样(便于本地/联调指向)。
 */
public final class AssetPlatformConfig {

    private static final String ASSET = "m5755-sdk-platform.properties";

    public static PlatformConfig load(Context context) {
        Properties p = new Properties();
        InputStream is = null;
        try {
            is = context.getAssets().open(ASSET);
            p.load(is);
        } catch (Exception e) {
            throw new IllegalStateException("缺少或无法读取 " + ASSET, e);
        } finally {
            closeQuietly(is);
        }
        String baseHost = p.getProperty("baseHost", "");
        String baseUrl = baseHost.contains("://") ? baseHost : "https://" + baseHost;
        return new PlatformConfig(
                baseUrl,
                p.getProperty("artifactType", "integration"),
                p.getProperty("platformEnv", "dev"),
                p.getProperty("keyId", ""),
                p.getProperty("signatureSecret", ""),
                p.getProperty("sdkVersion", "1.0.0"));
    }

    private static void closeQuietly(InputStream is) {
        if (is != null) {
            try {
                is.close();
            } catch (Exception ignored) {
            }
        }
    }

    private AssetPlatformConfig() {
    }
}
