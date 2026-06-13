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
                out.userCenterUrl = r.data.optString("userCenterUrl", "");
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
    public Results.Login loginPassword(String gameId, String loginAccount, String password,
                                       String deviceId, String deviceVerifyCode, String channelId, String channelSource) {
        try {
            JSONObject body = new JSONObject();
            body.put("gameId", gameId);
            body.put("loginMethod", "password");
            body.put("loginAccount", loginAccount);
            body.put("credential", password);
            body.put("deviceId", deviceId);
            if (deviceVerifyCode != null && !deviceVerifyCode.isEmpty()) {
                body.put("deviceVerifyCode", deviceVerifyCode);
            }
            body.put("channelId", channelId);
            body.put("channelSource", channelSource);
            return parseLogin(call("POST", "/api/sdk/v2/account-sessions", "", body.toString()));
        } catch (Exception e) {
            Results.Login out = new Results.Login();
            out.ok = false;
            out.reason = Reason.PLATFORM_UNAVAILABLE;
            out.message = "网络或解析失败";
            return out;
        }
    }

    private Results.Login parseLogin(Response r) {
        Results.Login out = new Results.Login();
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

    // ===== 里程碑 2 端点(#15-#18) =====

    private static final String H_PLATFORM_TOKEN = "X-M5755-Platform-Token";

    @Override
    public Results.AccountCheck checkAccount(String gameId, String platformAccountId, String platformToken) {
        Results.AccountCheck out = new Results.AccountCheck();
        try {
            String query = query("gameId", gameId, "platformAccountId", platformAccountId);
            Response r = callH("GET", "/api/sdk/v2/account-sessions", query, null, H_PLATFORM_TOKEN, platformToken);
            out.ok = r.success;
            out.reason = r.reason;
            out.message = r.message;
            if (r.data != null) {
                out.valid = r.data.optBoolean("valid", false);
                out.displayName = r.data.optString("displayName", "");
            }
        } catch (Exception e) {
            out.ok = false;
            out.reason = Reason.PLATFORM_UNAVAILABLE;
            out.message = "网络或解析失败";
        }
        return out;
    }

    private Results.RealName parseRealName(Response r) {
        Results.RealName out = new Results.RealName();
        out.ok = r.success;
        out.reason = r.reason;
        out.message = r.message;
        if (r.data != null) {
            out.verified = r.data.optBoolean("verified", false);
            out.adult = r.data.optBoolean("adult", false);
            out.entryBlocked = r.data.optBoolean("antiAddictionEntryBlocked", false);
            out.paymentBlocked = r.data.optBoolean("antiAddictionPaymentBlocked", false);
        }
        return out;
    }

    @Override
    public Results.RealName getRealName(String gameId, String platformAccountId, String platformToken) {
        try {
            String query = query("gameId", gameId, "platformAccountId", platformAccountId);
            return parseRealName(callH("GET", "/api/sdk/v2/real-name", query, null, H_PLATFORM_TOKEN, platformToken));
        } catch (Exception e) {
            return failRealName();
        }
    }

    @Override
    public Results.RealName submitRealName(String gameId, String platformAccountId, String platformToken,
                                           String realName, String idNumber) {
        try {
            JSONObject body = new JSONObject();
            body.put("gameId", gameId);
            body.put("platformAccountId", platformAccountId);
            body.put("platformToken", platformToken);
            body.put("realName", realName);
            body.put("idNumber", idNumber);
            return parseRealName(call("POST", "/api/sdk/v2/real-name", "", body.toString()));
        } catch (Exception e) {
            return failRealName();
        }
    }

    private static Results.RealName failRealName() {
        Results.RealName out = new Results.RealName();
        out.ok = false;
        out.reason = Reason.PLATFORM_UNAVAILABLE;
        out.message = "网络或解析失败";
        return out;
    }

    @Override
    public Results.SubaccountList listSubaccounts(String gameId, String platformAccountId, String platformToken) {
        Results.SubaccountList out = new Results.SubaccountList();
        try {
            String query = query("gameId", gameId, "platformAccountId", platformAccountId);
            Response r = callH("GET", "/api/sdk/v2/subaccounts", query, null, H_PLATFORM_TOKEN, platformToken);
            out.ok = r.success;
            out.reason = r.reason;
            out.message = r.message;
            if (r.data != null) {
                out.defaultAccount = r.data.isNull("defaultAccount") ? "" : r.data.optString("defaultAccount", "");
                org.json.JSONArray arr = r.data.optJSONArray("subaccounts");
                if (arr != null) {
                    for (int i = 0; i < arr.length(); i++) {
                        JSONObject o = arr.optJSONObject(i);
                        if (o == null) {
                            continue;
                        }
                        Results.SubaccountList.Item it = new Results.SubaccountList.Item();
                        it.account = o.optString("account", "");
                        it.displayName = o.optString("displayName", "");
                        it.isDefault = o.optBoolean("isDefault", false);
                        out.items.add(it);
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

    private Results.SubaccountOp subaccountOp(String method, String path, String gameId,
                                              String platformAccountId, String platformToken, String account) {
        Results.SubaccountOp out = new Results.SubaccountOp();
        try {
            JSONObject body = new JSONObject();
            body.put("gameId", gameId);
            body.put("platformAccountId", platformAccountId);
            body.put("platformToken", platformToken);
            if (account != null) {
                body.put("account", account);
            }
            Response r = call(method, path, "", body.toString());
            out.ok = r.success;
            out.reason = r.reason;
            out.message = r.message;
            if (r.data != null) {
                out.account = r.data.optString("account", "");
                out.displayName = r.data.optString("displayName", "");
            }
        } catch (Exception e) {
            out.ok = false;
            out.reason = Reason.PLATFORM_UNAVAILABLE;
            out.message = "网络或解析失败";
        }
        return out;
    }

    @Override
    public Results.SubaccountOp createSubaccount(String gameId, String platformAccountId, String platformToken) {
        return subaccountOp("POST", "/api/sdk/v2/subaccounts", gameId, platformAccountId, platformToken, null);
    }

    @Override
    public Results.SubaccountOp setDefaultSubaccount(String gameId, String platformAccountId, String platformToken, String account) {
        return subaccountOp("PUT", "/api/sdk/v2/subaccounts/default", gameId, platformAccountId, platformToken, account);
    }

    @Override
    public Results.SubaccountLogin loginSubaccount(String gameId, String platformAccountId, String platformToken, String account) {
        Results.SubaccountLogin out = new Results.SubaccountLogin();
        try {
            JSONObject body = new JSONObject();
            body.put("gameId", gameId);
            body.put("platformAccountId", platformAccountId);
            body.put("platformToken", platformToken);
            body.put("account", account);
            Response r = call("POST", "/api/sdk/v2/subaccount-sessions", "", body.toString());
            out.ok = r.success;
            out.reason = r.reason;
            out.message = r.message;
            if (r.data != null) {
                out.account = r.data.optString("account", "");
                out.token = r.data.optString("token", "");
            }
        } catch (Exception e) {
            out.ok = false;
            out.reason = Reason.PLATFORM_UNAVAILABLE;
            out.message = "网络或解析失败";
        }
        return out;
    }

    // ===== 里程碑 3 端点(#27/#28) =====

    @Override
    public Results.RoleReport reportRole(String gameId, String account, String token, java.util.Map<String, String> fields) {
        Results.RoleReport out = new Results.RoleReport();
        try {
            JSONObject body = new JSONObject();
            body.put("gameId", gameId);
            body.put("account", account);
            body.put("token", token);
            for (java.util.Map.Entry<String, String> e : fields.entrySet()) {
                body.put(e.getKey(), e.getValue());
            }
            Response r = call("PUT", "/api/sdk/v2/roles", "", body.toString());
            out.ok = r.success;
            out.reason = r.reason;
            out.message = r.message;
            if (r.data != null) {
                out.reported = r.data.optBoolean("reported", false);
            }
        } catch (Exception e) {
            out.ok = false;
            out.reason = Reason.PLATFORM_UNAVAILABLE;
            out.message = "网络或解析失败";
        }
        return out;
    }

    @Override
    public Results.OrderCreate createOrder(String gameId, String account, String token, java.util.Map<String, Object> order) {
        Results.OrderCreate out = new Results.OrderCreate();
        try {
            JSONObject body = new JSONObject();
            body.put("gameId", gameId);
            body.put("account", account);
            body.put("token", token);
            for (java.util.Map.Entry<String, Object> e : order.entrySet()) {
                body.put(e.getKey(), e.getValue());
            }
            Response r = call("POST", "/api/sdk/v2/orders", "", body.toString());
            out.ok = r.success;
            out.reason = r.reason;
            out.message = r.message;
            if (r.data != null) {
                out.platformOrderId = r.data.optString("platformOrderId", "");
                out.paymentUrl = r.data.optString("paymentUrl", "");
                out.account = r.data.optString("account", "");
                out.amount = r.data.optString("amount", "");
                out.commodity = r.data.optString("commodity", "");
                out.serverName = r.data.optString("serverName", "");
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
        return callH(method, path, query, body, null, null);
    }

    private Response callH(String method, String path, String query, String body,
                           String credHeader, String credValue) throws IOException {
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
        if (credHeader != null && credValue != null) {
            conn.setRequestProperty(credHeader, credValue); // 凭据头不参与 canonical(04 §1.4)
        }
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
