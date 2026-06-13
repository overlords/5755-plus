package com.m5755.sdk.ui.sample;

import android.content.Context;
import android.content.Intent;

import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.uiautomator.By;
import androidx.test.uiautomator.UiDevice;
import androidx.test.uiautomator.UiObject2;
import androidx.test.uiautomator.Until;

import com.m5755.operate.core.gateway.HttpPlatformGateway;
import com.m5755.operate.core.gateway.Results;
import com.m5755.operate.core.net.PlatformConfig;
import com.m5755.operate.core.net.Signer;

import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;

import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertTrue;

/**
 * 仪器化回归基建(ADR-0002):对接线上 sdk-dev;UiAutomator 文本选择器驱动,不用坐标。
 * dev 控制面注入用与 SDK 同源的签名实现直接调用。
 */
final class TestHarness {

    static final String BASE = "https://sdk-dev.xingninghuyu.com";
    static final String GAME = "m5755-demo";
    static final String KEY_ID = "dev-test-key";
    static final String SECRET = "m5755-dev-public-test-secret-v1";
    static final long WAIT = 12000;

    final UiDevice device = UiDevice.getInstance(InstrumentationRegistry.getInstrumentation());
    final HttpPlatformGateway gateway = new HttpPlatformGateway(
            new PlatformConfig(BASE, "integration", "dev", KEY_ID, SECRET, "1.0.0"));

    /** 清 app 数据(新装语义)并冷启动样例。 */
    void freshLaunch() throws Exception {
        Context ctx = InstrumentationRegistry.getInstrumentation().getTargetContext();
        // 清 prefs(新用户首登场景,08 §2.2)
        ctx.getSharedPreferences("m5755_operate_min", Context.MODE_PRIVATE).edit().clear().commit();
        launch();
    }

    void launch() {
        Context ctx = InstrumentationRegistry.getInstrumentation().getTargetContext();
        Intent i = ctx.getPackageManager().getLaunchIntentForPackage(ctx.getPackageName());
        i.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TASK);
        ctx.startActivity(i);
        device.wait(Until.hasObject(By.textContains("5755 SDK 样例")), WAIT);
    }

    UiObject2 waitText(String text) {
        device.wait(Until.hasObject(By.textContains(text)), WAIT);
        return device.findObject(By.textContains(text));
    }

    void tapText(String text) {
        UiObject2 o = waitText(text);
        assertNotNull("找不到控件: " + text, o);
        o.click();
    }

    /** 精确文本匹配(规避「登录」「进入游戏」等前缀歧义)。 */
    void tapExact(String text) {
        device.wait(Until.hasObject(By.text(text)), WAIT);
        UiObject2 o = device.findObject(By.text(text));
        assertNotNull("找不到控件(exact): " + text, o);
        o.click();
    }

    boolean hasText(String text, long timeoutMs) {
        return device.wait(Until.hasObject(By.textContains(text)), timeoutMs) == Boolean.TRUE;
    }

    /** init + login 到登录窗口(全新安装:经协议同意)。 */
    void toLoginWindow() throws Exception {
        tapText("进入游戏(onGameStart");
        assertTrue("init 后应可登录", hasText("init 成功", WAIT));
        tapText("登录(login)");
        if (hasText("个人信息保护引导", 6000)) {
            tapExact("同意"); // 正文含「同意后进入…」,必须精确匹配按钮
        }
        assertTrue("应到达登录窗口", hasText("验证码登录", WAIT));
    }

    /** 在登录窗口完成 devCode 登录(经 SDK 同源网关取 devCode)。 */
    String doSmsLogin(String phone) {
        Results.Sms sms = gateway.requestSms(GAME, phone);
        assertTrue("测试夹具取 devCode 失败: " + sms.message, sms.ok && sms.devCode != null);
        // 先勾协议(避免键盘遮挡),再填表
        tapText("我已阅读并同意");
        UiObject2 phoneInput = device.findObject(By.textContains("请输入手机号"));
        assertNotNull(phoneInput);
        phoneInput.click();
        phoneInput.setText(phone);
        UiObject2 codeInput = device.findObject(By.textContains("请输入验证码"));
        assertNotNull(codeInput);
        codeInput.click();
        codeInput.setText(sms.devCode);
        tapExact("登录");
        return phone;
    }

    // ===== dev 控制面(签名直调) =====

    void devControl(String path, JSONObject body) throws Exception {
        String url = BASE + "/internal/dev-control/" + path;
        String payload = body.toString();
        long ts = System.currentTimeMillis() / 1000L;
        String canonical = Signer.canonical("POST", "/internal/dev-control/" + path, "", String.valueOf(ts), payload);
        String sig = Signer.compute(SECRET, canonical);
        HttpURLConnection conn = (HttpURLConnection) new URL(url).openConnection();
        conn.setRequestMethod("POST");
        conn.setConnectTimeout(8000);
        conn.setReadTimeout(8000);
        conn.setRequestProperty("Content-Type", "application/json; charset=UTF-8");
        conn.setRequestProperty(Signer.HEADER_TIMESTAMP, String.valueOf(ts));
        conn.setRequestProperty(Signer.HEADER_KEY_ID, KEY_ID);
        conn.setRequestProperty(Signer.HEADER_SIGNATURE, sig);
        conn.setDoOutput(true);
        OutputStream os = conn.getOutputStream();
        os.write(payload.getBytes("UTF-8"));
        os.close();
        int code = conn.getResponseCode();
        InputStream is = code < 300 ? conn.getInputStream() : conn.getErrorStream();
        ByteArrayOutputStream bos = new ByteArrayOutputStream();
        byte[] buf = new byte[2048];
        int n;
        while (is != null && (n = is.read(buf)) != -1) {
            bos.write(buf, 0, n);
        }
        conn.disconnect();
        if (code >= 300) {
            throw new IllegalStateException("dev-control " + path + " http=" + code + " body=" + bos);
        }
    }

    void setMaintenance(boolean enabled, String message) throws Exception {
        JSONObject b = new JSONObject();
        b.put("gameId", GAME);
        b.put("enabled", enabled);
        b.put("message", message);
        devControl("maintenance", b);
    }

    void kick(String platformAccountId) throws Exception {
        JSONObject b = new JSONObject();
        b.put("gameId", GAME);
        b.put("platformAccountId", platformAccountId);
        devControl("kick", b);
    }

    void invalidateSubaccount(String account) throws Exception {
        JSONObject b = new JSONObject();
        b.put("gameId", GAME);
        b.put("account", account);
        devControl("invalidate-subaccount", b);
    }

    void antiAddiction(String platformAccountId, boolean entry, boolean payment) throws Exception {
        JSONObject b = new JSONObject();
        b.put("gameId", GAME);
        b.put("platformAccountId", platformAccountId);
        b.put("entryBlocked", entry);
        b.put("paymentBlocked", payment);
        devControl("anti-addiction", b);
    }

    void reset() throws Exception {
        JSONObject b = new JSONObject();
        b.put("gameId", GAME);
        devControl("reset", b);
    }

    static String uniquePhone() {
        return "151" + String.format("%08d", Math.abs((int) (System.nanoTime() % 100000000L)));
    }

    String prefs(String key) {
        Context ctx = InstrumentationRegistry.getInstrumentation().getTargetContext();
        return ctx.getSharedPreferences("m5755_operate_min", Context.MODE_PRIVATE).getString(key, null);
    }

    // ===== 屏幕方向(#44 横屏方向适配回归;ADR-0009 只验开屏即定) =====

    /** 设横屏(左旋 90°)。宿主声明 configChanges,Activity 不重建、浮层不销毁。 */
    void setLandscape() throws Exception {
        device.setOrientationLeft();
        device.waitForIdle();
    }

    /** 复位竖屏(自然方向)并解冻旋转。tearDown 必调,避免遗留横屏影响后续用例。 */
    void setPortrait() throws Exception {
        device.setOrientationNatural();
        device.unfreezeRotation();
        device.waitForIdle();
    }

    /** 断言控件可见且完整落在屏内(横屏裁切/溢出守卫):可见尺寸非零且四边不越界。 */
    void assertOnScreen(String textContains) {
        UiObject2 o = device.findObject(By.textContains(textContains));
        assertNotNull("找不到控件: " + textContains, o);
        android.graphics.Rect b = o.getVisibleBounds();
        int w = device.getDisplayWidth();
        int hpx = device.getDisplayHeight();
        assertTrue("控件不可见(可见尺寸为零,疑被裁): " + textContains + " bounds=" + b, b.height() > 0 && b.width() > 0);
        assertTrue("控件越出屏幕(横屏裁切/溢出): " + textContains + " bounds=" + b + " screen=" + w + "x" + hpx,
                b.left >= 0 && b.top >= 0 && b.right <= w && b.bottom <= hpx);
    }
}
