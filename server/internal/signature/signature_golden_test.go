package signature

import "testing"

// 跨端互通黄金向量:Go、openssl 与 Android Signer 必须对同一输入产出同一签名。
// canonical = "GET\n/api/sdk/v2/config\ngameId=m5755-demo&sdkVersion=1.0.0\n1700000000\n"(body 为空)
// 任一端改动签名算法会破坏此向量,从而暴露两端不互通。
const (
	goldenSecret    = "m5755-dev-public-test-secret-v1"
	goldenMethod    = "GET"
	goldenPath      = "/api/sdk/v2/config"
	goldenRawQuery  = "gameId=m5755-demo&sdkVersion=1.0.0"
	goldenTimestamp = "1700000000"
	goldenSignature = "bd479bbb8ee9f66079e2300896e2c917e0be5ca976955a18af79e97a4730149e"
)

func TestGoldenVector(t *testing.T) {
	can := Canonical(goldenMethod, goldenPath, goldenRawQuery, goldenTimestamp, nil)
	got := Compute(goldenSecret, can)
	if got != goldenSignature {
		t.Fatalf("Go 签名与黄金向量不符:\n  got  %s\n  want %s", got, goldenSignature)
	}
}

// 验证 query token 字典序规范化:乱序 query 应产出与已排序 query 相同的 canonical。
func TestCanonicalSortsQuery(t *testing.T) {
	a := Canonical("GET", "/p", "sdkVersion=1.0.0&gameId=m5755-demo", "1700000000", nil)
	b := Canonical("GET", "/p", "gameId=m5755-demo&sdkVersion=1.0.0", "1700000000", nil)
	if a != b {
		t.Fatalf("query 规范化应与顺序无关:\n  a=%q\n  b=%q", a, b)
	}
}
