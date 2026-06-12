package apksig

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"testing"
)

// buildFakeApk 构造带最小 Signing Block 的合成 APK 字节(结构级测试,不依赖真实签名)。
func buildFakeApk(t *testing.T, extraPairs []pair) []byte {
	t.Helper()
	var buf bytes.Buffer
	// 假 ZIP 本地区(内容不重要,结构测试只关心 Block/EOCD)
	buf.WriteString("PK\x03\x04fakelocaldata")

	// Signing Block:一个假 v2 签名条目 + extra
	var payload bytes.Buffer
	pairs := append([]pair{{id: 0x7109871a, value: []byte("fake-v2-sig")}}, extraPairs...)
	for _, p := range pairs {
		_ = binary.Write(&payload, binary.LittleEndian, uint64(4+len(p.value)))
		_ = binary.Write(&payload, binary.LittleEndian, p.id)
		payload.Write(p.value)
	}
	size := uint64(payload.Len() + 24)
	sigStart := buf.Len()
	_ = binary.Write(&buf, binary.LittleEndian, size)
	buf.Write(payload.Bytes())
	_ = binary.Write(&buf, binary.LittleEndian, size)
	buf.WriteString(magic)

	cd := buf.Len()
	buf.WriteString("PK\x01\x02fakecentraldir")
	eocd := buf.Len()
	// EOCD
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0x06054b50))
	buf.Write(make([]byte, 12)) // disk fields + counts + cd size(占位)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(cd))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0)) // comment len
	_ = sigStart
	_ = eocd
	return buf.Bytes()
}

func TestWriteReadRoundTrip(t *testing.T) {
	apk := buildFakeApk(t, nil)
	out, err := WriteChannel(apk, "guild_abc")
	if err != nil {
		t.Fatal(err)
	}
	f := t.TempDir() + "/x.apk"
	if err := os.WriteFile(f, out, 0o644); err != nil {
		t.Fatal(err)
	}
	ch, err := ReadChannel(f)
	if err != nil {
		t.Fatal(err)
	}
	if ch != "guild_abc" {
		t.Fatalf("读回应为 guild_abc,得 %q", ch)
	}
}

func TestWriteIdempotentReplace(t *testing.T) {
	apk := buildFakeApk(t, nil)
	out1, _ := WriteChannel(apk, "first")
	out2, err := WriteChannel(out1, "second")
	if err != nil {
		t.Fatal(err)
	}
	l, _ := parse(out2)
	ps, _ := l.pairs()
	count := 0
	for _, p := range ps {
		if p.id == ChannelBlockID || p.id == ChannelBlockIDAlt {
			count++
			if string(p.value) != "second" {
				t.Fatalf("应替换为 second,得 %q", p.value)
			}
		}
	}
	if count != 1 {
		t.Fatalf("渠道条目应恰一条(幂等替换),得 %d", count)
	}
	// 假 v2 签名条目仍在(不动签名条目)
	found := false
	for _, p := range ps {
		if p.id == 0x7109871a && string(p.value) == "fake-v2-sig" {
			found = true
		}
	}
	if !found {
		t.Fatal("签名条目不得被改动")
	}
}

func TestAltBlockIDReadable(t *testing.T) {
	apk := buildFakeApk(t, []pair{{id: ChannelBlockIDAlt, value: []byte("legacy_ch")}})
	f := t.TempDir() + "/y.apk"
	_ = os.WriteFile(f, apk, 0o644)
	ch, err := ReadChannel(f)
	if err != nil {
		t.Fatal(err)
	}
	if ch != "legacy_ch" {
		t.Fatalf("兼容 ID 应可读,得 %q", ch)
	}
}

// TestRealApkSignerVerify 真实 APK 链路:写入渠道后 apksigner verify 仍通过(免重签硬证据)。
// 需要 M5755_TEST_APK(已签名 APK)与 apksigner 在 PATH/ANDROID_HOME;缺省跳过。
func TestRealApkSignerVerify(t *testing.T) {
	apkPath := os.Getenv("M5755_TEST_APK")
	apksigner := os.Getenv("APKSIGNER")
	if apkPath == "" || apksigner == "" {
		t.Skip("未设置 M5755_TEST_APK/APKSIGNER,跳过真实 APK 验证")
	}
	out := t.TempDir() + "/channeled.apk"
	if err := WriteChannelFile(apkPath, out, "guild_e2e_test"); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command(apksigner, "verify", out).Run(); err != nil {
		t.Fatalf("写入渠道后 apksigner verify 失败(免重签主张被打破): %v", err)
	}
	ch, _ := ReadChannel(out)
	if ch != "guild_e2e_test" {
		t.Fatalf("真实 APK 读回渠道错: %q", ch)
	}
}
