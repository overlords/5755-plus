package com.m5755.operate.core.gateway;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertTrue;

import com.m5755.operate.core.net.PlatformConfig;

import org.junit.Assume;
import org.junit.Before;
import org.junit.Test;

/**
 * 实网集成:Android 真实网关 → HTTP(签名)→ 平台服务端 → Postgres 全栈往返。
 * 仅当传入 -DM5755_SERVER_URL=http://127.0.0.1:PORT 时运行,否则跳过。
 * 证明两端在真实服务端上互通(签名验证、ApiResult/reason 解析、登录建档),无需部署或模拟器。
 */
public class HttpPlatformGatewayLiveTest {

    private static final String KEY_ID = "dev-test-key";
    private static final String SECRET = "m5755-dev-public-test-secret-v1";
    private static final String GAME = "m5755-demo";

    private String url;

    @Before
    public void setup() {
        url = System.getProperty("M5755_SERVER_URL", "");
        Assume.assumeTrue("未提供 M5755_SERVER_URL,跳过实网集成测试", url != null && !url.isEmpty());
    }

    private HttpPlatformGateway gw(String secret) {
        return new HttpPlatformGateway(new PlatformConfig(url, "local", "local", KEY_ID, secret, "1.0.0"));
    }

    private static String uniquePhone() {
        return String.format("138%08d", Math.abs(System.nanoTime() % 100000000L));
    }

    @Test
    public void configRoundTrip() {
        Results.Config c = gw(SECRET).fetchConfig(GAME, "1.0.0", "com.x", "default", "manifest");
        assertTrue("config 应成功: reason=" + c.reason, c.ok);
        assertNotNull(c.protocolVersion);
        assertFalse(c.protocolVersion.isEmpty());
    }

    @Test
    public void smsThenLoginRoundTrip() {
        HttpPlatformGateway g = gw(SECRET);
        String phone = uniquePhone();

        Results.Sms sms = g.requestSms(GAME, phone);
        assertTrue("sms 应成功: reason=" + sms.reason, sms.ok);
        assertNotNull("mock 模式应返回 devCode", sms.devCode);

        Results.Login login = g.login(GAME, phone, sms.devCode, "default", "manifest");
        assertTrue("登录应成功: reason=" + login.reason, login.ok);
        assertNotNull(login.platformToken);
        assertFalse(login.platformToken.isEmpty());
        assertNotNull("新用户应建档首个游戏小号", login.firstAccount);
    }

    @Test
    public void badSignatureRejected() {
        Results.Config c = gw("wrong-secret").fetchConfig(GAME, "1.0.0", "com.x", "default", "manifest");
        assertFalse(c.ok);
        assertEquals(Reason.SIGNATURE_INVALID, c.reason);
    }

    @Test
    public void invalidCodeReturnsReason() {
        Results.Login login = gw(SECRET).login(GAME, uniquePhone(), "000000", "default", "manifest");
        assertFalse(login.ok);
        assertEquals(Reason.SMS_CODE_INVALID, login.reason);
    }
}
