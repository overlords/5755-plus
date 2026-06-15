package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------- 用户中心面(/api/uc/v2,ADR-0010) ----------
//
// 与 SDK 网关面不同:用户中心 SPA 只持有 platformToken,不带 platformAccountId/gameId,
// 故会话仅凭 token 反解(account_sessions.platform_token 为主键)。

// UCSession 是仅凭 token 反解出的主账户会话归属。
type UCSession struct {
	PlatformAccountID string
	GameID            string
}

// LookupAccountSessionByToken 仅凭 platformToken 反解会话。
// 返回 (session, valid, err):valid=false 表示不存在/吊销/过期。
func (s *Store) LookupAccountSessionByToken(ctx context.Context, platformToken string) (*UCSession, bool, error) {
	var sess UCSession
	var revoked bool
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT platform_account_id, game_id, revoked, expires_at
		FROM account_sessions WHERE platform_token=$1`,
		platformToken).Scan(&sess.PlatformAccountID, &sess.GameID, &revoked, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if revoked || time.Now().After(expiresAt) {
		return nil, false, nil
	}
	return &sess, true, nil
}

// UCAccount 是用户中心展示所需的主账户字段。
type UCAccount struct {
	DisplayName      string
	LoginAccount     string
	RealNameVerified bool
}

// GetPlatformAccount 读主账户展示字段。account 不存在返回 pgx.ErrNoRows。
func (s *Store) GetPlatformAccount(ctx context.Context, platformAccountID string) (*UCAccount, error) {
	var a UCAccount
	err := s.pool.QueryRow(ctx, `SELECT display_name, login_account, real_name_verified
		FROM platform_accounts WHERE platform_account_id=$1`,
		platformAccountID).Scan(&a.DisplayName, &a.LoginAccount, &a.RealNameVerified)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// UCSubaccount 是用户中心展示的当前游戏小号。
type UCSubaccount struct {
	Account string
	Label   string
}

// CurrentSubaccount 取该账户在该游戏下的当前小号:优先默认小号,否则最早创建的有效小号。
// 返回 (sub, ok, err):ok=false 表示该游戏下尚无有效小号。
func (s *Store) CurrentSubaccount(ctx context.Context, platformAccountID, gameID string) (*UCSubaccount, bool, error) {
	var sub UCSubaccount
	err := s.pool.QueryRow(ctx, `SELECT account, display_name
		FROM subaccounts WHERE platform_account_id=$1 AND game_id=$2 AND active
		ORDER BY is_default DESC, seq ASC LIMIT 1`,
		platformAccountID, gameID).Scan(&sub.Account, &sub.Label)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &sub, true, nil
}

// UCOrder 是用户中心充值订单列表行(06a §3:orderId/productName/amount/createdAt/status)。
type UCOrder struct {
	OrderID     string
	ProductName string
	Amount      string // numeric → 字符串保留精度
	CreatedAt   time.Time
	Status      string
}

// ListOrders 返回主账户充值订单(keyset 游标分页,按 created_at DESC, order_id DESC)。
// cursor 为上一页末行的 order_id;空串取首页。返回 (orders, nextCursor, err);nextCursor 空表示无更多。
func (s *Store) ListOrders(ctx context.Context, platformAccountID, cursor string, limit int) ([]UCOrder, string, error) {
	rows, err := s.pool.Query(ctx, `SELECT order_id, commodity, amount::text, payment_status, created_at
		FROM orders
		WHERE platform_account_id=$1
		  AND ($2='' OR (created_at, order_id) <
		       (SELECT created_at, order_id FROM orders WHERE order_id=$2))
		ORDER BY created_at DESC, order_id DESC
		LIMIT $3`, platformAccountID, cursor, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []UCOrder
	for rows.Next() {
		var o UCOrder
		if err := rows.Scan(&o.OrderID, &o.ProductName, &o.Amount, &o.Status, &o.CreatedAt); err != nil {
			return nil, "", err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > limit {
		next = out[limit-1].OrderID
		out = out[:limit]
	}
	return out, next, nil
}

// UpdateLoginAccount 把主账户绑定手机改为 newPhone(换绑)。
// 返回 (ok, err):ok=false 表示该手机号已被占用(login_account 唯一约束 23505)。
func (s *Store) UpdateLoginAccount(ctx context.Context, platformAccountID, newPhone string) (bool, error) {
	_, err := s.pool.Exec(ctx, `UPDATE platform_accounts SET login_account=$1 WHERE platform_account_id=$2`,
		newPhone, platformAccountID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UpdatePasswordHash 改主账户密码哈希(改密)。
func (s *Store) UpdatePasswordHash(ctx context.Context, platformAccountID, passwordHash string) error {
	_, err := s.pool.Exec(ctx, `UPDATE platform_accounts SET password_hash=$1 WHERE platform_account_id=$2`,
		passwordHash, platformAccountID)
	return err
}

// RevokeAllAccountSessions 吊销主账户**跨全部游戏**的会话(改密强制处处重登)。
func (s *Store) RevokeAllAccountSessions(ctx context.Context, platformAccountID string) error {
	if _, err := s.pool.Exec(ctx, `UPDATE account_sessions SET revoked=true
		WHERE platform_account_id=$1 AND NOT revoked`, platformAccountID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE subaccount_sessions SET revoked=true
		WHERE platform_account_id=$1 AND NOT revoked`, platformAccountID)
	return err
}
