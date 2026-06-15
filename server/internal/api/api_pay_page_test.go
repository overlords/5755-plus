package api

import (
	"strings"
	"testing"
)

// TestJsString_EscapesJSBreakouts 锁定收银台 JS 注入纵深防御(grill「手写 SQL 安全」补 nit):
// </script> 逃逸与 U+2028/U+2029 行终止符都必须被转义。用 rune(0x2028) 构造,源码纯 ASCII。
func TestJsString_EscapesJSBreakouts(t *testing.T) {
	sep2028 := string(rune(0x2028))
	sep2029 := string(rune(0x2029))
	out := jsString("a" + sep2028 + sep2029 + "b</script><x>'\"\\")

	if strings.Contains(out, sep2028) || strings.Contains(out, sep2029) {
		t.Errorf("U+2028/U+2029 未转义:%q", out)
	}
	if strings.Contains(out, "</script>") {
		t.Errorf("</script> 未转义(script 逃逸):%q", out)
	}
	if !strings.HasPrefix(out, `"`) || !strings.HasSuffix(out, `"`) {
		t.Errorf("应为合法双引号 JS 字面量:%q", out)
	}
}
