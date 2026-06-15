package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertTrue;

import androidx.test.ext.junit.runners.AndroidJUnit4;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * uc SPA「退出登录」回归:它依赖 WebView confirm() 二次确认。SDK WebView 此前没设
 * WebChromeClient,confirm() 默认返回 false → logout 静默失效。加 onJsConfirm(弹原生
 * AlertDialog)后:点退出登录 → 弹确认框 → 确定 → logout → 回登录窗。
 */
@RunWith(AndroidJUnit4.class)
public class UserCenterLogoutTest {

    private final TestHarness h = new TestHarness();

    @Before
    public void setUp() throws Exception {
        h.reset();
    }

    @After
    public void tearDown() throws Exception {
        h.reset();
    }

    @Test
    public void userCenterLogoutConfirmReturnsToLogin() throws Exception {
        h.loginToGame();
        h.tapExact("账");
        assertTrue("uc SPA 应渲染", h.hasText("退出登录", TestHarness.WAIT) || h.hasText("账号安全", TestHarness.WAIT));
        h.tapInWebView("退出登录"); // uc SPA 退出登录按钮
        // confirm() → WebChromeClient.onJsConfirm → 原生 AlertDialog(修复前此框根本不弹)
        assertTrue("退出登录应弹原生确认框", h.hasText("确认退出登录", TestHarness.WAIT));
        h.tapExact("确定");
        assertTrue("确认后应 logout → 回登录窗", h.hasText("验证码登录", TestHarness.WAIT));
    }
}
