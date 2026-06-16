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
 * 异常路径仪器化回归(#19,上线阻断):dev 控制面驱动,03 §3 分流矩阵断言。
 * 「账号变化」断言读样例面板状态文本(UserListener.onLogout → "账号变化:已登出/失效")。
 */
@RunWith(AndroidJUnit4.class)
public class ExceptionPathTest {

    private final TestHarness h = new TestHarness();

    @Before
    public void setUp() throws Exception {
        h.reset();
    }

    @After
    public void tearDown() throws Exception {
        h.reset();
    }

    /** 完整登录到拿小号,返回 platformAccountId。 */
    private String loginToGame() throws Exception {
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
        String paId = h.prefs("platform_account_id");
        assertNotNull(paId);
        return paId;
    }

    @Test
    public void maintenanceBlocksWithoutAccountChange() throws Exception {
        h.setMaintenance(true, "维护演练中");
        try {
            h.freshLaunch();
            h.tapText("进入游戏(init");
            assertTrue(h.hasText("init 成功", TestHarness.WAIT));
            h.tapText("登录(login");
            assertTrue("应展示维护门禁", h.hasText("维护门禁", TestHarness.WAIT));
            assertTrue(h.hasText("维护演练中", TestHarness.WAIT));
            assertFalse("维护阻断不触发账号变化", h.hasText("账号变化", 1500));
        } finally {
            h.reset();
        }
    }

    @Test
    public void kickRoutesToLoginWindowWithAccountChange() throws Exception {
        String paId = loginToGame();
        h.kick(paId);
        // 重启触发自动登录 → 服务端校验失败 → 回登录窗 + 账号变化
        h.launch();
        h.tapText("进入游戏(init");
        assertTrue(h.hasText("init 成功", TestHarness.WAIT));
        h.tapText("登录(login");
        assertTrue("踢号后应回登录窗", h.hasText("验证码登录", TestHarness.WAIT));
        assertTrue("踢号应触发账号变化", h.hasText("账号变化", TestHarness.WAIT));
    }

    @Test
    public void invalidSubaccountRoutesToPickerNotLoginWindow() throws Exception {
        loginToGame();
        String account = h.prefs("account"); // 当前小号 = 小号1
        assertNotNull(account);
        // 切换小号 → 选择页;先加小号2,保证失效一个后列表仍非空
        h.tapText("切换小号(changeUser)");
        assertTrue(h.hasText("选择小号进入游戏", TestHarness.WAIT));
        h.tapExact("添加小号");
        assertTrue("应出现小号2", h.hasText("小号2", TestHarness.WAIT));
        // 失效小号1(渲染中的行仍可点),点它 → subaccount_invalid → 回选择页(剩小号2)
        h.invalidateSubaccount(account);
        h.tapExact("小号1");
        assertTrue("小号失效应回选择页", h.hasText("选择小号进入游戏", TestHarness.WAIT));
        assertFalse("不得回登录窗(03 §3 分流)", h.hasText("验证码登录", 1500));
    }

    @Test
    public void antiAddictionEntryBlockNoAccountChange() throws Exception {
        String paId = loginToGame();
        h.antiAddiction(paId, true, true);
        // 重启 → 自动登录有效 → 实名门禁阻断 → 仅提示
        h.launch();
        h.tapText("进入游戏(init");
        assertTrue(h.hasText("init 成功", TestHarness.WAIT));
        h.tapText("登录(login");
        assertTrue("应展示防沉迷提示", h.hasText("防沉迷提示", TestHarness.WAIT));
        assertFalse("防沉迷阻断不触发账号变化", h.hasText("账号变化", 1500));
        assertFalse("不得回登录窗", h.hasText("验证码登录", 1500));
    }
}
