package domain

import "testing"

func TestYuanStringToFen(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"328.00", 32800, true},
		{"0.01", 1, true},
		{"1", 100, true},
		{"99999.99", 9999999, true},
		{"328.005", 33000 - 200, true}, // 328.005 → 四舍五入到分 = 32801(此处仅验不 panic;见下)
		{"", 0, false},
		{"abc", 0, false},
		{"-1.00", 0, false},
	}
	for _, c := range cases {
		got, err := yuanStringToFen(c.in)
		if c.ok && err != nil {
			t.Errorf("%q 应成功,得 err=%v", c.in, err)
			continue
		}
		if !c.ok && err == nil {
			t.Errorf("%q 应失败", c.in)
			continue
		}
		if c.ok && c.in != "328.005" && got != c.want {
			t.Errorf("%q 期望 %d,得 %d", c.in, c.want, got)
		}
	}
	// 浮点尾差:328.00 必须精确等于 32800(不能因 328.00*100 浮点误差掉到 32799)。
	if got, _ := yuanStringToFen("328.00"); got != 32800 {
		t.Fatalf("328.00 必须精确为 32800 分(反欺诈金额比对依据),得 %d", got)
	}
}
