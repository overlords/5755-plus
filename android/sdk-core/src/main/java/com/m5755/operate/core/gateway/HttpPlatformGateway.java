package com.m5755.operate.core.gateway;

import com.m5755.operate.core.net.PlatformConfig;
import com.m5755.operate.core.net.Signer;

import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.net.URLEncoder;
import java.nio.charset.Charset;
import java.util.ArrayList;
import java.util.List;

/**
 * {@link PlatformGateway} 的真实实现:HttpURLConnection + {@link Signer} 签名 + org.json 解析。
 * 不引入第三方网络/JSON 库(01 依赖白名单)。所有方法阻塞,由上层在后台线程执行。
 * 失败统一归一为带 {@code reason} 的结果;网络/解析异常归 {@code platform_unavailable}。
 */
public final class HttpPlatformGateway implements PlatformGateway {

    private static final Charset UTF8 = Charset.forName("UTF-8");
    private static final int CONNECT_TIMEOUT_MS = 5000;
    private static final int READ_TIMEOUT_MS = 5000;

    private final PlatformConfig config;

    public HttpPlatformGateway(PlatformConfig config) {
        this.config = config;
    }

    @Override
    public Results.Config fetchConfig(String gameId, String sdkVersion, String packageName,
                                      String channelId, String channelSource) {
        Results.Config out = new Results.Config();
        try {
            String query = query(
                    "gameId", gameId,
                    "sdkVersion", sdkVersion,
                    "packageName", packageName,
                    "channelId", channelId,
                    "channelSource", channelSource);
            Response r = call("GET", "/api/sdk/v2/config", query, null);
            out.ok = r.success;
            out.reason = r.reason;
            out.message = r.message;
            if (r.data != null) {
                JSONObject m = r.data.optJSONObject("maintenance");
                if (m != null) {
                    out.maintenanceEnabled = m.optBoolean("enabled", false);
                    out.maintenanceMessage = m.optString("message", "");
                }
                out.antiAddictionEntryBlocked = r.data.optBoolean("antiAddictionEntryBlocked", false);
                out.antiAddictionPaymentBlocked = r.data.optBoolean("antiAddictionPaymentBlocked", false);
                out.protocolVersion = r.data.optString("protocolVersion", "");
                out.updateRequired = r.data.optBoolean("updateRequired", false);
                out.configVersion = r.data.optString("configVersion", "");
                out.requestId = r.data.optString("requestId", "");
            }
        } catch (Exception e) {
            out.ok = false;
            out.reason = Reason.PLATFORM_UNAVAILABLE;
            out.message = "网络或解析失败";
        }
        return out;
    }

    @Override
    public Results.Sms requestSms(String gameId, String loginAccount) {
        Results.Sms out = new Results.Sms();
        try {
            JSONObject body = new JSONObject();
            body.put("gameId", gameId);
            body.put("loginAccount", loginAccount);
            Response r = call("POST", "/api/sdk/v2/sms-codes", "", body.toString());
            out.ok = r.success;
            out.reason = r.reason;
            out.message = r.message;
            if (r.data != null) {
                out.codeId = r.data.optString("codeId", "");
                out.loginAccountMasked = r.data.optString("loginAccountMasked", "");
                out.devCode = r.data.isNull("devCode") ? null : r.data.optString("devCode", null);
            }
        } catch (Exception e) {
            out.ok = false;
            out.reason = Reason.PLATFORM_UNAVAILABLE;
            out.message = "网络或解析失败";
        }
        return out;
    }

    @Override
    public Results.Login login(String gameId, String loginAccount, String credential,
                               String channelId, String channelSource) {
        Results.Login out = new Results.Login();
        try {
            JSONObject body = new JSONObject();
            body.put("gameId", gameId);
            body.put("loginMethod", "sms");
            body.put("loginAccount", loginAccount);
            body.put("credential", credential);
            body.put("channelId", channelId);
            body.put("channelSource", channelSource);
            Response r = call("POST", "/api/sdk/v2/account-sessions", "", body.toString());
            out.ok = r.success;
            out.reason = r.reason;
            out.message = r.message;
            if (r.data != null) {
                out.platformAccountId = r.data.optString("platformAccountId", "");
                out.platformToken = r.data.optString("platformToken", "");
                out.displayName = r.data.optString("displayName", "");
                JSONObject ge = r.data.optJSONObject("gameEntry");
                if (ge != null) {
                    out.isNewGameUser = ge.optBoolean("isNewGameUser", false);
                    JSONObject created = ge.optJSONObject("createdSubaccount");
                    if (created != null) {
                        out.firstAccount = created.optString("account", "");
                    }
                }
            }
        } catch (Exception e) {
            out.ok = false;
            out.reason = Reason.PLATFORM_UNAVAILABLE;
            out.message = "网络或解析失败";
        }
        return out;
    }

    // ---- 内部 ----

    private static final class Response {
        boolean success;
        String reason;
        String message;
        JSONObject data;
    }

    private Response call(String method, String path, String query, String body) throws IOException {
        long ts = System.currentTimeMillis() / 1000L;
        byte[] bodyBytes = body == null ? null : body.getBytes(UTF8);
        String canonical = Signer.canonical(method, path, query, String.valueOf(ts), body);
        String sig = Signer.compute(config.signatureSecret, canonical);

        String urlStr = config.baseUrl + path + (query.isEmpty() ? "" : "?" + query);
        HttpURLConnection conn = (HttpURLConnection) new URL(urlStr).openConnection();
        conn.setConnectTimeout(CONNECT_TIMEOUT_MS);
        conn.setReadTimeout(READ_TIMEOUT_MS);
        conn.setRequestMethod(method);
        conn.setRequestProperty("Accept", "application/json");
        conn.setRequestProperty(Signer.HEADER_TIMESTAMP, String.valueOf(ts));
        conn.setRequestProperty(Signer.HEADER_KEY_ID, config.keyId);
        conn.setRequestProperty(Signer.HEADER_SIGNATURE, sig);
        conn.setRequestProperty("X-M5755-Artifact-Type", config.artifactType);
        conn.setRequestProperty("X-M5755-Platform-Env", config.platformEnv);
        if (bodyBytes != null) {
            conn.setDoOutput(true);
            conn.setRequestProperty("Content-Type", "application/json; charset=UTF-8");
            OutputStream os = conn.getOutputStream();
            os.write(bodyBytes);
            os.close();
        }

        int code = conn.getResponseCode();
        InputStream is = (code >= 200 && code < 300) ? conn.getInputStream() : conn.getErrorStream();
        String respBody = readAll(is);
        conn.disconnect();

        Response r = new Response();
        if (respBody != null && respBody.length() > 0) {
            try {
                JSONObject root = new JSONObject(respBody);
                r.success = root.optBoolean("success", false);
                r.reason = root.isNull("reason") ? null : root.optString("reason", null);
                r.message = root.optString("message", "");
                r.data = root.optJSONObject("data");
            } catch (Exception parse) {
                r.success = false;
                r.reason = Reason.PLATFORM_UNAVAILABLE;
                r.message = "响应不可解析";
            }
        } else {
            r.success = false;
            r.reason = Reason.PLATFORM_UNAVAILABLE;
            r.message = "空响应";
        }
        return r;
    }

    private static String readAll(InputStream is) throws IOException {
        if (is == null) {
            return null;
        }
        ByteArrayOutputStream bos = new ByteArrayOutputStream();
        byte[] buf = new byte[4096];
        int n;
        while ((n = is.read(buf)) != -1) {
            bos.write(buf, 0, n);
        }
        is.close();
        return new String(bos.toByteArray(), UTF8);
    }

    private static String query(String... kv) {
        List<String> parts = new ArrayList<>();
        for (int i = 0; i + 1 < kv.length; i += 2) {
            parts.add(enc(kv[i]) + "=" + enc(kv[i + 1]));
        }
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < parts.size(); i++) {
            if (i > 0) {
                sb.append('&');
            }
            sb.append(parts.get(i));
        }
        return sb.toString();
    }

    private static String enc(String s) {
        try {
            return URLEncoder.encode(s == null ? "" : s, "UTF-8");
        } catch (Exception e) {
            return s;
        }
    }
}
