package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertNull;
import static org.junit.Assert.assertTrue;

import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.uiautomator.By;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * #44 S3:支付容器横竖「开屏即定」双形态核验。
 *
 * <p>支付抽屉按挂载时方向选形态(07 §9/§1.12):横屏 = 右侧全高抽屉(关闭符号 ‹,Gravity.END);
 * 竖屏 = 底部抽屉(关闭符号 ⌄,≤80% 屏高,顶部抓手条,Gravity.BOTTOM)。
 * 严格 ADR-0009 只验开屏即定,不测旋转切换。
 */
@RunWith(AndroidJUnit4.class)
public class OrientationPayDrawerTest {

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

    /** 横屏开屏:支付容器为右侧全高抽屉(‹ 关闭符号、内容贴右半屏)。 */
    @Test
    public void landscapePayDrawerIsRightSide() throws Exception {
        h.setLandscape();
        h.toGame(TestHarness.uniquePhone());
        h.scrollToText("游戏支付"); // 横屏按钮列表过长,滚到 recharge 按钮
        h.tapText("游戏支付");
        assertTrue("应展示支付容器", h.hasText("5755 游戏支付", TestHarness.WAIT));
        assertTrue("订单字段取自入参", h.hasText("648 元宝", TestHarness.WAIT));
        assertTrue(h.hasText("确认支付", TestHarness.WAIT));
        // 横屏 = 右侧抽屉:关闭符号 ‹,不应有底部抽屉的 ⌄
        assertNotNull("横屏支付应为右侧抽屉(关闭符号 ‹)", h.device.findObject(By.text("‹")));
        assertNull("横屏不应是底部抽屉(⌄)", h.device.findObject(By.text("⌄")));
        assertTrue("横屏支付抽屉应贴右(标题中心在右半屏)", h.centerInRightHalf("5755 游戏支付"));
    }

    /** 竖屏开屏:支付容器为底部抽屉(⌄ 关闭符号、支付栏在下半屏)。 */
    @Test
    public void portraitPayDrawerIsBottom() throws Exception {
        h.toGame(TestHarness.uniquePhone()); // 竖屏(默认)
        h.tapText("游戏支付");
        assertTrue("应展示支付容器", h.hasText("5755 游戏支付", TestHarness.WAIT));
        assertTrue(h.hasText("确认支付", TestHarness.WAIT));
        // 竖屏 = 底部抽屉:关闭符号 ⌄,不应有右侧抽屉的 ‹
        assertNotNull("竖屏支付应为底部抽屉(关闭符号 ⌄)", h.device.findObject(By.text("⌄")));
        assertNull("竖屏不应是右侧抽屉(‹)", h.device.findObject(By.text("‹")));
        assertTrue("竖屏支付栏应在下半屏(底部抽屉)", h.topInBottomHalf("确认支付"));
    }
}
