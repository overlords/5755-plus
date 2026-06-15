package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertTrue;

import androidx.test.ext.junit.runners.AndroidJUnit4;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * #44 横屏方向适配核验(07 §1.12 / ADR-0009「开屏即定」):锁横屏下挂载关键面,
 * 断言核心元素可见(裁切则底部不可见)+ 截图供视觉核验。
 * 只测「开屏即定」(锁方向挂载即正确),不测旋转实时切换(ADR-0009:不做不测)。
 */
@RunWith(AndroidJUnit4.class)
public class OrientationTest {

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

    /** 小号选择页(固定 dp 面板)横屏:最可能裁切的面。 */
    @Test
    public void subaccountPickerLandscape() throws Exception {
        h.setLandscape();
        h.toSubaccountPicker();
        assertTrue("横屏标题应可见", h.hasText("选择小号进入游戏", TestHarness.WAIT));
        assertTrue("横屏小号行应可见", h.hasText("小号1", TestHarness.WAIT));
        h.screencap("orient_picker_land");
    }

    /** 支付容器横屏:应为右侧全高抽屉。 */
    @Test
    public void payDrawerLandscape() throws Exception {
        h.setLandscape();
        h.loginToGame();
        h.scrollToTap("游戏支付"); // 样例主屏 ScrollView 横屏需滚到按钮
        assertTrue("横屏支付容器应可见", h.hasText("5755 游戏支付", TestHarness.WAIT));
        assertTrue("订单字段取自入参", h.hasText("648 元宝", TestHarness.WAIT));
        h.screencap("orient_pay_land"); // 抽屉弹出后截,看横屏右侧全高形态
    }

    /** 用户中心横屏:应为左侧全高抽屉(右侧留游戏可见)。 */
    @Test
    public void userCenterLandscape() throws Exception {
        h.setLandscape();
        h.loginToGame();
        h.tapExact("账");
        Thread.sleep(4000); // uc SPA 远程加载(uc.xingninghuyu.com)
        assertTrue("横屏用户中心 uc SPA 应渲染(主页:当前小号/账号安全)",
                h.hasText("账号安全", TestHarness.WAIT) || h.hasText("当前小号", TestHarness.WAIT));
        h.screencap("orient_uc_land"); // uc SPA 渲染后截,看横屏左侧全高抽屉
    }

    /** 弹性弹窗(登录窗):横屏靠 Android 自动重排,不溢出(回归基线;弹性面不退化为固定 dp)。 */
    @Test
    public void loginWindowLandscape() throws Exception {
        h.setLandscape();
        h.freshLaunch();
        h.toLoginWindow();
        assertTrue("横屏登录窗(弹性弹窗自动重排)应完整可见", h.hasText("验证码登录", TestHarness.WAIT));
        h.screencap("orient_login_land");
    }
}
