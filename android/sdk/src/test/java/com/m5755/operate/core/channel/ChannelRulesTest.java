package com.m5755.operate.core.channel;

import static org.junit.Assert.assertEquals;

import org.junit.Test;

import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;

/** 01 §6 渠道规则矩阵 + 跨端黄金向量(channel-pack 产出的夹具)。 */
public class ChannelRulesTest {

    private static ChannelRules.Result d(String m, String b) {
        return ChannelRules.decide(m, b, false);
    }

    @Test
    public void normalizationMatrix() {
        assertEquals("guild_abc", d("GUILD_ABC", null).resolved); // 大写归一
        assertEquals("a.b-c_1", d(" a.b-c_1 ", null).resolved);   // trim
        assertEquals(ChannelRules.SOURCE_MANIFEST, d("ch1", null).source);
        // 非法字符 → invalid_format + default
        ChannelRules.Result bad = d("非法!", null);
        assertEquals(ChannelRules.DEFAULT, bad.resolved);
        assertEquals(ChannelRules.REASON_INVALID_FORMAT, bad.reason);
        // 超长 65 → invalid
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < 65; i++) {
            sb.append('a');
        }
        assertEquals(ChannelRules.REASON_INVALID_FORMAT, d(sb.toString(), null).reason);
        // 恰 64 合法
        assertEquals("", d(sb.substring(0, 64), null).reason);
    }

    @Test
    public void dualSourceRules() {
        // 一致(归一后)→ signing_block 来源
        ChannelRules.Result ok = d("CH_X", "ch_x");
        assertEquals("ch_x", ok.resolved);
        assertEquals(ChannelRules.SOURCE_SIGNING_BLOCK, ok.source);
        assertEquals("", ok.reason);
        // 不一致 → default + source_mismatch
        ChannelRules.Result mm = d("ch_a", "ch_b");
        assertEquals(ChannelRules.DEFAULT, mm.resolved);
        assertEquals(ChannelRules.REASON_SOURCE_MISMATCH, mm.reason);
        // 仅块
        assertEquals(ChannelRules.SOURCE_SIGNING_BLOCK, d(null, "blk").source);
        // 全缺 → missing
        ChannelRules.Result none = d(null, null);
        assertEquals(ChannelRules.DEFAULT, none.resolved);
        assertEquals(ChannelRules.REASON_MISSING, none.reason);
        // 块不可读且无 manifest → unreadable
        assertEquals(ChannelRules.REASON_UNREADABLE, ChannelRules.decide(null, null, true).reason);
    }

    /** 跨端黄金向量:channel-pack(Go)写出的夹具,Java 解析端读出同值。 */
    @Test
    public void goldenFixtureFromChannelPack() throws Exception {
        InputStream in = getClass().getClassLoader().getResourceAsStream("channeled-fixture.apk");
        File tmp = File.createTempFile("fixture", ".apk");
        tmp.deleteOnExit();
        FileOutputStream out = new FileOutputStream(tmp);
        byte[] buf = new byte[4096];
        int n;
        while ((n = in.read(buf)) != -1) {
            out.write(buf, 0, n);
        }
        out.close();
        in.close();
        assertEquals("golden_channel", ApkSigningBlock.readChannel(tmp.getAbsolutePath()));
    }
}
