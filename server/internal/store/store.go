// Package store 是平台服务端的持久化边界:pgx 连接池 + 嵌入式迁移 + 手写 SQL。
package store

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrNotFound 表示按键查询无记录。
var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("连接 Postgres 失败: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("Ping Postgres 失败: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Migrate 套用 migrations/*.sql,按文件名字典序;用 schema_migrations 跟踪,幂等可重跑。
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("建 schema_migrations 失败: %w", err)
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("套用迁移 %s 失败: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func newID(prefix string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// ---------- 游戏配置 ----------

type GameConfig struct {
	GameID                      string
	GameName                    string
	MaintenanceEnabled          bool
	MaintenanceMessage          string
	AntiAddictionEntryBlocked   bool
	AntiAddictionPaymentBlocked bool
	ProtocolVersion             string
	ConfigVersion               string
	SDKLatestVersion            string
	SDKMinVersion               string
	LoginDomain                 string
	PaymentDomain               string
	UserCenterURL               string
}

// GetGameConfig 读游戏配置,并叠加 dev 控制面维护注入(若有)。游戏不存在返回 ErrNotFound。
func (s *Store) GetGameConfig(ctx context.Context, gameID string) (*GameConfig, error) {
	var g GameConfig
	err := s.pool.QueryRow(ctx, `SELECT game_id, game_name, maintenance_enabled, maintenance_message,
		anti_addiction_entry_blocked, anti_addiction_payment_blocked, protocol_version, config_version,
		sdk_latest_version, sdk_min_version, login_domain, payment_domain, user_center_url
		FROM games WHERE game_id=$1`, gameID).Scan(
		&g.GameID, &g.GameName, &g.MaintenanceEnabled, &g.MaintenanceMessage,
		&g.AntiAddictionEntryBlocked, &g.AntiAddictionPaymentBlocked, &g.ProtocolVersion, &g.ConfigVersion,
		&g.SDKLatestVersion, &g.SDKMinVersion, &g.LoginDomain, &g.PaymentDomain, &g.UserCenterURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// 叠加维护注入覆盖
	var injEnabled *bool
	var injMsg *string
	err = s.pool.QueryRow(ctx, `SELECT maintenance_enabled, maintenance_message FROM dev_injections WHERE game_id=$1`, gameID).Scan(&injEnabled, &injMsg)
	if err == nil {
		if injEnabled != nil {
			g.MaintenanceEnabled = *injEnabled
		}
		if injMsg != nil {
			g.MaintenanceMessage = *injMsg
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return &g, nil
}

// ---------- 验签密钥 ----------

func (s *Store) LookupSigningKey(ctx context.Context, keyID string) (string, bool, error) {
	var secret string
	err := s.pool.QueryRow(ctx, `SELECT secret FROM signing_keys WHERE key_id=$1 AND active`, keyID).Scan(&secret)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return secret, true, nil
}

// ---------- 短信验证码 ----------

type SmsCode struct {
	CodeID    string
	Code      string
	ExpiresAt time.Time
}

// CreateSmsCode 为(game,手机号)签发一次验证码并入库。
func (s *Store) CreateSmsCode(ctx context.Context, gameID, loginAccount, code string, ttl time.Duration) (*SmsCode, error) {
	id := newID("sms_")
	exp := time.Now().Add(ttl)
	_, err := s.pool.Exec(ctx, `INSERT INTO sms_codes (code_id, game_id, login_account, code, provider_mode, expires_at)
		VALUES ($1,$2,$3,$4,'mock',$5)`, id, gameID, loginAccount, code, exp)
	if err != nil {
		return nil, err
	}
	return &SmsCode{CodeID: id, Code: code, ExpiresAt: exp}, nil
}

// CountRecentSmsCodes 统计某号近窗口内的请求数,用于限流。
func (s *Store) CountRecentSmsCodes(ctx context.Context, gameID, loginAccount string, within time.Duration) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM sms_codes WHERE game_id=$1 AND login_account=$2 AND created_at > $3`,
		gameID, loginAccount, time.Now().Add(-within)).Scan(&n)
	return n, err
}

// smsConsumeResult 区分"未找到/已用"、"过期"、"有效"。
type SmsConsume int

const (
	SmsConsumeOK SmsConsume = iota
	SmsConsumeInvalid
	SmsConsumeExpired
)

// ConsumeSmsCode 校验并消费最近一条匹配验证码:返回消费结果。
func (s *Store) ConsumeSmsCode(ctx context.Context, gameID, loginAccount, code string) (SmsConsume, error) {
	var codeID string
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT code_id, expires_at FROM sms_codes
		WHERE game_id=$1 AND login_account=$2 AND code=$3 AND NOT consumed
		ORDER BY created_at DESC LIMIT 1`, gameID, loginAccount, code).Scan(&codeID, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SmsConsumeInvalid, nil
	}
	if err != nil {
		return SmsConsumeInvalid, err
	}
	if time.Now().After(expiresAt) {
		return SmsConsumeExpired, nil
	}
	if _, err := s.pool.Exec(ctx, `UPDATE sms_codes SET consumed=true WHERE code_id=$1`, codeID); err != nil {
		return SmsConsumeInvalid, err
	}
	return SmsConsumeOK, nil
}

// ---------- 账户 / 会话 / 小号 ----------

type Account struct {
	PlatformAccountID string
	LoginAccount      string
	DisplayName       string
	isNew             bool
}

type Subaccount struct {
	Account     string
	GameID      string
	DisplayName string
	IsDefault   bool
}

// FindOrCreateAccount 按手机号取或建 5755 账户;第二返回值表示是否新建。
func (s *Store) FindOrCreateAccount(ctx context.Context, loginAccount, channelID, channelSource string) (*Account, bool, error) {
	var a Account
	err := s.pool.QueryRow(ctx, `SELECT platform_account_id, login_account, display_name
		FROM platform_accounts WHERE login_account=$1`, loginAccount).Scan(&a.PlatformAccountID, &a.LoginAccount, &a.DisplayName)
	if err == nil {
		return &a, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	id := newID("pa_")
	display := "玩家" + id[len(id)-6:]
	_, err = s.pool.Exec(ctx, `INSERT INTO platform_accounts (platform_account_id, login_account, display_name, channel_id, channel_source)
		VALUES ($1,$2,$3,$4,$5)`, id, loginAccount, display, channelID, channelSource)
	if err != nil {
		return nil, false, err
	}
	return &Account{PlatformAccountID: id, LoginAccount: loginAccount, DisplayName: display}, true, nil
}

// CreateSession 为账户在某游戏下签发主账户会话令牌。
func (s *Store) CreateSession(ctx context.Context, platformAccountID, gameID string, ttl time.Duration) (string, time.Time, error) {
	token := newID("pt_")
	exp := time.Now().Add(ttl)
	_, err := s.pool.Exec(ctx, `INSERT INTO account_sessions (platform_token, platform_account_id, game_id, expires_at)
		VALUES ($1,$2,$3,$4)`, token, platformAccountID, gameID, exp)
	return token, exp, err
}

// EnsureFirstSubaccount 保障某账户在某游戏下至少有一个真实小号;返回该游戏下当前小号列表。
func (s *Store) EnsureFirstSubaccount(ctx context.Context, platformAccountID, gameID string) ([]Subaccount, *Subaccount, error) {
	subs, err := s.ListSubaccounts(ctx, platformAccountID, gameID)
	if err != nil {
		return nil, nil, err
	}
	var created *Subaccount
	if len(subs) == 0 {
		acc := newID("sub_")
		display := "小号1"
		_, err := s.pool.Exec(ctx, `INSERT INTO subaccounts (account, platform_account_id, game_id, display_name, seq, is_default)
			VALUES ($1,$2,$3,$4,1,false)`, acc, platformAccountID, gameID, display)
		if err != nil {
			return nil, nil, err
		}
		created = &Subaccount{Account: acc, GameID: gameID, DisplayName: display, IsDefault: false}
		subs = append(subs, *created)
	}
	return subs, created, nil
}

func (s *Store) ListSubaccounts(ctx context.Context, platformAccountID, gameID string) ([]Subaccount, error) {
	rows, err := s.pool.Query(ctx, `SELECT account, game_id, display_name, is_default
		FROM subaccounts WHERE platform_account_id=$1 AND game_id=$2 AND active ORDER BY seq ASC`, platformAccountID, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Subaccount
	for rows.Next() {
		var sub Subaccount
		if err := rows.Scan(&sub.Account, &sub.GameID, &sub.DisplayName, &sub.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// ---------- dev 控制面注入态 ----------

func (s *Store) SetMaintenanceInjection(ctx context.Context, gameID string, enabled bool, message string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO dev_injections (game_id, maintenance_enabled, maintenance_message, updated_at)
		VALUES ($1,$2,$3,now())
		ON CONFLICT (game_id) DO UPDATE SET maintenance_enabled=$2, maintenance_message=$3, updated_at=now()`,
		gameID, enabled, message)
	return err
}

type InjectionState struct {
	GameID             string  `json:"gameId"`
	MaintenanceEnabled *bool   `json:"maintenanceEnabled,omitempty"`
	MaintenanceMessage *string `json:"maintenanceMessage,omitempty"`
}

func (s *Store) GetInjectionState(ctx context.Context, gameID string) (*InjectionState, error) {
	st := &InjectionState{GameID: gameID}
	err := s.pool.QueryRow(ctx, `SELECT maintenance_enabled, maintenance_message FROM dev_injections WHERE game_id=$1`, gameID).
		Scan(&st.MaintenanceEnabled, &st.MaintenanceMessage)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) ClearInjections(ctx context.Context, gameID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM dev_injections WHERE game_id=$1`, gameID)
	return err
}
