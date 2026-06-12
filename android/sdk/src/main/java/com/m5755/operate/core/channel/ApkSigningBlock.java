package com.m5755.operate.core.channel;

import java.io.RandomAccessFile;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.Charset;

/**
 * APK Signing Block 渠道条目读取(纯解析,JVM 可测)。
 * 块格式契约(与平台侧 channel-pack 写入端共享,跨端黄金向量钉死):
 * 主 ID 0x71777777,兼容 ID 0x57550001,值为 UTF-8 渠道串。
 */
public final class ApkSigningBlock {

    static final int CHANNEL_ID = 0x71777777;
    static final int CHANNEL_ID_ALT = 0x57550001;
    private static final String MAGIC = "APK Sig Block 42";
    private static final Charset UTF8 = Charset.forName("UTF-8");

    /** 从 APK 文件读渠道条目;无块/无条目返回 null;IO/结构异常抛出(上层归 unreadable)。 */
    public static String readChannel(String apkPath) throws Exception {
        RandomAccessFile f = new RandomAccessFile(apkPath, "r");
        try {
            long len = f.length();
            // 定位 EOCD(向前扫描注释区)
            long scanFrom = Math.max(0, len - 22 - 65535);
            long eocd = -1;
            byte[] tail = new byte[(int) (len - scanFrom)];
            f.seek(scanFrom);
            f.readFully(tail);
            for (int i = tail.length - 22; i >= 0; i--) {
                if (readLeInt(tail, i) == 0x06054b50) {
                    eocd = scanFrom + i;
                    break;
                }
            }
            if (eocd < 0) {
                throw new IllegalStateException("no EOCD");
            }
            long cd = readLeInt(tail, (int) (eocd - scanFrom) + 16) & 0xFFFFFFFFL;
            if (cd < 24) {
                return null;
            }
            // 块尾 24 字节:size+magic
            byte[] tailBlock = new byte[24];
            f.seek(cd - 24);
            f.readFully(tailBlock);
            byte[] magicBytes = new byte[16];
            System.arraycopy(tailBlock, 8, magicBytes, 0, 16);
            if (!MAGIC.equals(new String(magicBytes, UTF8))) {
                return null; // 无 Signing Block(v1-only 包):按缺失处理
            }
            long size = readLeLong(tailBlock, 0);
            long start = cd - size - 8;
            if (start < 0) {
                throw new IllegalStateException("bad block size");
            }
            f.seek(start);
            byte[] head = new byte[8];
            f.readFully(head);
            if (readLeLong(head, 0) != size) {
                throw new IllegalStateException("size mismatch");
            }
            long payloadLen = size - 24;
            byte[] payload = new byte[(int) payloadLen];
            f.readFully(payload);

            String alt = null;
            ByteBuffer bb = ByteBuffer.wrap(payload).order(ByteOrder.LITTLE_ENDIAN);
            while (bb.remaining() >= 12) {
                long plen = bb.getLong();
                int id = bb.getInt();
                int vlen = (int) (plen - 4);
                if (vlen < 0 || vlen > bb.remaining()) {
                    throw new IllegalStateException("bad pair");
                }
                byte[] val = new byte[vlen];
                bb.get(val);
                if (id == CHANNEL_ID) {
                    return new String(val, UTF8);
                }
                if (id == CHANNEL_ID_ALT) {
                    alt = new String(val, UTF8);
                }
            }
            return alt;
        } finally {
            f.close();
        }
    }

    private static int readLeInt(byte[] b, int off) {
        return (b[off] & 0xFF) | ((b[off + 1] & 0xFF) << 8) | ((b[off + 2] & 0xFF) << 16) | ((b[off + 3] & 0xFF) << 24);
    }

    private static long readLeLong(byte[] b, int off) {
        long lo = readLeInt(b, off) & 0xFFFFFFFFL;
        long hi = readLeInt(b, off + 4) & 0xFFFFFFFFL;
        return (hi << 32) | lo;
    }

    private ApkSigningBlock() {
    }
}
