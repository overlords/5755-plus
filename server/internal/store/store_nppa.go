package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// NppaCreds 单游戏的 NPPA 凭据(ADR-0007:per-game,接入者授权)。
type NppaCreds struct {
	AppID     string
	BizID     string
	SecretKey string
}

// GetGameNppaCreds 读取某游戏的 NPPA 凭据;游戏不存在返回 ErrNotFound,未配置则字段为空。
func (s *Store) GetGameNppaCreds(ctx context.Context, gameID string) (*NppaCreds, error) {
	var c NppaCreds
	err := s.pool.QueryRow(ctx, `SELECT nppa_app_id, nppa_biz_id, nppa_secret_key FROM games WHERE game_id=$1`, gameID).
		Scan(&c.AppID, &c.BizID, &c.SecretKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SetGameNppaCreds 配置某游戏的 NPPA 凭据(接入者授权后由运维/console 写入;测试亦用)。
func (s *Store) SetGameNppaCreds(ctx context.Context, gameID, appID, bizID, secretKey string) error {
	_, err := s.pool.Exec(ctx, `UPDATE games SET nppa_app_id=$2, nppa_biz_id=$3, nppa_secret_key=$4 WHERE game_id=$1`,
		gameID, appID, bizID, secretKey)
	return err
}

// StageRealName 提交时落库脱敏姓名/号 + 成年判定 + 查询用 ai(verified/pending 先不置)。
// 这样异步"认证中"经查询接口出结果时,无需再持有原始姓名/号即可定案。
func (s *Store) StageRealName(ctx context.Context, platformAccountID, ai, nameMasked, idMasked string, adult bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE platform_accounts
		SET real_name_masked=$2, id_number_masked=$3, adult=$4, real_name_ai=$5,
		    real_name_verified=false, real_name_pending=false
		WHERE platform_account_id=$1`, platformAccountID, nameMasked, idMasked, adult, ai)
	return err
}

// MarkRealNameVerified 认证成功:置 verified + pi,清 pending/ai(脱敏与 adult 已由 StageRealName 落库)。
func (s *Store) MarkRealNameVerified(ctx context.Context, platformAccountID, pi string) error {
	_, err := s.pool.Exec(ctx, `UPDATE platform_accounts
		SET real_name_verified=true, real_name_pending=false, real_name_ai='', real_name_pi=$2
		WHERE platform_account_id=$1`, platformAccountID, pi)
	return err
}

// MarkRealNamePending 认证中:置 pending(等查询接口出结果)。
func (s *Store) MarkRealNamePending(ctx context.Context, platformAccountID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE platform_accounts SET real_name_pending=true
		WHERE platform_account_id=$1`, platformAccountID)
	return err
}

// MarkRealNameFailed 认证失败:清 pending/ai(verified 保持 false)。
func (s *Store) MarkRealNameFailed(ctx context.Context, platformAccountID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE platform_accounts
		SET real_name_pending=false, real_name_ai='' WHERE platform_account_id=$1`, platformAccountID)
	return err
}
