package com.m5755.operate.core.gateway;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import com.m5755.operate.core.net.PlatformConfig;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetAddress;
import java.net.ServerSocket;
import java.net.Socket;
import java.nio.charset.Charset;

/**
 * HttpPlatformGateway 单测:用 java.net.ServerSocket 在进程内起极简 HTTP 端点喂 canned ApiResult,
 * 覆盖真实网关的 JSON 解析 / reason 映射 / 坏响应健壮性(04 §1.2)。
 *
 * 与 {@link HttpPlatformGatewayLiveTest}(需外部 -DM5755_SERVER_URL、CI 里 Assume skip)互补:
 * 本测无外部依赖、CI 每次真跑,堵住「真网关解析逻辑在 CI 零覆盖」的缺口。
 * 只用 java.net(android.jar 内含),不引第三方、不违 01 依赖白名单。
 */
public class HttpPlatformGatewayTest {

    private static final Charset UTF8 = Charset.forName("UTF-8");

    private ServerSocket serverSocket;
    private Thread acceptThread;
    private volatile boolean running;
    private volatile int status = 200;
    private volatile String body = "{}";
    private HttpPlatformGateway gw;

    @Before
    public void setUp() throws IOException {
        serverSocket = new ServerSocket(0, 0, InetAddress.getByName("127.0.0.1"));
        running = true;
        acceptThread = new Thread(this::serveLoop, "gw-test-http");
        acceptThread.setDaemon(true);
        acceptThread.start();
        String base = "http://127.0.0.1:" + serverSocket.getLocalPort();
        gw = new HttpPlatformGateway(new PlatformConfig(base, "local", "local", "k", "s", "1.0.0"));
    }

    @After
    public void tearDown() throws IOException {
        running = false;
        if (serverSocket != null) {
            serverSocket.close(); // 打断 accept()
        }
    }

    /** 每连接一应:读完请求头(+按 Content-Length 排空体)→ 回一个带 Content-Length 的响应 → 关连接。 */
    private void serveLoop() {
        while (running) {
            try (Socket sock = serverSocket.accept()) {
                InputStream in = sock.getInputStream();
                int contentLength = drainHeadersReturnContentLength(in);
                for (int i = 0; i < contentLength; i++) {
                    if (in.read() == -1) {
                        break;
                    }
                }
                byte[] payload = body.getBytes(UTF8);
                String head = "HTTP/1.1 " + status + " X\r\n"
                        + "Content-Type: application/json; charset=UTF-8\r\n"
                        + "Content-Length: " + payload.length + "\r\n"
                        + "Connection: close\r\n\r\n";
                OutputStream out = sock.getOutputStream();
                out.write(head.getBytes(UTF8));
                out.write(payload);
                out.flush();
            } catch (IOException e) {
                // serverSocket.close() 打断 accept 时落此;running=false 即退出
            }
        }
    }

    /** 逐字节读到 \r\n\r\n,顺带解析 Content-Length(大小写不敏感)。 */
    private static int drainHeadersReturnContentLength(InputStream in) throws IOException {
        StringBuilder sb = new StringBuilder();
        int b;
        while ((b = in.read()) != -1) {
            sb.append((char) b);
            int n = sb.length();
            if (n >= 4 && sb.charAt(n - 1) == '\n' && sb.charAt(n - 2) == '\r'
                    && sb.charAt(n - 3) == '\n' && sb.charAt(n - 4) == '\r') {
                break;
            }
        }
        int cl = 0;
        for (String line : sb.toString().split("\r\n")) {
            int colon = line.indexOf(':');
            if (colon > 0 && line.substring(0, colon).trim().equalsIgnoreCase("Content-Length")) {
                try {
                    cl = Integer.parseInt(line.substring(colon + 1).trim());
                } catch (NumberFormatException ignored) {
                    cl = 0;
                }
            }
        }
        return cl;
    }

    private void respond(int code, String json) {
        this.status = code;
        this.body = json;
    }

    @Test
    public void fetchConfig_success_parsesData() {
        respond(200, "{\"success\":true,\"data\":{\"protocolVersion\":\"v3\",\"configVersion\":\"c1\","
                + "\"userCenterUrl\":\"https://uc.example/\",\"maintenance\":{\"enabled\":false}}}");
        Results.Config c = gw.fetchConfig("g", "1.0.0", "com.x", "default", "manifest");
        assertTrue("应成功: reason=" + c.reason, c.ok);
        assertEquals("v3", c.protocolVersion);
        assertEquals("c1", c.configVersion);
        assertEquals("https://uc.example/", c.userCenterUrl);
        assertFalse(c.maintenanceEnabled);
    }

    @Test
    public void login_success_parsesTokenAndFirstAccount() {
        respond(200, "{\"success\":true,\"data\":{\"platformAccountId\":\"pa_1\",\"platformToken\":\"pt_abc\","
                + "\"displayName\":\"云起\",\"gameEntry\":{\"isNewGameUser\":true,"
                + "\"createdSubaccount\":{\"account\":\"ga_1\",\"isDefault\":false}}}}");
        Results.Login l = gw.login("g", "13800000000", "123456", "default", "manifest");
        assertTrue("应成功: reason=" + l.reason, l.ok);
        assertEquals("pt_abc", l.platformToken);
        assertEquals("pa_1", l.platformAccountId);
        assertTrue(l.isNewGameUser);
        assertEquals("ga_1", l.firstAccount);
    }

    @Test
    public void login_failure_mapsReasonFromNon2xx() {
        // 非 2xx + reason:网关须读 errorStream 并透传 reason(04 §1.2 不因 HTTP 状态丢业务原因)
        respond(401, "{\"success\":false,\"reason\":\"sms_code_invalid\",\"message\":\"验证码错误\"}");
        Results.Login l = gw.login("g", "13800000000", "000000", "default", "manifest");
        assertFalse(l.ok);
        assertEquals(Reason.SMS_CODE_INVALID, l.reason);
    }

    @Test
    public void requestSms_success_parsesDevCode() {
        respond(200, "{\"success\":true,\"data\":{\"codeId\":\"cid\",\"loginAccountMasked\":\"138****0000\",\"devCode\":\"654321\"}}");
        Results.Sms s = gw.requestSms("g", "13800000000");
        assertTrue(s.ok);
        assertEquals("654321", s.devCode);
        assertEquals("138****0000", s.loginAccountMasked);
    }

    @Test
    public void malformedJson_mapsToPlatformUnavailable() {
        // 04 §1.2:响应 JSON 不可解析时按失败处理(不崩)
        respond(200, "<html>not json</html>");
        Results.Config c = gw.fetchConfig("g", "1.0.0", "com.x", "default", "manifest");
        assertFalse(c.ok);
        assertEquals(Reason.PLATFORM_UNAVAILABLE, c.reason);
    }
}
