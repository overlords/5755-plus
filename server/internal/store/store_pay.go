package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------- #60 入站支付:收银台所选渠道 + 回调幂等 ----------

// SetOrderPaymentMethod 落库收银台所选支付方式(wechat/alipay)。方式无关订单的"用什么付",
// 仅供对账/诊断;不改 04 契约。仅在订单仍为待支付时更新(已支付订单不覆盖)。
func (s *Store) SetOrderPaymentMethod(ctx context.Context, orderID, method string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE orders SET payment_method=$2 WHERE order_id=$1 AND payment_status=$3`,
		orderID, method, PaymentPending)
	return err
}

// ErrNotifyAlreadyProcessed 表示该渠道+订单的回调已被先前的 notify 处理过(幂等命中)。
var ErrNotifyAlreadyProcessed = errors.New("payment notify already processed")

// ClaimPaymentNotification 原子认领一笔渠道回调:首条插入成功返回 nil(可继续发放);
// 重复回调因主键冲突返回 ErrNotifyAlreadyProcessed(直接回 ACK,不重复触发)。
func (s *Store) ClaimPaymentNotification(ctx context.Context, channel, orderID, channelTxnID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO payment_notifications (channel, order_id, channel_txn_id)
		 VALUES ($1,$2,$3)`,
		channel, orderID, channelTxnID)
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return ErrNotifyAlreadyProcessed
	}
	return err
}

// ReleasePaymentNotification 回滚认领(后续发放编排失败时,允许渠道重推再试)。
func (s *Store) ReleasePaymentNotification(ctx context.Context, channel, orderID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM payment_notifications WHERE channel=$1 AND order_id=$2`,
		channel, orderID)
	return err
}

// PaymentNotificationProcessed 查询某渠道+订单是否已处理(测试/诊断用)。
func (s *Store) PaymentNotificationProcessed(ctx context.Context, channel, orderID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM payment_notifications WHERE channel=$1 AND order_id=$2)`,
		channel, orderID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return exists, err
}
