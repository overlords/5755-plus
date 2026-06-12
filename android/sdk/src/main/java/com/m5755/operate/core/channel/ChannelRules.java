package com.m5755.operate.core.channel;

import java.util.regex.Pattern;

/**
 * 渠道标识符规则与双源决策(01 §6,纯逻辑 JVM 可测)。
 * 诊断五字段:manifestChannelRaw / signingBlockChannelRaw / resolvedChannel / channelSource / reason。
 */
public final class ChannelRules {

    public static final String DEFAULT = "default";

    public static final String SOURCE_MANIFEST = "manifest";
    public static final String SOURCE_SIGNING_BLOCK = "signing_block";
    public static final String SOURCE_DEFAULT = "default";

    public static final String REASON_MISSING = "missing";
    public static final String REASON_UNREADABLE = "unreadable";
    public static final String REASON_INVALID_FORMAT = "invalid_format";
    public static final String REASON_SOURCE_MISMATCH = "source_mismatch";

    private static final Pattern VALID = Pattern.compile("^[a-z0-9_.-]{1,64}$");

    public static final class Result {
        public final String manifestRaw;
        public final String signingBlockRaw;
        public final String resolved;
        public final String source;
        public final String reason; // 成功解析真实渠道时为 ""

        Result(String mRaw, String sRaw, String resolved, String source, String reason) {
            this.manifestRaw = mRaw;
            this.signingBlockRaw = sRaw;
            this.resolved = resolved;
            this.source = source;
            this.reason = reason;
        }
    }

    /** 归一化:trim + 小写;非法返回 null。 */
    static String normalize(String raw) {
        if (raw == null) {
            return null;
        }
        String v = raw.trim().toLowerCase(java.util.Locale.ROOT);
        if (!VALID.matcher(v).matches()) {
            return null;
        }
        return v;
    }

    /**
     * 双源决策(01 §6):
     * 两源都在且归一后不一致 → default + source_mismatch;
     * 一致或仅一源 → 取该值(块来源优先标记 signing_block);
     * 任一源原值存在但非法 → invalid_format;两源皆缺 → missing。
     * unreadable 由调用方在读取异常时传入。
     */
    public static Result decide(String manifestRaw, String blockRaw, boolean blockUnreadable) {
        String m = normalize(manifestRaw);
        String b = normalize(blockRaw);
        if (blockUnreadable && manifestRaw == null) {
            return new Result(null, null, DEFAULT, SOURCE_DEFAULT, REASON_UNREADABLE);
        }
        if (m != null && b != null) {
            if (!m.equals(b)) {
                return new Result(manifestRaw, blockRaw, DEFAULT, SOURCE_DEFAULT, REASON_SOURCE_MISMATCH);
            }
            return new Result(manifestRaw, blockRaw, b, SOURCE_SIGNING_BLOCK, "");
        }
        if (b != null) {
            return new Result(manifestRaw, blockRaw, b, SOURCE_SIGNING_BLOCK, "");
        }
        if (m != null) {
            return new Result(manifestRaw, blockRaw, m, SOURCE_MANIFEST, "");
        }
        // 没有有效值:区分"有原值但非法"与"全缺"
        if (manifestRaw != null || blockRaw != null) {
            return new Result(manifestRaw, blockRaw, DEFAULT, SOURCE_DEFAULT, REASON_INVALID_FORMAT);
        }
        return new Result(null, null, DEFAULT, SOURCE_DEFAULT, REASON_MISSING);
    }

    private ChannelRules() {
    }
}
