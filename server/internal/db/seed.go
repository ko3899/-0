// Package db 提供 PostgreSQL 连接池管理、迁移执行与演示数据注入。
package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// roomTypeSeed 房型种子定义（价格=门市价，用于房价日历与在住账单）。
type roomTypeSeed struct {
	name     string
	bed      string
	capacity int
	price    float64
}

// Seed 在用户表为空时注入演示数据（幂等：已有数据则跳过）。
// 演示数据：3 门店、每店 4 房型 32 间房、房价日历、客户/会员、真实在住账单。
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

	// 2. 门店 × 3
	storeDefs := []struct{ name, addr, phone, manager string }{
		{"华庭酒店·总店", "北京市朝阳区建国路88号", "010-88886666", "张店长"},
		{"华庭酒店·朝阳店", "北京市朝阳区望京街10号", "010-66668888", "李店长"},
		{"华庭酒店·海淀店", "北京市海淀区中关村大街1号", "010-55557777", "王店长"},
	}
	storeIDs := make([]int64, 0, len(storeDefs))
	for _, s := range storeDefs {
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO store (region_id, name, address, phone, manager)
			 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			regionID, s.name, s.addr, s.phone, s.manager,
		).Scan(&id); err != nil {
			return err
		}
		storeIDs = append(storeIDs, id)
	}

	// 3. 角色（集团管理员/店长/前台）
	var adminRoleID, managerRoleID, frontRoleID int64
	if err := tx.QueryRow(ctx, `INSERT INTO roles (name, level) VALUES ('集团管理员', 9) RETURNING id`).Scan(&adminRoleID); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO roles (name, level) VALUES ('店长', 5) RETURNING id`).Scan(&managerRoleID); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO roles (name, level) VALUES ('前台', 3) RETURNING id`).Scan(&frontRoleID); err != nil {
		return err
	}

	// 4. 用户 + 用户-门店关联
	userDefs := []struct {
		username, name, pw string
		roleID             int64
		storeIdx           int // -1 表示全部门店（集团）
	}{
		{"admin", "系统管理员", "admin123", adminRoleID, -1},
		{"manager1", "张店长", "123456", managerRoleID, 0},
		{"front1", "前台小王", "123456", frontRoleID, 0},
		{"front2", "前台小李", "123456", frontRoleID, 1},
	}
	for _, u := range userDefs {
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO users (username, password_hash, name, role_id) VALUES ($1, $2, $3, $4) RETURNING id`,
			u.username, HashPassword(u.pw), u.name, u.roleID,
		).Scan(&id); err != nil {
			return err
		}
		// 集团管理员关联全部门店；店长/前台关联指定门店
		stores := storeIDs
		if u.storeIdx >= 0 {
			stores = []int64{storeIDs[u.storeIdx]}
		}
		for _, sid := range stores {
			if _, err := tx.Exec(ctx,
				`INSERT INTO user_store (user_id, store_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				id, sid,
			); err != nil {
				return err
			}
		}
	}

	// 5. 房型（每店 4 个）
	roomTypeDefs := []roomTypeSeed{
		{"标准间", "双床", 2, 298},
		{"大床房", "大床", 2, 358},
		{"豪华套房", "大床", 3, 588},
		{"家庭房", "双床", 4, 458},
	}
	// roomTypeIDs[storeIdx][typeIdx]
	roomTypeIDs := make([][]int64, len(storeIDs))
	for i := range storeIDs {
		roomTypeIDs[i] = make([]int64, len(roomTypeDefs))
		for j, t := range roomTypeDefs {
			var id int64
			if err := tx.QueryRow(ctx,
				`INSERT INTO room_type (store_id, name, bed_type, capacity) VALUES ($1, $2, $3, $4) RETURNING id`,
				storeIDs[i], t.name, t.bed, t.capacity,
			).Scan(&id); err != nil {
				return err
			}
			roomTypeIDs[i][j] = id
		}
	}

	// 6. 房间（每店 4 层 × 8 间 = 32 间；房号=楼层+房号，如 101~408）
	// 状态：0空净 1空脏 2住客 3维修 4预留
	type roomSeed struct {
		no     string
		floor  string
		rtIdx  int
		status int
	}
	var roomSeeds []roomSeed
	for floor := 1; floor <= 4; floor++ {
		rtIdx := floor - 1 // 1F标准间 2F大床房 3F豪华套房 4F家庭房
		for roomNo := 1; roomNo <= 8; roomNo++ {
			no := fmt.Sprintf("%d%02d", floor, roomNo)
			status := 0
			// 每店固定几间特殊状态，让房态图有层次
			switch no {
			case "103":
				status = 1 // 空脏
			case "108":
				status = 3 // 维修
			case "205":
				status = 4 // 预留
			case "105", "201", "304":
				status = 2 // 住客
			}
			roomSeeds = append(roomSeeds, roomSeed{no, fmt.Sprintf("%d", floor), rtIdx, status})
		}
	}
	// 记录每个门店的住客房间号（用于后续生成在住账单）
	occupiedRooms := make(map[int][]string) // storeIdx -> []room_no
	for _, si := range []int{0, 1, 2} {
		var occ []string
		for _, rs := range roomSeeds {
			if rs.status == 2 {
				occ = append(occ, rs.no)
			}
		}
		occupiedRooms[si] = occ
	}
	// roomIDs[storeIdx][roomNo] = roomID
	roomIDs := make([]map[string]int64, len(storeIDs))
	for i := range storeIDs {
		roomIDs[i] = make(map[string]int64)
		for _, rs := range roomSeeds {
			var id int64
			if err := tx.QueryRow(ctx,
				`INSERT INTO room (store_id, room_type_id, room_no, floor, status) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
				storeIDs[i], roomTypeIDs[i][rs.rtIdx], rs.no, rs.floor, rs.status,
			).Scan(&id); err != nil {
				return err
			}
			roomIDs[i][rs.no] = id
		}
	}

	// 7. 房价方案（每店：门市价 + 协议价）
	type planSeed struct {
		name string
		typ  string
		rate float64 // 相对门市价的折扣
	}
	planDefs := []planSeed{
		{"门市价", "rack", 1.0},
		{"协议价", "contract", 0.85},
	}
	planIDs := make([][]int64, len(storeIDs))
	for i := range storeIDs {
		planIDs[i] = make([]int64, len(planDefs))
		for j, p := range planDefs {
			var id int64
			if err := tx.QueryRow(ctx,
				`INSERT INTO rate_plan (store_id, name, type) VALUES ($1, $2, $3) RETURNING id`,
				storeIDs[i], p.name, p.typ,
			).Scan(&id); err != nil {
				return err
			}
			planIDs[i][j] = id
		}
	}

	// 8. 房价日历（未来 7 天 × 每房型 × 每方案）
	for i := range storeIDs {
		for j, t := range roomTypeDefs {
			for k, p := range planDefs {
				price := t.price * p.rate
				for d := 0; d < 7; d++ {
					if _, err := tx.Exec(ctx,
						`INSERT INTO rate_calendar (store_id, room_type_id, rate_plan_id, biz_date, price)
						 VALUES ($1, $2, $3, CURRENT_DATE + $4::int, $5)`,
						storeIDs[i], roomTypeIDs[i][j], planIDs[i][k], d, price,
					); err != nil {
						return err
					}
				}
			}
		}
	}

	// 9. 客户 × 6（集团级）
	customerDefs := []struct{ name, phone, idNo string }{
		{"王建国", "13800138001", "110101198001011234"},
		{"李秀兰", "13800138002", "110101198502022345"},
		{"陈志强", "13800138003", "110101199003033456"},
		{"赵雅琴", "13800138004", "110101199204044567"},
		{"刘伟民", "13800138005", "110101198806055678"},
		{"孙丽华", "13800138006", "110101199508066789"},
	}
	customerIDs := make([]int64, 0, len(customerDefs))
	for _, c := range customerDefs {
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO customer (name, phone, id_no, id_type, gender) VALUES ($1, $2, $3, 'id_card', 1) RETURNING id`,
			c.name, c.phone, c.idNo,
		).Scan(&id); err != nil {
			return err
		}
		customerIDs = append(customerIDs, id)
	}

	// 10. 会员 × 3（关联前 3 个客户）
	memberDefs := []struct{ custIdx, level, points int }{
		{0, 2, 1500},
		{1, 1, 600},
		{2, 0, 0},
	}
	for m, md := range memberDefs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO member (customer_id, member_no, level, points, balance, join_date)
			 VALUES ($1, $2, $3, $4, 0, CURRENT_DATE)`,
			customerIDs[md.custIdx], fmt.Sprintf("M%06d", 100001+m), md.level, md.points,
		); err != nil {
			return err
		}
	}

	// 11. 在住数据（为每个住客房间生成 check_in + folio + 房费明细）
	for i := range storeIDs {
		for ri, roomNo := range occupiedRooms[i] {
			// 房费取该房间对应房型的门市价
			var rtIdx int
			for _, rs := range roomSeeds {
				if rs.no == roomNo {
					rtIdx = rs.rtIdx
					break
				}
			}
			price := roomTypeDefs[rtIdx].price
			custID := customerIDs[(i*3+ri)%len(customerIDs)] // 轮转分配客户

			var checkInID int64
			if err := tx.QueryRow(ctx,
				`INSERT INTO check_in (store_id, customer_id, room_id, check_in_time, expected_checkout_time, status)
				 VALUES ($1, $2, $3, now() - interval '1 day', now() + interval '1 day', 0) RETURNING id`,
				storeIDs[i], custID, roomIDs[i][roomNo],
			).Scan(&checkInID); err != nil {
				return err
			}

			var folioID int64
			if err := tx.QueryRow(ctx,
				`INSERT INTO folio (check_in_id, total_amount, paid_amount, balance, status)
				 VALUES ($1, $2, 0, $2, 0) RETURNING id`,
				checkInID, price,
			).Scan(&folioID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO folio_item (folio_id, item_type, amount, remark) VALUES ($1, 'room_fee', $2, '房费 1 晚')`,
				folioID, price,
			); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	log.Printf("演示数据注入完成：3 门店 / 96 间房 / 房价日历 / 客户会员 / 在住账单已就绪（admin/admin123）")
	return nil
}
