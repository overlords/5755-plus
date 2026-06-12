package com.m5755.operate.core.channel;

import android.content.Context;
import android.content.pm.ApplicationInfo;
import android.content.pm.PackageManager;
import android.os.Bundle;

/**
 * 渠道解析(Android 胶水):manifest meta-data 四键 + 自身 APK Signing Block 两 ID;
 * 决策与归一化在 {@link ChannelRules}(JVM 可测)。诊断型自检项:任何异常回退 default,不阻断。
 */
public final class ChannelResolver {

    /** manifest meta-data 键优先级(01 §6)。 */
    private static final String[] META_KEYS = {"m5755_channel", "m5755.channel", "com.m5755.channel", "channel"};

    public static ChannelRules.Result resolve(Context context) {
        String manifestRaw = null;
        try {
            ApplicationInfo ai = context.getPackageManager()
                    .getApplicationInfo(context.getPackageName(), PackageManager.GET_META_DATA);
            Bundle md = ai.metaData;
            if (md != null) {
                for (String k : META_KEYS) {
                    Object v = md.get(k);
                    if (v != null) {
                        manifestRaw = String.valueOf(v);
                        break;
                    }
                }
            }
        } catch (Exception ignored) {
            // manifest 不可读按缺失处理(诊断型)
        }

        String blockRaw = null;
        boolean blockUnreadable = false;
        try {
            String apk = context.getApplicationInfo().sourceDir;
            blockRaw = ApkSigningBlock.readChannel(apk);
        } catch (Exception e) {
            blockUnreadable = true;
        }

        ChannelRules.Result r = ChannelRules.decide(manifestRaw, blockRaw, blockUnreadable);
        android.util.Log.i("M5755Sdk", "channel resolvedChannel=" + r.resolved
                + " channelSource=" + r.source
                + " manifestChannelRaw=" + (r.manifestRaw == null ? "" : r.manifestRaw)
                + " signingBlockChannelRaw=" + (r.signingBlockRaw == null ? "" : r.signingBlockRaw)
                + (r.reason.isEmpty() ? "" : " reason=" + r.reason));
        return r;
    }

    private ChannelResolver() {
    }
}
