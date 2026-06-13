package nppa

import (
	"strings"
	"testing"
)

// 规范 V2.1 官方示例黄金向量。
const (
	gSecretKey  = "b4c932250dbe59ff53b15ee993a9feb5"
	gAppID      = "fdc6688637bc468e9aea874654cbead2"
	gBizID      = "1101999999"
	gCipherText = "kGuV06piX8av9vsZGofHI1viPrHG/IpjsGGu75DYmRyQx6UEvPXrKkAdwWs3SmzEQ5GctOK/N5x5J4Yykw61plWqIL/PytfMZfcnqM43+HmW04agmLU6TJ1ydUnirDl8xGiofmrLLg=="
)

// 认证接口签名黄金向量。
func TestSignAuthGolden(t *testing.T) {
	body := `{"data":"` + gCipherText + `"}`
	got := Sign(gSecretKey, map[string]string{
		"appId": gAppID, "bizId": gBizID, "timestamps": "1705975788903",
	}, body)
	want := "13806865bd4ea428a6c28e85c2e37c3d9bf8748d825f1f475e2252f527394b20"
	if got != want {
		t.Fatalf("认证签名不匹配\n got=%s\nwant=%s", got, want)
	}
}

// 查询接口签名黄金向量(含 ai,无 body)。
func TestSignQueryGolden(t *testing.T) {
	got := Sign(gSecretKey, map[string]string{
		"ai": "100000000000000001", "appId": gAppID, "bizId": gBizID, "timestamps": "1705975247808",
	}, "")
	want := "1a2cabf56d3dbc961c35692092fbcbe911d748c8505418e796f2af8369ea0475"
	if got != want {
		t.Fatalf("查询签名不匹配\n got=%s\nwant=%s", got, want)
	}
}

// AES-128-GCM 布局黄金向量:用官方 secretKey 解官方密文,应得官方明文(证 IV/tag 布局与 NPPA 一致)。
func TestDecryptGolden(t *testing.T) {
	pt, err := Decrypt(gSecretKey, gCipherText)
	if err != nil {
		t.Fatalf("解密官方密文失败(AES 布局可能不符): %v", err)
	}
	if !strings.Contains(pt, `"ai":"100000000000000001"`) || !strings.Contains(pt, `"idNum":"110000190101010001"`) {
		t.Fatalf("解出明文与官方示例不符: %s", pt)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plain := `{"ai":"abc123","name":"测试","idNum":"11010119900307001X"}`
	enc, err := Encrypt(gSecretKey, plain)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decrypt(gSecretKey, enc)
	if err != nil {
		t.Fatal(err)
	}
	if back != plain {
		t.Fatalf("往返不一致: %s", back)
	}
}
