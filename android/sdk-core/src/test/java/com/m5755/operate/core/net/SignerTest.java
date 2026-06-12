package com.m5755.operate.core.net;

import static org.junit.Assert.assertEquals;

import org.junit.Test;

/**
 * 跨端互通黄金向量:与平台服务端 Go 端 signature_golden_test.go 及 openssl 同值。
 * 任一端改签名算法都会让此测试失败,从而在合并前暴露两端不互通。
 */
public class SignerTest {

    private static final String SECRET = "m5755-dev-public-test-secret-v1";
    private static final String GOLDEN = "bd479bbb8ee9f66079e2300896e2c917e0be5ca976955a18af79e97a4730149e";

    @Test
    public void goldenVectorMatchesServer() {
        String canonical = Signer.canonical("GET", "/api/sdk/v2/config",
                "gameId=m5755-demo&sdkVersion=1.0.0", "1700000000", null);
        assertEquals(GOLDEN, Signer.compute(SECRET, canonical));
    }

    @Test
    public void queryOrderDoesNotMatter() {
        String a = Signer.canonical("GET", "/p", "sdkVersion=1.0.0&gameId=m5755-demo", "1700000000", null);
        String b = Signer.canonical("GET", "/p", "gameId=m5755-demo&sdkVersion=1.0.0", "1700000000", null);
        assertEquals(a, b);
    }

    @Test
    public void postBodyParticipatesInSignature() {
        String withBody = Signer.canonical("POST", "/api/sdk/v2/sms-codes", "",
                "1700000000", "{\"gameId\":\"m5755-demo\"}");
        String empty = Signer.canonical("POST", "/api/sdk/v2/sms-codes", "", "1700000000", "");
        // body 不同则 canonical 不同 → 签名不同
        assertEquals(false, Signer.compute(SECRET, withBody).equals(Signer.compute(SECRET, empty)));
    }
}
