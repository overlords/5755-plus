package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertTrue;

import androidx.test.ext.junit.runners.AndroidJUnit4;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * 里程碑 3 运营闭环仪器化(对线上 sdk-dev):登录到游戏 → 角色上报 → 支付容器 → 悬浮球/用户中心。
 * 场景 8-12 等价。
 */
@RunWith(AndroidJUnit4.class)
public class OperatingLoopTest {

    private final TestHarness h = new TestHarness();

    @Before
    public void setUp() throws Exception {
        h.reset();
    }

    @After
    public void tearDown() throws Exception {
        h.reset();
    }

    private void loginToGame() throws Exception {
        h.freshLaunch();
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

    @Test
    public void roleReportShowsRealFields() throws Exception {
        loginToGame();
        h.tapText("角色上报");
        assertTrue("应展示角色上报结果", h.hasText("角色上报成功", TestHarness.WAIT));
        assertTrue("展示真实区服", h.hasText("星河一区", TestHarness.WAIT));
        h.tapExact("我知道了");
    }

    @Test
    public void rechargeShowsPayDrawerFromOrder() throws Exception {
        loginToGame();
        h.tapText("游戏支付");
        assertTrue("应展示支付容器", h.hasText("5755 游戏支付", TestHarness.WAIT));
        assertTrue("订单字段取自入参", h.hasText("648 元宝", TestHarness.WAIT));
        assertTrue("发放口径固化", h.hasText("充值回调为准", TestHarness.WAIT));
        assertTrue(h.hasText("确认支付", TestHarness.WAIT));
    }

    @Test
    public void userCenterSwitchAccountEntersPicker() throws Exception {
        loginToGame();
        // 悬浮球"账" → 用户中心(平台 uc SPA,以主账户为核心)→ 切换 → 小号选择页
        // demo userCenterUrl 指向 uc.xingninghuyu.com(已部署),加载真 uc SPA(06a)
        h.tapExact("账");
        assertTrue("用户中心应渲染 uc SPA(主账户视图)",
                h.hasText("换绑手机", TestHarness.WAIT) || h.hasText("充值订单", TestHarness.WAIT));
        h.tapInWebView("切换"); // uc SPA 当前小号行「切换」→ bridge switch_account(在 WebView 内点,避开主屏按钮)
        assertTrue("切换进选择页", h.hasText("选择小号进入游戏", TestHarness.WAIT));
        assertTrue("不触发登出", !h.hasText("账号变化", 1500));
    }
}
