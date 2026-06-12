//go:build production

package main

import (
	"context"
	"log"
	"os"

	"m5755/server/internal/domain"
	"m5755/server/internal/sms"
	"m5755/server/internal/store"
)

// bootstrapEnv production 构建:fail-closed——缺生产密钥拒绝启动;
// 注入生产验签密钥并停用其它所有 key(占位/历史/dev 测试),杜绝后门;
// 不种任何 dev 测试账户;实名/短信 mock 关闭——实名提交与短信发送在未配置真实 provider 时明确失败,
// 生产绝不返回 devCode。
func bootstrapEnv(ctx context.Context, st *store.Store, platformEnv string) domain.Options {
	cs := os.Getenv("CALLBACK_SECRET")
	keyID := os.Getenv("SIGNING_KEY_ID")
	secret := os.Getenv("SIGNING_KEY_SECRET")
	if cs == "" || keyID == "" || secret == "" {
		log.Fatal("生产启动缺少 CALLBACK_SECRET / SIGNING_KEY_ID / SIGNING_KEY_SECRET(fail-closed)")
	}
	if err := st.UpsertSigningKey(ctx, keyID, secret); err != nil {
		log.Fatalf("生产验签密钥注入失败: %v", err)
	}
	// 停用除当前 keyId 外的所有密钥(dev-test-key、prod-key-placeholder 等),杜绝残留后门。
	if err := st.DeactivateOtherSigningKeys(ctx, keyID); err != nil {
		log.Fatalf("停用历史验签密钥失败: %v", err)
	}
	smsCfg := sms.Config{
		AccessKeyID:     os.Getenv("JDCLOUD_SMS_ACCESS_KEY_ID"),
		AccessKeySecret: os.Getenv("JDCLOUD_SMS_ACCESS_KEY_SECRET"),
		SignID:          os.Getenv("JDCLOUD_SMS_SIGN_ID"),
		TemplateID:      os.Getenv("JDCLOUD_SMS_TEMPLATE_ID"),
		Region:          os.Getenv("JDCLOUD_SMS_REGION"),
		Endpoint:        os.Getenv("JDCLOUD_SMS_ENDPOINT"),
	}
	if fails := smsCfg.Validate(); len(fails) > 0 {
		// 不 fatal:服务可起(healthz/其它端点可用),但 /sms-codes 将 fail-closed 503;
		// 生产绝不退回 mock、绝不返回 devCode。提示运维补齐京东云凭据。
		log.Printf("警告:京东云短信凭据未就绪 %v —— /sms-codes 将 fail-closed,短信登录不可用直至补齐", fails)
	}
	return domain.Options{
		CallbackSecret: cs,
		RealNameMock:   false, // 生产实名 = fail-closed
		SmsMock:        false, // 生产短信 = 京东云真发,绝不返回 devCode
		SmsConfig:      smsCfg,
	}
}
