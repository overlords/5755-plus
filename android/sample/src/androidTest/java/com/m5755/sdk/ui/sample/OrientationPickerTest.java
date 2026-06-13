package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertTrue;

import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.uiautomator.By;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * #44 S2:小号选择页(固定 dp 面板)横屏「开屏即定」fit 核验。
 *
 * <p>面板挂载时算 fixedW=min(480dp,屏宽-40dp) / fixedH=min(430dp,max(320dp,屏高-70dp)),
 * 横屏约 480×341dp 应 fit 屏不裁。本类在横屏开屏断言面板完整在屏内、控件可用、× 关闭有效。
 * 严格按 ADR-0009 只验开屏即定,不测旋转切换。
 */
@RunWith(AndroidJUnit4.class)
public class OrientationPickerTest {

    private final TestHarness h = new TestHarness();

    @Before
    public void setUp() throws Exception {
        h.reset();
        h.freshLaunch();
    }

    @After
    public void tearDown() throws Exception {
        h.setPortrait();
        h.reset();
    }

    /** 横屏开屏:小号选择页面板完整在屏内,⇄/!/× 与小号行可用。 */
    @Test
    public void landscapeSubaccountPickerFits() throws Exception {
        h.setLandscape();
        h.toSubaccountPicker(TestHarness.uniquePhone());

        // 面板关键元素在屏内(横屏裁切守卫:若面板竖向溢出,顶部标题/添加小号会越界)
        assertTrue(h.hasText("添加小号", TestHarness.WAIT));
        assertTrue(h.hasText("小号1", TestHarness.WAIT));
        h.assertOnScreen("选择小号进入游戏"); // 标题行(面板上部)
        h.assertOnScreen("添加小号");         // 标题行右
        h.assertOnScreen("小号1");            // 列表首行

        // 方向无关的控件契约(contentDescription)在横屏仍在
        assertNotNull("应有 ⇄ 切换5755账户", h.device.findObject(By.desc("切换5755账户")));
        assertNotNull("应有 × 关闭小号选择页", h.device.findObject(By.desc("关闭小号选择页")));
        assertNotNull("应有进入箭头", h.device.findObject(By.desc("进入")));

        // 横屏下点骑角 × 关闭选择页 → onPickerClosed → onFlowCanceled → 登录回调 success=false
        h.device.findObject(By.desc("关闭小号选择页")).click();
        assertTrue("横屏点 × 应关闭选择页并通知游戏「登录未完成」",
                h.hasText("登录未完成", TestHarness.WAIT));
        assertTrue("× 后选择页应消失", !h.hasText("选择小号进入游戏", 2000));
    }
}
