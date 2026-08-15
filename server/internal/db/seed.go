// Package db 提供 PostgreSQL 连接池管理、迁移执行与演示数据注入。
package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HashPassword 对密码做 SHA-256 摘要（第一期演示用，生产环境应改用 bcrypt/argon2）。
func HashPassword(pw string) string {
	sum := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(sum[:])
}

// NewToken 生成随机会话令牌（第一期演示用，生产环境应改用 JWT）。
func NewToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "demo-token"
	}
	return hex.EncodeToString(b)
}

// Seed 在用户表为空时注入演示数据（幂等：已有数据则跳过）。
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	var cnt int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		log.Printf("检测到已有用户数据，跳过演示数据注入")
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. 区域（集团总部）
	var regionID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO region (name, level) VALUES ('集团总部', 1) RETURNING id`,
	).Scan(&regionID); err != nil {
		return err
	}

	// 2. 门店
	var storeID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO store (region_id, name, address, phone, manager)
		 VALUES ($1, '华庭酒店·总店', '北京市朝阳区建国路88号', '010-88886666', '张店长') RETURNING id`,
		regionID,
	).Scan(&storeID); err != nil {
		return err
	}

	// 3. 角色
	var adminRoleID, frontRoleID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO roles (name, level) VALUES ('集团管理员', 9) RETURNING id`,
	).Scan(&adminRoleID); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO roles (name, level) VALUES ('前台', 3) RETURNING id`,
	).Scan(&frontRoleID); err != nil {
		return err
	}

	// 4. 用户
	if _, err := tx.Exec(ctx,
		`INSERT INTO users (username, password_hash, name, role_id) VALUES ('admin', $1, '系统管理员', $2)`,
		HashPassword("admin123"), adminRoleID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO users (username, password_hash, name, role_id) VALUES ('front', $1, '前台小王', $2)`,
		HashPassword("123456"), frontRoleID,
	); err != nil {
		return err
	}

	// 5. 房型
	roomTypeDefs := []struct {
		name     string
		bed      string
		capacity int
	}{
		{"标准间", "双床", 2},
		{"大床房", "大床", 2},
		{"豪华套房", "大床", 3},
	}
	roomTypeIDs := make([]int64, 0, len(roomTypeDefs))
	for _, t := range roomTypeDefs {
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO room_type (store_id, name, bed_type, capacity) VALUES ($1, $2, $3, $4) RETURNING id`,
			storeID, t.name, t.bed, t.capacity,
		).Scan(&id); err != nil {
			return err
		}
		roomTypeIDs = append(roomTypeIDs, id)
	}

	// 6. 房间（房号/楼层/房型/状态：0空净 1空脏 2住客 3维修 4预留）
	roomDefs := []struct {
		no     string
		floor  string
		rtIdx  int
		status int
	}{
		{"101", "1", 0, 0}, {"102", "1", 0, 1}, {"103", "1", 0, 2},
		{"104", "1", 0, 0}, {"105", "1", 0, 4},
		{"201", "2", 1, 0}, {"202", "2", 1, 2}, {"203", "2", 1, 1},
		{"204", "2", 1, 3},
		{"301", "3", 2, 0}, {"302", "3", 2, 2},
	}
	for _, r := range roomDefs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO room (store_id, room_type_id, room_no, floor, status) VALUES ($1, $2, $3, $4, $5)`,
			storeID, roomTypeIDs[r.rtIdx], r.no, r.floor, r.status,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	log.Printf("演示数据注入完成：门店/房型/房间/用户已就绪（admin/admin123）")
	return nil
}
