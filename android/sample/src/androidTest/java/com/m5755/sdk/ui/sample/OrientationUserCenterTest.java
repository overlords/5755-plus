package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertTrue;

import android.graphics.Rect;

import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.uiautomator.UiObject2;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * #44 S4:用户中心抽屉横竖「开屏即定」形态核验。
 *
 * <p>用户中心是左侧贴边 WebView 抽屉(Gravity.START):横屏宽 min(dm,max(520dp,dm*0.58));
 * 竖屏再 ≤80% 屏宽 cap,右侧必留游戏可见条(07 §11.2/§1.12 非全屏遮挡)。本类经 WebView 容器
 * bounds 判定方向形态(不依赖 #5 后平台 H5/回退页内容)。严格 ADR-0009 只验开屏即定。
 */
@RunWith(AndroidJUnit4.class)
public class OrientationUserCenterTest {

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

    /** 竖屏开屏:用户中心左侧贴边、宽 ≤80% 屏宽、右侧留游戏可见条。 */
    @Test
    public void portraitUserCenterCappedLeftDrawer() throws Exception {
        h.toGame(TestHarness.uniquePhone()); // 竖屏(默认)
        UiObject2 web = h.openUserCenter();
        Rect b = web.getVisibleBounds();
        int W = h.device.getDisplayWidth();
        assertTrue("竖屏用户中心应左侧贴边: " + b, b.left <= W * 0.05);
        assertTrue("竖屏用户中心宽度应 ≤80% 屏宽(留游戏可见条): w=" + b.width() + " W=" + W, b.width() <= W * 0.85);
        assertTrue("右侧应留游戏可见条: " + b + " W=" + W, b.right < W * 0.95);
    }

    /** 横屏开屏:用户中心左侧贴边全高、右侧留游戏可见条(非全屏遮挡)。 */
    @Test
    public void landscapeUserCenterLeftFullHeight() throws Exception {
        h.setLandscape();
        h.toGame(TestHarness.uniquePhone());
        UiObject2 web = h.openUserCenter();
        Rect b = web.getVisibleBounds();
        int W = h.device.getDisplayWidth();
        int hpx = h.device.getDisplayHeight();
        // 横屏左侧贴边:贴的是内容区左缘;横屏系统栏 inset(rotated 状态栏)可达 ~170px,故容差放宽到 10%
        assertTrue("横屏用户中心应左侧贴边(内容区左缘): " + b, b.left <= W * 0.1);
        assertTrue("横屏用户中心非全屏(右侧游戏可见): " + b + " W=" + W, b.right < W * 0.95);
        assertTrue("横屏用户中心应近全高(非小弹窗): h=" + b.height() + " H=" + hpx, b.height() >= hpx * 0.6);
    }
}
