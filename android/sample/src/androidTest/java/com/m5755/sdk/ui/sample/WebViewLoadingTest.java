package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertTrue;

import androidx.test.ext.junit.runners.AndroidJUnit4;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * #51 WebView 加载态:错误路径仪器化回归(07 §1.13)。
 *
 * <p>断网触发用户中心远程加载失败 → 断言「加载失败」+「重试」→ 复网点重试 → 内容最终显示。
 * 占位+淡入属瞬时/视觉,不在此硬断言(视觉走人工)。复用 androidTest/TestHarness(ADR-0002)。
 */
@RunWith(AndroidJUnit4.class)
public class WebViewLoadingTest {

    private final TestHarness h = new TestHarness();

    @Before
    public void setUp() throws Exception {
        h.reset();
        h.freshLaunch();
    }

    @After
    public void tearDown() throws Exception {
        h.setNetwork(true); // 必复网,避免遗留断网影响后续用例
        h.reset();
    }

    /** 断网→用户中心加载失败+重试;复网→重试→uc SPA 内容显示。 */
    @Test
    public void userCenterLoadErrorThenRetrySucceeds() throws Exception {
        loginToGame();

        // 断网 → 打开用户中心(远程 loadUrl 失败)
        h.setNetwork(false);
        h.tapExact("账");
        assertTrue("断网时用户中心应显示加载失败", h.hasText("加载失败", TestHarness.WAIT));
        assertTrue("应有重试入口", h.hasText("重试", TestHarness.WAIT));

        // 复网 → 重试 → 内容最终显示(uc SPA 主账户视图)
        h.setNetwork(true);
        h.tapExact("重试");
        assertTrue("重试后应加载出 uc SPA 内容",
                h.hasText("换绑手机", TestHarness.WAIT) || h.hasText("充值订单", TestHarness.WAIT));
        assertFalse("重试成功后不应再停在加载失败", h.hasText("加载失败", 2000));
    }

    /** 复用 OperatingLoopTest 的进游戏链路(网络开)。 */
    private void loginToGame() throws Exception {
        h.toLoginWindow();
        h.doSmsLogin(TestHarness.uniquePhone());
        h.waitText("请输入真实姓名").setText("测试玩家");
        h.waitText("请输入身份证号").setText("11010119900101001X");
        h.tapExact("提交");
        assertTrue(h.hasText("选择小号进入游戏", TestHarness.WAIT));
        h.tapExact("小号1");
        assertTrue(h.hasText("登录态校验通过", TestHarness.WAIT));
        h.tapExact("进入游戏");
        assertNotNull(h.prefs("account"));
    }
}
