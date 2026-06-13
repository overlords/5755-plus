package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertTrue;

import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.uiautomator.By;
import androidx.test.uiautomator.UiObject2;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * #44 S1:方向适配回归的使能切片 + 弹性弹窗横屏基线。
 *
 * <p>弹性弹窗(MATCH_PARENT/WRAP/weight,如协议告知、登录窗口)在宿主声明 configChanges 后,
 * 旋转不重建 Activity、由 Android 对视图树自动 re-measure/layout 重排(SDK 无监听器,ADR-0009
 * 「开屏即定」)。本类断言:横屏开屏这些弹窗完整在屏内,且旋转时浮层与输入态保留。
 */
@RunWith(AndroidJUnit4.class)
public class OrientationBaselineTest {

    private final TestHarness h = new TestHarness();

    @Before
    public void setUp() throws Exception {
        h.reset();
        h.freshLaunch();
    }

    @After
    public void tearDown() throws Exception {
        h.setPortrait(); // 复位竖屏,避免遗留横屏影响后续用例
        h.reset();
    }

    /** 横屏开屏:协议告知弹窗完整显示、拒绝/同意按钮在屏内。 */
    @Test
    public void landscapeProtocolDialogFits() throws Exception {
        h.setLandscape();
        h.tapText("进入游戏(onGameStart");
        assertTrue("init 后应可登录", h.hasText("init 成功", TestHarness.WAIT));
        h.tapText("登录(login)");
        assertTrue("横屏应展示协议告知", h.hasText("个人信息保护引导", TestHarness.WAIT));
        assertTrue(h.hasText("拒绝", TestHarness.WAIT));
        assertTrue(h.hasText("同意", TestHarness.WAIT));
        h.assertOnScreen("同意"); // 底部按钮在屏内 = 弹窗未竖向溢出
    }

    /** 横屏开屏:登录窗口完整显示,底部协议勾选行在屏内(无竖向溢出)。 */
    @Test
    public void landscapeLoginWindowFits() throws Exception {
        h.setLandscape();
        h.toLoginWindow();
        assertTrue(h.hasText("验证码登录", TestHarness.WAIT));
        assertTrue(h.hasText("请输入手机号", TestHarness.WAIT));
        assertTrue(h.hasText("登录", TestHarness.WAIT));
        h.assertOnScreen("我已阅读并同意"); // 登录窗最底部元素在屏内
    }

    /** 旋转保留:竖屏输入手机号后旋转到横屏,登录窗口仍在、已输入文字不丢(configChanges)。 */
    @Test
    public void rotatePreservesLoginWindowAndInput() throws Exception {
        h.toLoginWindow(); // 竖屏
        String phone = TestHarness.uniquePhone();
        UiObject2 phoneInput = h.device.findObject(By.textContains("请输入手机号"));
        assertNotNull("应有手机号输入框", phoneInput);
        phoneInput.setText(phone);
        h.setLandscape(); // 实时旋转
        assertTrue("旋转后登录窗口应仍在(configChanges 不重建)", h.hasText("验证码登录", TestHarness.WAIT));
        UiObject2 preserved = h.device.findObject(By.text(phone));
        assertNotNull("旋转后已输入手机号应保留", preserved);
    }
}
