package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.uiautomator.By;
import androidx.test.uiautomator.UiObject2;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * #43 本轮 6 项用户视角 UX 修复的仪器化回归(GA 前防回退,ADR-0002 / 08 第三面)。
 * 断言一律走可观察状态(弹窗/页面文本/导航),不断言 toast——Android 11+ uiautomator
 * 抓不到 toast 文本(单独 window)。故 ! 信息/手机号提示文案等纯 toast 效果改测其关联
 * 可观察行为(如手机号非法 → 不发码 → 按钮不进倒计时)。自动登录提示已由
 * LoginFlowTest.autoLoginSkipsLoginWindowAfterRestart 覆盖。
 */
@RunWith(AndroidJUnit4.class)
public class UxRegressionTest {

    private final TestHarness h = new TestHarness();

    @Before
    public void setUp() throws Exception {
        h.reset();
    }

    @After
    public void tearDown() throws Exception {
        h.reset();
    }

    /** 协议告知弹窗展示四个协议名(可读性前提;任一断链/缺失会被发现)。 */
    @Test
    public void protocolDialogShowsFourProtocols() throws Exception {
        h.freshLaunch();
        h.tapText("进入游戏(onGameStart");
        assertTrue("init 成功", h.hasText("init 成功", TestHarness.WAIT));
        h.tapText("登录(login");
        assertTrue("应展示协议告知弹窗", h.hasText("个人信息保护引导", TestHarness.WAIT));
        assertTrue("注册协议", h.hasText("用户注册协议", TestHarness.WAIT));
        assertTrue("隐私协议", h.hasText("用户隐私协议", TestHarness.WAIT));
        assertTrue("儿童指引", h.hasText("儿童隐私保护指引", TestHarness.WAIT));
        assertTrue("第三方清单", h.hasText("第三方信息共享清单", TestHarness.WAIT));
    }

    /** 手机号校验:11 位非 1 开头被拦,不发码(按钮停留「发送验证码」、不进倒计时);
     *  合法号能发码(按钮进入「重新发送」倒计时)——证明校验收紧没误伤正常登录。 */
    @Test
    public void phoneValidationBlocksInvalidNotLegal() throws Exception {
        h.freshLaunch();
        h.toLoginWindow();
        UiObject2 phone = h.device.findObject(By.textContains("请输入手机号"));
        phone.click();
        phone.setText("23456789012"); // 11 位、非 1 开头
        h.tapText("发送验证码");
        Thread.sleep(1500);
        assertTrue("非法号不发码:按钮停留发送验证码", h.hasText("发送验证码", 2000));
        assertFalse("非法号不应进入倒计时", h.hasText("重新发送", 1200));

        // 合法号:发码成功 → 按钮进倒计时(60s→「重新发送」),证明校验未误伤
        phone.setText(TestHarness.uniquePhone());
        h.tapText("发送验证码");
        assertTrue("合法号应发码并倒计时(60s 后重新发送)", h.hasText("重新发送", 65000));
    }

    /** 小号选择页 ⇄ 切换 5755 账户 → logout 语义 → 回登录窗。 */
    @Test
    public void subaccountSwitchAccountReturnsToLogin() throws Exception {
        h.toSubaccountPicker();
        h.tapExact("⇄");
        assertTrue("⇄ 切 5755 账户应回登录窗", h.hasText("验证码登录", TestHarness.WAIT));
    }

    /** 小号选择页骑角 × → onPickerClosed → 关闭选择页(登录链路=进入未完成)。 */
    @Test
    public void subaccountCloseDismissesPicker() throws Exception {
        h.toSubaccountPicker();
        UiObject2 closeX = h.device.findObject(By.desc("关闭小号选择页"));
        closeX.click();
        Thread.sleep(800);
        assertFalse("× 关闭后小号选择页应消失", h.hasText("选择小号进入游戏", 2500));
    }
}
