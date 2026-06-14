package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---------- #21 角色快照 ----------

type RoleSnapshot struct {
	Account, GameID, ServerID, ServerName, RoleID, RoleName, RoleLevel string
	RoleCE, RoleStage, RoleRechargeAmount, RoleGuild                   string
}

func (s *Store) UpsertRole(ctx context.Context, r RoleSnapshot) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO roles
		(account, game_id, server_id, role_id, server_name, role_name, role_level, role_ce, role_stage, role_recharge_amount, role_guild, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())
		ON CONFLICT (account, game_id, server_id, role_id) DO UPDATE SET
		  server_name=$5, role_name=$6, role_level=$7, role_ce=$8, role_stage=$9, role_recharge_amount=$10, role_guild=$11, updated_at=now()`,
		r.Account, r.GameID, r.ServerID, r.RoleID, r.ServerName, r.RoleName, r.RoleLevel,
		r.RoleCE, r.RoleStage, r.RoleRechargeAmount, r.RoleGuild)
	return err
}

// SubaccountByToken 按小号令牌取 account/归属(角色上报与支付的鉴权)。
func (s *Store) SubaccountByToken(ctx context.Context, token, gameID string) (account, platformAccountID string, ok bool, err error) {
	var revoked bool
	var exp time.Time
	var active bool
	e := s.pool.QueryRow(ctx, `SELECT ss.account, ss.platform_account_id, ss.revoked, ss.expires_at, sa.active
		FROM subaccount_sessions ss JOIN subaccounts sa ON sa.account = ss.account
		WHERE ss.token=$1 AND ss.game_id=$2`, token, gameID).Scan(&account, &platformAccountID, &revoked, &exp, &active)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if e != nil {
		return "", "", false, e
	}
	if revoked || !active || time.Now().After(exp) {
		return "", "", false, nil
	}
	return account, platformAccountID, true, nil
}

// ---------- #22 订单 ----------

type Order struct {
	PlatformOrderID, CPOrderID, Account, GameID, PlatformAccountID string
	Amount                                                         string
	Commodity, ServerID, ServerName, RoleID, RoleName, RoleLevel   string
	PaymentStatus, CallbackStatus                                  string
}

func (s *Store) CreateOrder(ctx context.Context, o Order) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO orders
		(platform_order_id, cp_order_id, account, game_id, platform_account_id, amount, commodity, server_id, server_name, role_id, role_name, role_level)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		o.PlatformOrderID, o.CPOrderID, o.Account, o.GameID, o.PlatformAccountID, o.Amount,
		o.Commodity, o.ServerID, o.ServerName, o.RoleID, o.RoleName, o.RoleLevel)
	return err
}

func (s *Store) GetOrder(ctx context.Context, platformOrderID string) (*Order, error) {
	var o Order
	err := s.pool.QueryRow(ctx, `SELECT platform_order_id, cp_order_id, account, game_id, platform_account_id,
		amount::text, commodity, server_id, server_name, role_id, role_name, role_level, payment_status, callback_status
		FROM orders WHERE platform_order_id=$1`, platformOrderID).Scan(
		&o.PlatformOrderID, &o.CPOrderID, &o.Account, &o.GameID, &o.PlatformAccountID, &o.Amount,
		&o.Commodity, &o.ServerID, &o.ServerName, &o.RoleID, &o.RoleName, &o.RoleLevel, &o.PaymentStatus, &o.CallbackStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Store) GetOrderForGame(ctx context.Context, platformOrderID, gameID string) (*Order, error) {
	o, err := s.GetOrder(ctx, platformOrderID)
	if err != nil {
		return nil, err
	}
	if o.GameID != gameID {
		return nil, ErrNotFound
	}
	return o, nil
}

func (s *Store) UpdateOrderStatus(ctx context.Context, platformOrderID, paymentStatus, callbackStatus string) error {
	_, err := s.pool.Exec(ctx, `UPDATE orders SET payment_status=$2, callback_status=$3 WHERE platform_order_id=$1`,
		platformOrderID, paymentStatus, callbackStatus)
	return err
}

// ListUndeliveredPaidOrders 取已支付但充值回调未送达(投递失败/投递中)的订单,供平台侧重投巡检。
// 渠道确认支付后即 ACK 止重推,出站充值回调的最终送达由本巡检补偿,不依赖渠道重推。
func (s *Store) ListUndeliveredPaidOrders(ctx context.Context, limit int) ([]Order, error) {
	rows, err := s.pool.Query(ctx, `SELECT platform_order_id, cp_order_id, account, game_id, platform_account_id,
		amount::text, commodity, server_id, server_name, role_id, role_name, role_level, payment_status, callback_status
		FROM orders WHERE payment_status='已支付' AND callback_status IN ('投递失败','投递中')
		ORDER BY platform_order_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.PlatformOrderID, &o.CPOrderID, &o.Account, &o.GameID, &o.PlatformAccountID, &o.Amount,
			&o.Commodity, &o.ServerID, &o.ServerName, &o.RoleID, &o.RoleName, &o.RoleLevel, &o.PaymentStatus, &o.CallbackStatus); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ---------- #23 充值回调配置 ----------

func (s *Store) GetCallbackURL(ctx context.Context, gameID string) (string, error) {
	var url string
	err := s.pool.QueryRow(ctx, `SELECT callback_url FROM games WHERE game_id=$1`, gameID).Scan(&url)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return url, err
}

// SetCallbackURL 测试夹具用:把回调投递目标指向测试接收端。
func (s *Store) SetCallbackURL(ctx context.Context, gameID, url string) error {
	_, err := s.pool.Exec(ctx, `UPDATE games SET callback_url=$2 WHERE game_id=$1`, gameID, url)
	return err
}

// ---------- #25 密码 / 设备信任 ----------

func (s *Store) GetPasswordHash(ctx context.Context, platformAccountID string) (string, error) {
	var h string
	err := s.pool.QueryRow(ctx, `SELECT password_hash FROM platform_accounts WHERE platform_account_id=$1`, platformAccountID).Scan(&h)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return h, err
}

// FindAccountByLogin 按登录标识(手机号/账号)取账户与密码哈希。
func (s *Store) FindAccountByLogin(ctx context.Context, loginAccount string) (platformAccountID, passwordHash, displayName string, ok bool, err error) {
	e := s.pool.QueryRow(ctx, `SELECT platform_account_id, password_hash, display_name FROM platform_accounts WHERE login_account=$1`,
		loginAccount).Scan(&platformAccountID, &passwordHash, &displayName)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", "", "", false, nil
	}
	if e != nil {
		return "", "", "", false, e
	}
	return platformAccountID, passwordHash, displayName, true, nil
}

func (s *Store) IsDeviceTrusted(ctx context.Context, platformAccountID, deviceID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM device_trust WHERE platform_account_id=$1 AND device_id=$2)`,
		platformAccountID, deviceID).Scan(&exists)
	return exists, err
}

func (s *Store) TrustDevice(ctx context.Context, platformAccountID, deviceID string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO device_trust (platform_account_id, device_id) VALUES ($1,$2)
		ON CONFLICT DO NOTHING`, platformAccountID, deviceID)
	return err
}

// EnsureDevPasswordAccount dev 种子:预置一个带密码的测试账户(login=13900000000),供密码登录回归。
// 仅 dev 启动时调用;passwordHash 由调用方用 bcrypt 算好。
func (s *Store) EnsureDevPasswordAccount(ctx context.Context, loginAccount, displayName, passwordHash string) error {
	id := newID("pa_")
	_, err := s.pool.Exec(ctx, `INSERT INTO platform_accounts (platform_account_id, login_account, display_name, password_hash, real_name_verified, adult)
		VALUES ($1,$2,$3,$4,true,true)
		ON CONFLICT (login_account) DO UPDATE SET password_hash=$4`,
		id, loginAccount, displayName, passwordHash)
	return err
}

// ConsumeSmsCodeForDevice 校验并消费验证码(设备验证复用 sms 链路)。
func (s *Store) ConsumeDeviceCode(ctx context.Context, gameID, loginAccount, code string) (SmsConsume, error) {
	return s.ConsumeSmsCode(ctx, gameID, loginAccount, code)
}

// ---------- M4-S3 生产加固 ----------

// UpsertSigningKey 注入(生产)验签密钥。
func (s *Store) UpsertSigningKey(ctx context.Context, keyID, secret string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO signing_keys (key_id, secret, active) VALUES ($1,$2,true)
		ON CONFLICT (key_id) DO UPDATE SET secret=$2, active=true`, keyID, secret)
	return err
}

// DeactivateSigningKey 停用密钥(生产启动时停用 dev 公开测试密钥)。
func (s *Store) DeactivateSigningKey(ctx context.Context, keyID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE signing_keys SET active=false WHERE key_id=$1`, keyID)
	return err
}

// DeactivateOtherSigningKeys 停用除当前生产 keyId 外的所有密钥;
// 杜绝占位/历史 key(如 dev-test-key、prod-key-placeholder)残留 active 形成后门。
func (s *Store) DeactivateOtherSigningKeys(ctx context.Context, keepKeyID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE signing_keys SET active=false WHERE key_id<>$1`, keepKeyID)
	return err
}
