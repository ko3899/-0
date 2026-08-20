-- ============================================================
-- 006_ota.sql：OTA 渠道对接模块
-- 说明：
--   * 支持多 OTA 渠道（美团、同程、携程、飞猪等）
--   * 渠道配置含 API 密钥、回调地址等
--   * 房型映射：PMS 房型 ↔ OTA 房型（含价格系数）
--   * 同步日志：记录每次推送/拉取的结果
--   * 后续可扩展 ota_order 表存储拉取的订单
-- ============================================================

-- 1. OTA 渠道配置
CREATE TABLE ota_channel (
    id            BIGSERIAL PRIMARY KEY,
    store_id      BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    name          VARCHAR(50) NOT NULL,
    channel_code  VARCHAR(20) NOT NULL,
    api_url       VARCHAR(255),
    app_key       VARCHAR(255),
    app_secret    VARCHAR(255),
    hotel_id      VARCHAR(100),
    callback_url  VARCHAR(255),
    status        SMALLINT NOT NULL DEFAULT 1,
    synced_at     TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (store_id, channel_code)
);
COMMENT ON TABLE ota_channel IS 'OTA 渠道配置';
COMMENT ON COLUMN ota_channel.channel_code IS 'meituan/tongcheng/ctrip/fliggy';
COMMENT ON COLUMN ota_channel.status IS '1启用 0禁用';
COMMENT ON COLUMN ota_channel.synced_at IS '最近一次同步时间';

-- 2. OTA 房型映射（PMS 房型 ↔ OTA 房型）
CREATE TABLE ota_room_mapping (
    id              BIGSERIAL PRIMARY KEY,
    channel_id      BIGINT NOT NULL REFERENCES ota_channel(id) ON DELETE CASCADE,
    room_type_id    BIGINT NOT NULL REFERENCES room_type(id) ON DELETE CASCADE,
    ota_room_type_id VARCHAR(100) NOT NULL,
    ota_room_name   VARCHAR(100),
    price_factor     NUMERIC(5,3) NOT NULL DEFAULT 1.000,
    auto_sync       BOOLEAN NOT NULL DEFAULT true,
    status           SMALLINT NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (channel_id, room_type_id)
);
COMMENT ON TABLE ota_room_mapping IS 'OTA 房型映射';
COMMENT ON COLUMN ota_room_mapping.ota_room_type_id IS 'OTA 平台上的房型 ID';
COMMENT ON COLUMN ota_room_mapping.price_factor IS '价格系数（如 0.9 表示 OTA 价 = PMS 价 × 0.9）';
COMMENT ON COLUMN ota_room_mapping.auto_sync IS '是否自动同步房态';
COMMENT ON COLUMN ota_room_mapping.status IS '1启用 0禁用';

-- 3. OTA 同步日志
CREATE TABLE ota_sync_log (
    id           BIGSERIAL PRIMARY KEY,
    channel_id   BIGINT NOT NULL REFERENCES ota_channel(id) ON DELETE CASCADE,
    sync_type    VARCHAR(20) NOT NULL,
    status       VARCHAR(10) NOT NULL DEFAULT 'success',
    request_body TEXT,
    response_body TEXT,
    error_msg    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE ota_sync_log IS 'OTA 同步日志';
COMMENT ON COLUMN ota_sync_log.sync_type IS 'inventory(房态)/rate(房价)/order(订单)';
COMMENT ON COLUMN ota_sync_log.status IS 'success/fail';

-- 索引
CREATE INDEX idx_ota_channel_store ON ota_channel(store_id);
CREATE INDEX idx_ota_room_mapping_channel ON ota_room_mapping(channel_id);
CREATE INDEX idx_ota_room_mapping_room_type ON ota_room_mapping(room_type_id);
CREATE INDEX idx_ota_sync_log_channel ON ota_sync_log(channel_id);
CREATE INDEX idx_ota_sync_log_time ON ota_sync_log(created_at DESC);