package com.m5755.sdk.ui;

import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.junit.Test;

/**
 * 支付域外跳 scheme 白名单收窄回归(01 §4.2 受限例外 / ADR-0014)。
 * 锁住「仅微信/支付宝付款 scheme 放行,危险/未知 scheme 一律拦」,
 * 防未来有人放宽前缀(如改成 startsWith("weixin") 漏掉 ://、或新增 intent://)无声破防。
 */
public class SchemeWhitelistTest {

    @Test
    public void allowsOnlyWechatAndAlipayPaySchemes() {
        assertTrue(SdkUi.isPaySchemeWhitelisted("weixin://wap/pay?prepayid=x"));
        assertTrue(SdkUi.isPaySchemeWhitelisted("weixin://"));
        assertTrue(SdkUi.isPaySchemeWhitelisted("alipays://platformapi/startapp"));
        assertTrue(SdkUi.isPaySchemeWhitelisted("alipayqr://platformapi/startapp"));
    }

    @Test
    public void rejectsDangerousAndUnknownSchemes() {
        // intent:// 重定向攻击(可指定任意 component/action)必须被拦在 startActivity 之外
        assertFalse(SdkUi.isPaySchemeWhitelisted("intent://evil#Intent;action=android.intent.action.CALL;end"));
        // 通用外跳 / 本地资源 / 脚本 / 应用市场:01 §4.2 通用外跳仍永久排除
        assertFalse(SdkUi.isPaySchemeWhitelisted("http://example.com"));
        assertFalse(SdkUi.isPaySchemeWhitelisted("https://example.com"));
        assertFalse(SdkUi.isPaySchemeWhitelisted("file:///etc/hosts"));
        assertFalse(SdkUi.isPaySchemeWhitelisted("javascript:alert(1)"));
        assertFalse(SdkUi.isPaySchemeWhitelisted("market://details?id=com.x"));
        assertFalse(SdkUi.isPaySchemeWhitelisted("mqqapi://"));  // QQ 钱包不在 v2 范围
        assertFalse(SdkUi.isPaySchemeWhitelisted("alipay://"));  // 无 s,非标准支付 scheme、非白名单
        assertFalse(SdkUi.isPaySchemeWhitelisted(""));
        assertFalse(SdkUi.isPaySchemeWhitelisted(null));
    }
}
