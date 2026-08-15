-- ============================================================
-- 酒店管理系统（连锁多门店）第一期数据库初始化迁移
-- 对应设计文档「设计-数据库表结构」7 大域 20 张表
-- 说明：
--   * user/role 为 PostgreSQL 保留字，实现时表名用 users/roles
--   * 门店级业务表带 store_id 做数据隔离
--   * 客户/会员为集团级（跨店共享，不带 store_id）
-- ============================================================

-- ============================================================
-- 1. 组织架构
-- ============================================================

-- 区域（自关联多级，集团总部 → 区域 → 门店）
CREATE TABLE region (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(100) NOT NULL,
    parent_id  BIGINT REFERENCES region(id) ON DELETE SET NULL,
    level      SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE region IS '区域（自关联多级）';
COMMENT ON COLUMN region.level IS '层级：1集团 2区域 3...';

-- 门店
CREATE TABLE store (
    id         BIGSERIAL PRIMARY KEY,
    region_id  BIGINT REFERENCES region(id),
    name       VARCHAR(100) NOT NULL,
    address    VARCHAR(255),
    phone      VARCHAR(30),
    manager    VARCHAR(50),
    open_date  DATE,
    status     SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE store IS '门店档案';
COMMENT ON COLUMN store.status IS '1营业 0停业';

-- ============================================================
-- 7. 用户与权限（前置，因后续表引用 users）
-- ============================================================

-- 角色（users 依赖 roles，故先建）
CREATE TABLE roles (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(50) NOT NULL UNIQUE,
    level      SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE roles IS '角色';
COMMENT ON COLUMN roles.name IS '集团管理员/集团财务/运营总监/区域经理/店长/前台/门店财务';
COMMENT ON COLUMN roles.level IS '层级，数值越大权限越高';

-- 权限（菜单树）
CREATE TABLE permissions (
    id         BIGSERIAL PRIMARY KEY,
    code       VARCHAR(100) NOT NULL UNIQUE,
    name       VARCHAR(100) NOT NULL,
    parent_id  BIGINT REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE permissions IS '权限（菜单树）';

-- 角色权限关联
CREATE TABLE role_permission (
    role_id       BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);
COMMENT ON TABLE role_permission IS '角色-权限关联';

-- 用户
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name          VARCHAR(100),
    role_id       BIGINT REFERENCES roles(id),
    status        SMALLINT NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE users IS '用户';
COMMENT ON COLUMN users.status IS '1启用 0禁用';

-- 用户门店关联（数据权限）
CREATE TABLE user_store (
    user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    store_id BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, store_id)
);
COMMENT ON TABLE user_store IS '用户-门店数据权限关联';

-- ============================================================
-- 2. 房型与房间
-- ============================================================

-- 房型（门店级）
CREATE TABLE room_type (
    id         BIGSERIAL PRIMARY KEY,
    store_id   BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL,
    bed_type   VARCHAR(50),
    capacity   SMALLINT NOT NULL DEFAULT 1,
    area       NUMERIC(6,2),
    facilities TEXT,
    status     SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE room_type IS '房型（门店级）';
COMMENT ON COLUMN room_type.status IS '1启用 0禁用';

-- 房间（门店级）
CREATE TABLE room (
    id           BIGSERIAL PRIMARY KEY,
    store_id     BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    room_type_id BIGINT NOT NULL REFERENCES room_type(id),
    room_no      VARCHAR(20) NOT NULL,
    floor        VARCHAR(20),
    orientation  VARCHAR(20),
    status       SMALLINT NOT NULL DEFAULT 0,
    features     TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (store_id, room_no)
);
COMMENT ON TABLE room IS '房间（门店级）';
COMMENT ON COLUMN room.status IS '0空净 1空脏 2住客 3维修 4预留';

-- ============================================================
-- 3. 房价政策
-- ============================================================

-- 房价方案
CREATE TABLE rate_plan (
    id         BIGSERIAL PRIMARY KEY,
    store_id   BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL,
    type       VARCHAR(20) NOT NULL,
    status     SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE rate_plan IS '房价方案';
COMMENT ON COLUMN rate_plan.name IS '门市/协议/会员/促销';
COMMENT ON COLUMN rate_plan.type IS 'rack/contract/member/promo';

-- 房价日历
CREATE TABLE rate_calendar (
    id           BIGSERIAL PRIMARY KEY,
    store_id     BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    room_type_id BIGINT NOT NULL REFERENCES room_type(id) ON DELETE CASCADE,
    rate_plan_id BIGINT NOT NULL REFERENCES rate_plan(id) ON DELETE CASCADE,
    biz_date     DATE NOT NULL,
    price        NUMERIC(10,2) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (store_id, room_type_id, rate_plan_id, biz_date)
);
COMMENT ON TABLE rate_calendar IS '房价日历（按营业日期定价）';

-- 价格调整审计
CREATE TABLE price_change_log (
    id           BIGSERIAL PRIMARY KEY,
    store_id     BIGINT NOT NULL REFERENCES store(id),
    operator_id  BIGINT REFERENCES users(id),
    room_type_id BIGINT NOT NULL REFERENCES room_type(id),
    old_price    NUMERIC(10,2),
    new_price    NUMERIC(10,2),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE price_change_log IS '价格调整审计日志';

-- ============================================================
-- 4. 客户与会员（集团级，不带 store_id）
-- ============================================================

-- 客户（集团级）
CREATE TABLE customer (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(100) NOT NULL,
    gender     SMALLINT,
    id_type    VARCHAR(20),
    id_no      VARCHAR(100),
    phone      VARCHAR(30),
    tags       VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE customer IS '客户档案（集团级，跨店共享）';
COMMENT ON COLUMN customer.id_no IS '证件号（加密存储）';
COMMENT ON COLUMN customer.tags IS '标签：黑名单/贵宾';

-- 会员（集团级）
CREATE TABLE member (
    id          BIGSERIAL PRIMARY KEY,
    customer_id BIGINT NOT NULL REFERENCES customer(id) ON DELETE CASCADE,
    member_no   VARCHAR(50) NOT NULL UNIQUE,
    level       SMALLINT NOT NULL DEFAULT 0,
    points      INT NOT NULL DEFAULT 0,
    balance     NUMERIC(12,2) NOT NULL DEFAULT 0,
    join_date   DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE member IS '会员（集团级，积分/储值跨店累计）';

-- ============================================================
-- 5. 预订
-- ============================================================

-- 预订
CREATE TABLE reservation (
    id            BIGSERIAL PRIMARY KEY,
    store_id      BIGINT NOT NULL REFERENCES store(id),
    customer_id   BIGINT REFERENCES customer(id),
    channel       VARCHAR(20) NOT NULL DEFAULT 'offline',
    status        SMALLINT NOT NULL DEFAULT 0,
    check_in_date DATE NOT NULL,
    check_out_date DATE NOT NULL,
    deposit       NUMERIC(12,2) NOT NULL DEFAULT 0,
    contact       VARCHAR(50),
    remark        TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE reservation IS '预订';
COMMENT ON COLUMN reservation.channel IS 'phone/walk_in/online/ota';
COMMENT ON COLUMN reservation.status IS '0预订 1已入住 2已取消 3已退房 4No-show';

-- 预订明细
CREATE TABLE reservation_item (
    id            BIGSERIAL PRIMARY KEY,
    reservation_id BIGINT NOT NULL REFERENCES reservation(id) ON DELETE CASCADE,
    room_type_id  BIGINT NOT NULL REFERENCES room_type(id),
    room_id       BIGINT REFERENCES room(id),
    rate_plan_id  BIGINT REFERENCES rate_plan(id),
    price         NUMERIC(10,2),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE reservation_item IS '预订明细（房间行）';

-- ============================================================
-- 6. 入住与账单
-- ============================================================

-- 入住
CREATE TABLE check_in (
    id                     BIGSERIAL PRIMARY KEY,
    store_id               BIGINT NOT NULL REFERENCES store(id),
    reservation_id         BIGINT REFERENCES reservation(id),
    customer_id            BIGINT NOT NULL REFERENCES customer(id),
    room_id                BIGINT NOT NULL REFERENCES room(id),
    check_in_time          TIMESTAMPTZ NOT NULL DEFAULT now(),
    expected_checkout_time TIMESTAMPTZ,
    deposit                NUMERIC(12,2) NOT NULL DEFAULT 0,
    status                 SMALLINT NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE check_in IS '入住登记';
COMMENT ON COLUMN check_in.status IS '0在住 1已退房';

-- 账单
CREATE TABLE folio (
    id           BIGSERIAL PRIMARY KEY,
    check_in_id  BIGINT NOT NULL REFERENCES check_in(id) ON DELETE CASCADE,
    total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    paid_amount  NUMERIC(12,2) NOT NULL DEFAULT 0,
    balance      NUMERIC(12,2) NOT NULL DEFAULT 0,
    status       SMALLINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE folio IS '账单';
COMMENT ON COLUMN folio.status IS '0未结清 1已结清';

-- 账单明细
CREATE TABLE folio_item (
    id         BIGSERIAL PRIMARY KEY,
    folio_id   BIGINT NOT NULL REFERENCES folio(id) ON DELETE CASCADE,
    item_type  VARCHAR(20) NOT NULL,
    amount     NUMERIC(12,2) NOT NULL,
    biz_time   TIMESTAMPTZ NOT NULL DEFAULT now(),
    remark     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE folio_item IS '账单明细';
COMMENT ON COLUMN folio_item.item_type IS 'room_fee/goods/other';

-- 支付
CREATE TABLE payment (
    id          BIGSERIAL PRIMARY KEY,
    folio_id    BIGINT NOT NULL REFERENCES folio(id) ON DELETE CASCADE,
    method      VARCHAR(20) NOT NULL,
    amount      NUMERIC(12,2) NOT NULL,
    pay_time    TIMESTAMPTZ NOT NULL DEFAULT now(),
    operator_id BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE payment IS '支付记录';
COMMENT ON COLUMN payment.method IS 'cash/bank_card/wechat/alipay';

-- ============================================================
-- 索引（高频查询）
-- ============================================================

CREATE INDEX idx_store_region ON store(region_id);
CREATE INDEX idx_room_store ON room(store_id);
CREATE INDEX idx_room_type_store ON room_type(store_id);
CREATE INDEX idx_rate_calendar_store_date ON rate_calendar(store_id, biz_date);
CREATE INDEX idx_rate_plan_store ON rate_plan(store_id);
CREATE INDEX idx_reservation_store_date ON reservation(store_id, check_in_date);
CREATE INDEX idx_reservation_customer ON reservation(customer_id);
CREATE INDEX idx_customer_phone ON customer(phone);
CREATE INDEX idx_member_no ON member(member_no);
CREATE INDEX idx_check_in_store ON check_in(store_id);
CREATE INDEX idx_check_in_room ON check_in(room_id);
CREATE INDEX idx_folio_check_in ON folio(check_in_id);
CREATE INDEX idx_folio_item_folio ON folio_item(folio_id);
CREATE INDEX idx_payment_folio ON payment(folio_id);
CREATE INDEX idx_user_store_store ON user_store(store_id);
CREATE INDEX idx_user_username ON users(username);
