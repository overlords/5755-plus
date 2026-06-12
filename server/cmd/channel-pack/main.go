// Command channel-pack:渠道写入 CLI(M4 渠道三件套,平台侧)。
// 把渠道标识符写入 APK 的 v2/v3 Signing Block 自定义条目(ID 0x71777777),免重签。
// 不进入任何运行时产物(01 §4.2);M5 动态打包服务届时包装本命令。
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"m5755/server/internal/apksig"
)

// 01 §6 渠道标识符规格:1-64 ASCII(字母/数字/下划线/短横线/点),统一小写。
var channelRe = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)

func main() {
	apk := flag.String("apk", "", "输入 APK(须已 v2/v3 签名)")
	channel := flag.String("channel", "", "渠道标识符(1-64,字母/数字/下划线/短横线/点)")
	out := flag.String("out", "", "输出 APK(缺省为 <输入>-<渠道>.apk)")
	read := flag.Bool("read", false, "只读模式:打印 APK 内现有渠道条目")
	flag.Parse()

	if *apk == "" {
		fmt.Fprintln(os.Stderr, "用法:channel-pack -apk app.apk -channel guild_abc [-out out.apk] | -read")
		os.Exit(2)
	}
	if *read {
		ch, err := apksig.ReadChannel(*apk)
		if err != nil {
			fmt.Fprintln(os.Stderr, "读取失败:", err)
			os.Exit(1)
		}
		fmt.Println(ch)
		return
	}

	normalized := strings.ToLower(strings.TrimSpace(*channel))
	if !channelRe.MatchString(normalized) {
		fmt.Fprintln(os.Stderr, "渠道标识符非法:须 1-64 个 ASCII(字母/数字/下划线/短横线/点),统一小写(01 §6)")
		os.Exit(2)
	}
	dst := *out
	if dst == "" {
		dst = strings.TrimSuffix(*apk, ".apk") + "-" + normalized + ".apk"
	}
	if err := apksig.WriteChannelFile(*apk, dst, normalized); err != nil {
		fmt.Fprintln(os.Stderr, "写入失败:", err)
		os.Exit(1)
	}
	fmt.Printf("OK channel=%s out=%s(免重签:仅写 Signing Block 自定义条目)\n", normalized, dst)
}
