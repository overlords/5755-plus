package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---------- 账户会话校验与踢号(#11) ----------

type SessionOwner struct {
	PlatformAccountID string
	DisplayName       string
}

// ValidateAccountSession 按(令牌, 账户, 游戏)三元组校验主账户会话。
// 返回 (owner, valid, err):valid=false 表示明确失效(不存在/吊销/过期/三元组不匹配)。
func (s *Store) ValidateAccountSession(ctx context.Context, platformToken, platformAccountID, gameID string) (*SessionOwner, bool, error) {
	var owner SessionOwner
	var revoked bool
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT a.platform_account_id, a.display_name, s.revoked, s.expires_at
		FROM account_sessions s JOIN platform_accounts a ON a.platform_account_id = s.platform_account_id
		WHERE s.platform_token=$1 AND s.platform_account_id=$2 AND s.game_id=$3`,
		platformToken, platformAccountID, gameID).Scan(&owner.PlatformAccountID, &owner.DisplayName, &revoked, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if revoked || time.Now().After(expiresAt) {
		return nil, false, nil
	}
	return &owner, true, nil
}

// KickAccount 吊销账户在某游戏下全部主账户会话与小号会话(踢号,不可逆)。
func (s *Store) KickAccount(ctx context.Context, gameID, platformAccountID string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE account_sessions SET revoked=true
		WHERE game_id=$1 AND platform_account_id=$2 AND NOT revoked`, gameID, platformAccountID)
	if err != nil {
		return 0, err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE subaccount_sessions SET revoked=true
		WHERE game_id=$1 AND platform_account_id=$2 AND NOT revoked`, gameID, platformAccountID); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ---------- 实名(#12) ----------

type RealNameState struct {
	Verified bool
	Adult    bool
}

func (s *Store) GetRealName(ctx context.Context, platformAccountID string) (*RealNameState, error) {
	var st RealNameState
	err := s.pool.QueryRow(ctx, `SELECT real_name_verified, adult FROM platform_accounts
		WHERE platform_account_id=$1`, platformAccountID).Scan(&st.Verified, &st.Adult)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// SubmitRealName 落库脱敏实名;已实名时幂等成功不改值(返回当前状态)。
func (s *Store) SubmitRealName(ctx context.Context, platformAccountID, nameMasked, idMasked string, adult bool) (*RealNameState, error) {
	cur, err := s.GetRealName(ctx, platformAccountID)
	if err != nil {
		return nil, err
	}
	if cur.Verified {
		return cur, nil // 已实名锁定
	}
	_, err = s.pool.Exec(ctx, `UPDATE platform_accounts
		SET real_name_verified=true, adult=$2, real_name_masked=$3, id_number_masked=$4
		WHERE platform_account_id=$1`, platformAccountID, adult, nameMasked, idMasked)
	if err != nil {
		return nil, err
	}
	return &RealNameState{Verified: true, Adult: adult}, nil
}

// ---------- 账户级 dev 注入(#12) ----------

type AccountInjection struct {
	EntryBlocked   *bool `json:"entryBlocked,omitempty"`
	PaymentBlocked *bool `json:"paymentBlocked,omitempty"`
}

func (s *Store) SetAccountInjection(ctx context.Context, gameID, platformAccountID string, entry, payment *bool) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO dev_account_injections (game_id, platform_account_id, entry_blocked, payment_blocked, updated_at)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (game_id, platform_account_id) DO UPDATE SET entry_blocked=$3, payment_blocked=$4, updated_at=now()`,
		gameID, platformAccountID, entry, payment)
	return err
}

func (s *Store) GetAccountInjection(ctx context.Context, gameID, platformAccountID string) (*AccountInjection, error) {
	inj := &AccountInjection{}
	err := s.pool.QueryRow(ctx, `SELECT entry_blocked, payment_blocked FROM dev_account_injections
		WHERE game_id=$1 AND platform_account_id=$2`, gameID, platformAccountID).Scan(&inj.EntryBlocked, &inj.PaymentBlocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return inj, nil
	}
	if err != nil {
		return nil, err
	}
	return inj, nil
}

// ClearGameInjections 清除游戏作用域内全部注入(维护 + 账户级)。
func (s *Store) ClearGameInjections(ctx context.Context, gameID string) error {
	if err := s.ClearInjections(ctx, gameID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM dev_account_injections WHERE game_id=$1`, gameID)
	return err
}

// ---------- 小号(#13) ----------

// CountSubaccounts 当前账户当前游戏下有效小号数。
func (s *Store) CountSubaccounts(ctx context.Context, platformAccountID, gameID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM subaccounts
		WHERE platform_account_id=$1 AND game_id=$2 AND active`, platformAccountID, gameID).Scan(&n)
	return n, err
}

// CreateSubaccount 平台命名(小号N,N=历史总数+1 保证不重名)创建非默认小号。
func (s *Store) CreateSubaccount(ctx context.Context, platformAccountID, gameID string) (*Subaccount, error) {
	var maxSeq int
	if err := s.pool.QueryRow(ctx, `SELECT coalesce(max(seq),0) FROM subaccounts
		WHERE platform_account_id=$1 AND game_id=$2`, platformAccountID, gameID).Scan(&maxSeq); err != nil {
		return nil, err
	}
	seq := maxSeq + 1
	acc := newID("sub_")
	display := fmt.Sprintf("小号%d", seq)
	_, err := s.pool.Exec(ctx, `INSERT INTO subaccounts (account, platform_account_id, game_id, display_name, seq, is_default)
		VALUES ($1,$2,$3,$4,$5,false)`, acc, platformAccountID, gameID, display, seq)
	if err != nil {
		return nil, err
	}
	return &Subaccount{Account: acc, GameID: gameID, DisplayName: display, IsDefault: false}, nil
}

// SetDefaultSubaccount 设默认:互斥 + 幂等。目标不存在/失效返回 ErrNotFound。
func (s *Store) SetDefaultSubaccount(ctx context.Context, platformAccountID, gameID, account string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM subaccounts
		WHERE account=$1 AND platform_account_id=$2 AND game_id=$3 AND active)`,
		account, platformAccountID, gameID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE subaccounts SET is_default=false
		WHERE platform_account_id=$1 AND game_id=$2 AND is_default AND account<>$3`,
		platformAccountID, gameID, account); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE subaccounts SET is_default=true WHERE account=$1`, account); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// InvalidateSubaccount 停用小号并吊销其全部小号令牌(dev 注入)。
func (s *Store) InvalidateSubaccount(ctx context.Context, gameID, account string) error {
	if _, err := s.pool.Exec(ctx, `UPDATE subaccounts SET active=false WHERE account=$1 AND game_id=$2`, account, gameID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE subaccount_sessions SET revoked=true WHERE account=$1 AND game_id=$2 AND NOT revoked`, account, gameID)
	return err
}

// ---------- 小号会话(#14) ----------

// GetSubaccount 取小号(含归属与有效性信息)。
func (s *Store) GetSubaccount(ctx context.Context, account string) (*Subaccount, string, bool, error) {
	var sub Subaccount
	var owner string
	var active bool
	err := s.pool.QueryRow(ctx, `SELECT account, game_id, display_name, is_default, platform_account_id, active
		FROM subaccounts WHERE account=$1`, account).Scan(&sub.Account, &sub.GameID, &sub.DisplayName, &sub.IsDefault, &owner, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", false, ErrNotFound
	}
	if err != nil {
		return nil, "", false, err
	}
	return &sub, owner, active, nil
}

// CreateSubaccountSession 签发小号登录令牌。
func (s *Store) CreateSubaccountSession(ctx context.Context, account, platformAccountID, gameID string, ttl time.Duration) (string, time.Time, error) {
	token := newID("st_")
	exp := time.Now().Add(ttl)
	_, err := s.pool.Exec(ctx, `INSERT INTO subaccount_sessions (token, account, platform_account_id, game_id, expires_at)
		VALUES ($1,$2,$3,$4,$5)`, token, account, platformAccountID, gameID, exp)
	return token, exp, err
}

// ValidateSubaccountSession 校验小号登录态:返回 (account, valid)。
// 令牌存在但小号已停用 → valid=false(subaccount_invalid 由 domain 判定)。
func (s *Store) ValidateSubaccountSession(ctx context.Context, token, account, gameID string) (bool, error) {
	var revoked bool
	var expiresAt time.Time
	var active bool
	err := s.pool.QueryRow(ctx, `SELECT ss.revoked, ss.expires_at, sa.active
		FROM subaccount_sessions ss JOIN subaccounts sa ON sa.account = ss.account
		WHERE ss.token=$1 AND ss.account=$2 AND ss.game_id=$3`, token, account, gameID).Scan(&revoked, &expiresAt, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !revoked && active && time.Now().Before(expiresAt), nil
}
