package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSessionInvalid 会话不存在或已过期。
var ErrSessionInvalid = errors.New("会话无效或已过期")

// CreateSession 为用户创建登录会话，返回随机令牌。
// ttl 为会话有效期，过期后鉴权中间件将拒绝。
func CreateSession(ctx context.Context, p *pgxpool.Pool, userID int64, ttl time.Duration) (string, error) {
	token := NewToken()
	_, err := p.Exec(ctx,
		`INSERT INTO session (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		token, userID, time.Now().Add(ttl),
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

// GetSessionUser 校验令牌并返回其用户 ID。
// 令牌不存在或已过期时返回 ErrSessionInvalid。
func GetSessionUser(ctx context.Context, p *pgxpool.Pool, token string) (int64, error) {
	var uid int64
	err := p.QueryRow(ctx,
		`SELECT user_id FROM session WHERE token = $1 AND expires_at > now()`,
		token,
	).Scan(&uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrSessionInvalid
		}
		return 0, err
	}
	return uid, nil
}

// DeleteSession 删除会话（登出）。
func DeleteSession(ctx context.Context, p *pgxpool.Pool, token string) error {
	_, err := p.Exec(ctx, `DELETE FROM session WHERE token = $1`, token)
	return err
}
