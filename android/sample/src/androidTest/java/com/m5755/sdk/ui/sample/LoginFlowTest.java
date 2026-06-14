package com.m5755.sdk.ui.sample;

import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertTrue;

import androidx.test.ext.junit.runners.AndroidJUnit4;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;

/**
 * 正常链仪器化(#15/#16/#17/#18 DoD):对接线上 sdk-dev,真实新用户全链——
 * 冷启动 → 协议 → 登录窗 → devCode 登录 → 实名页 → 小号选择页 → 小号登录 → 登录态校验。
 */
@RunWith(AndroidJUnit4.class)
public class LoginFlowTest {

    private final TestHarness h = new TestHarness();

    @Before
    public void setUp() throws Exception {
        h.reset();
        h.freshLaunch();
    }

    @After
    public void tearDown() throws Exception {
        h.reset();
    }

    @Test
    public void fullChainNewUserToSubaccountLogin() throws Exception {
        h.toLoginWindow();
        h.doSmsLogin(TestHarness.uniquePhone());

        // 新账户 → 实名页(#16)
        assertTrue("新账户应进入实名页", h.hasText("实名认证", TestHarness.WAIT));
        h.waitText("请输入真实姓名").setText("测试玩家");
        h.waitText("请输入身份证号").setText("11010119900101001X");
        h.tapExact("提交");

        // 实名通过 → 小号选择页(#17;新用户无默认)
        assertTrue("实名通过应进小号选择页", h.hasText("选择小号进入游戏", TestHarness.WAIT));
        assertTrue("应展示平台建档的首个小号", h.hasText("小号1", TestHarness.WAIT));

        // 点小号行进入 → 小号登录 → 登录态校验弹窗(#18)
        h.tapExact("小号1");
        assertTrue("应展示登录态校验弹窗", h.hasText("登录态校验通过", TestHarness.WAIT));
        assertTrue("详情应含脱敏令牌", h.hasText("登录令牌", TestHarness.WAIT));
        h.tapExact("进入游戏");

        // 公开口径:shared_prefs 持久化三件套(08 §2.2)
        assertNotNull(h.prefs("platform_account_id"));
        assertNotNull(h.prefs("platform_token"));
        assertNotNull("account 应为小号 ID", h.prefs("account"));
        assertNotNull("小号令牌应持久化", h.prefs("sub_token"));

        // 样例面板状态:登录成功回调已到
        assertTrue("DataListener 应收到 User", h.hasText("登录成功 account=", TestHarness.WAIT));
    }

    @Test
    public void autoLoginSkipsLoginWindowAfterRestart() throws Exception {
        // 先完整登录一次(设默认以走自动进入提示)
        h.toLoginWindow();
        h.doSmsLogin(TestHarness.uniquePhone());
        h.waitText("请输入真实姓名").setText("测试玩家");
        h.waitText("请输入身份证号").setText("11010119900101001X");
        h.tapExact("提交");
        assertTrue(h.hasText("选择小号进入游戏", TestHarness.WAIT));
        h.tapExact("默认"); // 显式设默认(默认标签,非小号行;tip 文案含「默认」须精确匹配)
        // #4 重做后徽标 = 独立 radio「✓」+ label「默认」两个 TextView;选中态断言 ✓ 勾选出现
        assertTrue("设默认后应出现勾选态(✓)", h.hasText("✓", TestHarness.WAIT));
        h.tapExact("小号1");
        assertTrue(h.hasText("登录态校验通过", TestHarness.WAIT));
        h.tapExact("进入游戏");

        // 重启(不清数据)→ 自动登录 → 不弹登录窗 → 自动进入提示(#15 + 03 §2.8)
        h.launch();
        h.tapText("进入游戏(onGameStart");
        assertTrue(h.hasText("init 成功", TestHarness.WAIT));
        h.tapText("登录(login");
        assertTrue("已设默认应展示自动进入提示", h.hasText("将以「小号1」进入游戏", TestHarness.WAIT));
        // 1800ms 后自动进入 → 登录态校验
        assertTrue(h.hasText("登录态校验通过", TestHarness.WAIT));
    }
}
