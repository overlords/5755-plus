package com.m5755.operate.core.net;

import java.nio.charset.Charset;
import java.util.Arrays;
import java.util.Locale;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/**
 * 入站请求签名(04 §1.3),与平台服务端 Go 端 {@code internal/signature} 严格同构。
 * canonical = 方法\n路径\n字典序query\n时间戳\n请求体(GET 体为空串);HMAC-SHA256 十六进制小写。
 * 互通由跨端黄金向量测试钉死。
 */
public final class Signer {

    private static final Charset UTF8 = Charset.forName("UTF-8");

    public static final String HEADER_TIMESTAMP = "X-M5755-Timestamp";
    public static final String HEADER_KEY_ID = "X-M5755-Key-Id";
    public static final String HEADER_SIGNATURE = "X-M5755-Signature";

    /** 构造签名原文。rawQuery 在原始 token(k=v)层面按字典序排序;为空则该段为空串。 */
    public static String canonical(String method, String path, String rawQuery, String timestamp, String body) {
        String q = "";
        if (rawQuery != null && rawQuery.length() > 0) {
            String[] parts = rawQuery.split("&");
            Arrays.sort(parts);
            q = join(parts);
        }
        String b = (body == null) ? "" : body;
        return method.toUpperCase(Locale.ROOT) + "\n" + path + "\n" + q + "\n" + timestamp + "\n" + b;
    }

    /** HMAC-SHA256(secret, canonical),十六进制小写。 */
    public static String compute(String secret, String canonical) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(secret.getBytes(UTF8), "HmacSHA256"));
            return hex(mac.doFinal(canonical.getBytes(UTF8)));
        } catch (Exception e) {
            throw new IllegalStateException("签名计算失败", e);
        }
    }

    private static String join(String[] parts) {
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < parts.length; i++) {
            if (i > 0) {
                sb.append('&');
            }
            sb.append(parts[i]);
        }
        return sb.toString();
    }

    private static String hex(byte[] bytes) {
        StringBuilder sb = new StringBuilder(bytes.length * 2);
        for (byte x : bytes) {
            sb.append(Character.forDigit((x >> 4) & 0xF, 16));
            sb.append(Character.forDigit(x & 0xF, 16));
        }
        return sb.toString();
    }

    private Signer() {
    }
}
