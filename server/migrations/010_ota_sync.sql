-- ============================================================
-- 010_ota_sync.sql：OTA 渠道同步闭环（方案C·模拟API层）
-- 新增三张表：
--   * ota_quota      渠道配额（超卖防护：每渠道每房型分配配额，原子扣减）
--   * ota_order      OTA 订单（回调/拉取进入，自动转 PMS 预订）
--   * ota_push_log   推送明细日志（每次库存/价格推送的逐条记录）
-- 设计要点：
--   * 可推给某渠道的房量 = min(物理空净房, 该渠道配额 - 已用)
--   * OTA 下单 → 原子扣配额 used+1；退房/取消 → used-1
--   * callChannelAPI 为模拟层，后续接真实 API 只改一处
-- ============================================================

-- 1. 渠道配额（超卖防护）
CREATE TABLE IF NOT EXISTS ota_quota (
    id            BIGSERIAL PRIMARY KEY,
    store_id      BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    channel_id    BIGINT NOT NULL REFERENCES ota_channel(id) ON DELETE CASCADE,
    room_type_id  BIGINT NOT NULL REFERENCES room_type(id) ON DELETE CASCADE,
    quota         INT NOT NULL DEFAULT 0,    -- 分配给该渠道的可售配额
    used          INT NOT NULL DEFAULT 0,    -- 已占用（OTA下单+1，退房/取消-1）
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (channel_id, room_type_id)
);
COMMENT ON TABLE ota_quota IS 'OTA 渠道配额（超卖防护）';
COMMENT ON COLUMN ota_quota.quota IS '该渠道该房型可售配额上限';
COMMENT ON COLUMN ota_quota.used IS '已占用配额（OTA下单+1，退房/取消-1）';

CREATE INDEX IF NOT EXISTS idx_ota_quota_store ON ota_quota(store_id);
CREATE INDEX IF NOT EXISTS idx_ota_quota_channel_rt ON ota_quota(channel_id, room_type_id);

-- 2. OTA 订单
CREATE TABLE IF NOT EXISTS ota_order (
    id              BIGSERIAL PRIMARY KEY,
    store_id        BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    channel_id      BIGINT NOT NULL REFERENCES ota_channel(id) ON DELETE CASCADE,
    ota_order_no    VARCHAR(100) NOT NULL,    -- OTA 平台订单号
    customer_name   VARCHAR(100) NOT NULL,
    customer_phone  VARCHAR(30),
    check_in_date   DATE NOT NULL,
    check_out_date  DATE NOT NULL,
    room_type_id    BIGINT NOT NULL REFERENCES room_type(id),
    price           NUMERIC(10,2) NOT NULL DEFAULT 0,
    nights          INT NOT NULL DEFAULT 1,
    status          SMALLINT NOT NULL DEFAULT 0,  -- 0待处理 1已转预订 2已取消 3已入住
    reservation_id  BIGINT REFERENCES reservation(id) ON DELETE SET NULL,
    source          VARCHAR(20) NOT NULL DEFAULT 'callback',  -- callback/pull/manual
    raw_data        TEXT,                       -- 原始报文（回调/拉取的JSON）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (channel_id, ota_order_no)
);
COMMENT ON TABLE ota_order IS 'OTA 订单（回调/拉取进入，自动转 PMS 预订）';
COMMENT ON COLUMN ota_order.status IS '0待处理 1已转预订 2已取消 3已入住';
COMMENT ON COLUMN ota_order.source IS 'callback(平台回调) pull(定时拉取) manual(手动录入)';

CREATE INDEX IF NOT EXISTS idx_ota_order_store ON ota_order(store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ota_order_status ON ota_order(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ota_order_channel ON ota_order(channel_id, created_at DESC);

-- 3. 推送明细日志（比 ota_sync_log 更细：按渠道+房型+动作）
CREATE TABLE IF NOT EXISTS ota_push_log (
    id            BIGSERIAL PRIMARY KEY,
    store_id      BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    channel_id    BIGINT NOT NULL REFERENCES ota_channel(id) ON DELETE CASCADE,
    room_type_id  BIGINT NOT NULL REFERENCES room_type(id) ON DELETE CASCADE,
    push_type     VARCHAR(20) NOT NULL,    -- inventory/rate
    action        VARCHAR(20) NOT NULL,    -- open/close/update
    payload       TEXT,                     -- 推送内容JSON
    status        VARCHAR(10) NOT NULL DEFAULT 'success',  -- success/fail
    error_msg     TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE ota_push_log IS 'OTA 推送明细日志（按渠道+房型+动作）';

CREATE INDEX IF NOT EXISTS idx_ota_push_log_store ON ota_push_log(store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ota_push_log_channel ON ota_push_log(channel_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ota_push_log_type ON ota_push_log(push_type, created_at DESC);
