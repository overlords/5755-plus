package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertTrue;

import androidx.test.ext.junit.runners.AndroidJUnit4;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * #53 uc SPA 四页正式截图验收(主页/换绑/改密/订单)× 横屏/竖屏独立。
 * 进游戏 → 悬浮球「账」打开用户中心(SDK 铸 platformToken 注入 uc SPA),WebView 内
 * data-nav 导航 + subhead「‹」返回,逐页 screencap;host 侧 adb pull 入 docs/assets/acceptance-uc/。
 * 横屏/竖屏各一个 @Test(独立),横屏优先。
 */
@RunWith(AndroidJUnit4.class)
public class UcScreenshotTest {

    private final TestHarness h = new TestHarness();

    @Before
    public void setUp() throws Exception {
        h.reset();
    }

    @After
    public void tearDown() throws Exception {
        h.setPortrait(); // 复位,避免污染后续测试
        h.reset();
    }

    private void shootFourPages(String suffix) throws Exception {
        h.tapExact("账");
        assertTrue("uc SPA 主页应渲染", h.hasText("账号安全", TestHarness.WAIT) || h.hasText("当前小号", TestHarness.WAIT));
        h.screencap("uc_home_" + suffix);
        h.tapInWebView("绑定手机");
        Thread.sleep(1500);
        h.screencap("uc_phone_" + suffix);
        h.tapInWebView("返回");
        Thread.sleep(900);
        h.tapInWebView("修改密码");
        Thread.sleep(1500);
        h.screencap("uc_password_" + suffix);
        h.tapInWebView("返回");
        Thread.sleep(900);
        h.tapInWebView("充值订单");
        Thread.sleep(1500);
        h.screencap("uc_orders_" + suffix);
    }

    /** 横屏(优先):主页/换绑/改密/订单四页截图。 */
    @Test
    public void ucFourPagesLandscape() throws Exception {
        h.setLandscape();
        h.loginToGame();
        shootFourPages("land");
    }

    /** 竖屏:主页/换绑/改密/订单四页截图。 */
    @Test
    public void ucFourPagesPortrait() throws Exception {
        h.setPortrait();
        h.loginToGame();
        shootFourPages("port");
    }
}
