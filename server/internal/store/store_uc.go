package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
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
