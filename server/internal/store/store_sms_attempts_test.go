package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// newTestStore 连接真实 Postgres(DATABASE_URL),套迁移;未设置则跳过(与 internal/api 测试同口径)。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 DATABASE_URL,跳过 store Postgres-seam 测试")
	}
	ctx := context.Background()
	st, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		st.Close()
		t.Fatalf("迁移失败: %v", err)
	}
	t.Cleanup(st.Close)
	return st
}

const smsTestGame = "m5755-demo"

// uniqueLoginAccount 每个用例用独立号,避免历史码相互干扰(ConsumeSmsCode 取"最新未消费码")。
func uniqueLoginAccount(prefix string) string {
	return prefix + newID("")
}

// TestConsumeSmsCode_OK 正确码在上限内 → OK;消费后再用同码 → Invalid(已 consumed)。
func TestConsumeSmsCode_OK(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	login := uniqueLoginAccount("ok-")

	sc, err := st.CreateSmsCode(ctx, smsTestGame, login, "123456", 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateSmsCode: %v", err)
	}

	res, err := st.ConsumeSmsCode(ctx, smsTestGame, login, sc.Code)
	if err != nil {
		t.Fatalf("ConsumeSmsCode: %v", err)
	}
	if res != SmsConsumeOK {
		t.Fatalf("正确码应 OK,得 %v", res)
	}

	// 消费后同码再用:已 consumed,查不到 → Invalid。
	res, err = st.ConsumeSmsCode(ctx, smsTestGame, login, sc.Code)
	if err != nil {
		t.Fatalf("ConsumeSmsCode 二次: %v", err)
	}
	if res != SmsConsumeInvalid {
		t.Fatalf("已消费码应 Invalid,得 %v", res)
	}
}

// TestConsumeSmsCode_AttemptCapVoidsCode 连续 smsMaxAttempts 次错误后,即便随后提交正确码也 Invalid(码已作废)。
func TestConsumeSmsCode_AttemptCapVoidsCode(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	login := uniqueLoginAccount("cap-")

	if _, err := st.CreateSmsCode(ctx, smsTestGame, login, "654321", 5*time.Minute); err != nil {
		t.Fatalf("CreateSmsCode: %v", err)
	}

	// 连续 smsMaxAttempts 次错误猜测,每次都应 Invalid。
	for i := 0; i < smsMaxAttempts; i++ {
		res, err := st.ConsumeSmsCode(ctx, smsTestGame, login, "000000")
		if err != nil {
			t.Fatalf("错误猜测 #%d: %v", i+1, err)
		}
		if res != SmsConsumeInvalid {
			t.Fatalf("错误猜测 #%d 应 Invalid,得 %v", i+1, res)
		}
	}

	// 验证计数随错误累加且达阈值已作废(无未消费码可查)。
	var attempts int
	var consumed bool
	if err := st.pool.QueryRow(ctx, `SELECT attempts, consumed FROM sms_codes
		WHERE game_id=$1 AND login_account=$2 ORDER BY created_at DESC LIMIT 1`,
		smsTestGame, login).Scan(&attempts, &consumed); err != nil {
		t.Fatalf("查 attempts/consumed: %v", err)
	}
	if attempts < smsMaxAttempts {
		t.Fatalf("attempts 应累加至 >=%d,得 %d", smsMaxAttempts, attempts)
	}
	if !consumed {
		t.Fatalf("达阈值后码应被作废 consumed=true")
	}

	// 关键断言:随后提交正确码也被拒(爆破上限生效)。
	res, err := st.ConsumeSmsCode(ctx, smsTestGame, login, "654321")
	if err != nil {
		t.Fatalf("达上限后正确码: %v", err)
	}
	if res != SmsConsumeInvalid {
		t.Fatalf("达上限后正确码应 Invalid(码已作废),得 %v", res)
	}
}

// TestConsumeSmsCode_AttemptsAccumulate 计数随每次错误猜测严格 +1(未达阈值前不作废)。
func TestConsumeSmsCode_AttemptsAccumulate(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	login := uniqueLoginAccount("acc-")

	if _, err := st.CreateSmsCode(ctx, smsTestGame, login, "111111", 5*time.Minute); err != nil {
		t.Fatalf("CreateSmsCode: %v", err)
	}

	for i := 1; i < smsMaxAttempts; i++ { // 停在阈值前,确保码未作废
		if res, err := st.ConsumeSmsCode(ctx, smsTestGame, login, "999999"); err != nil || res != SmsConsumeInvalid {
			t.Fatalf("错误猜测 #%d: res=%v err=%v", i, res, err)
		}
		var attempts int
		var consumed bool
		if err := st.pool.QueryRow(ctx, `SELECT attempts, consumed FROM sms_codes
			WHERE game_id=$1 AND login_account=$2 ORDER BY created_at DESC LIMIT 1`,
			smsTestGame, login).Scan(&attempts, &consumed); err != nil {
			t.Fatalf("查 attempts: %v", err)
		}
		if attempts != i {
			t.Fatalf("第 %d 次错误后 attempts 应为 %d,得 %d", i, i, attempts)
		}
		if consumed {
			t.Fatalf("阈值前码不应被作废(attempts=%d)", attempts)
		}
	}

	// 阈值前正确码仍可登入(确认未提前作废)。
	if res, err := st.ConsumeSmsCode(ctx, smsTestGame, login, "111111"); err != nil || res != SmsConsumeOK {
		t.Fatalf("阈值前正确码应 OK:res=%v err=%v", res, err)
	}
}

// TestConsumeSmsCode_Expired 过期码 → Expired,且不消费、不计数(维持原语义)。
func TestConsumeSmsCode_Expired(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	login := uniqueLoginAccount("exp-")

	// 负 ttl:立即过期。
	if _, err := st.CreateSmsCode(ctx, smsTestGame, login, "222222", -time.Minute); err != nil {
		t.Fatalf("CreateSmsCode: %v", err)
	}

	res, err := st.ConsumeSmsCode(ctx, smsTestGame, login, "222222")
	if err != nil {
		t.Fatalf("ConsumeSmsCode: %v", err)
	}
	if res != SmsConsumeExpired {
		t.Fatalf("过期码应 Expired,得 %v", res)
	}
	var attempts int
	var consumed bool
	if err := st.pool.QueryRow(ctx, `SELECT attempts, consumed FROM sms_codes
		WHERE game_id=$1 AND login_account=$2 ORDER BY created_at DESC LIMIT 1`,
		smsTestGame, login).Scan(&attempts, &consumed); err != nil {
		t.Fatalf("查 attempts: %v", err)
	}
	if attempts != 0 || consumed {
		t.Fatalf("过期分支不应计数/消费,得 attempts=%d consumed=%v", attempts, consumed)
	}
}

// TestConsumeDeviceCode_SharesAttemptCap 设备码路径委托 ConsumeSmsCode,同受 per-code 上限保护。
func TestConsumeDeviceCode_SharesAttemptCap(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	login := uniqueLoginAccount("dev-")

	if _, err := st.CreateSmsCode(ctx, smsTestGame, login, "333333", 5*time.Minute); err != nil {
		t.Fatalf("CreateSmsCode: %v", err)
	}

	for i := 0; i < smsMaxAttempts; i++ {
		if res, err := st.ConsumeDeviceCode(ctx, smsTestGame, login, "000000"); err != nil || res != SmsConsumeInvalid {
			t.Fatalf("设备码错误猜测 #%d: res=%v err=%v", i+1, res, err)
		}
	}

	// 达上限后正确设备码也被拒。
	if res, err := st.ConsumeDeviceCode(ctx, smsTestGame, login, "333333"); err != nil || res != SmsConsumeInvalid {
		t.Fatalf("达上限后正确设备码应 Invalid:res=%v err=%v", res, err)
	}
}
