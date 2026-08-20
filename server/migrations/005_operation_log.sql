-- 005_operation_log.sql
-- 操作日志：记录关键业务操作，用于审计追溯

-- 先删除旧表重建（如果存在）
DROP TABLE IF EXISTS operation_log CASCADE;

CREATE TABLE operation_log (
    id          BIGSERIAL PRIMARY KEY,
    store_id    BIGINT,  -- NULL 表示系统级操作（登录/登出/用户管理等）
    user_id     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    username    VARCHAR(50) NOT NULL,
    action      VARCHAR(50) NOT NULL,
    target      VARCHAR(100) NOT NULL DEFAULT '',
    detail      TEXT NOT NULL DEFAULT '',
    ip          VARCHAR(50) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_oplog_store    ON operation_log(store_id, created_at DESC);
CREATE INDEX idx_oplog_user     ON operation_log(user_id, created_at DESC);
CREATE INDEX idx_oplog_action   ON operation_log(action, created_at DESC);
CREATE INDEX idx_oplog_created  ON operation_log(created_at DESC);

COMMENT ON TABLE operation_log IS '操作日志（审计追溯）';
COMMENT ON COLUMN operation_log.action IS '操作类型：login/logout/checkin/checkout/change_room/extend/charge/payment/reservation_create/update/cancel/room_status/user_edit 等';
COMMENT ON COLUMN operation_log.target IS '操作目标，如房间号、订单号、用户名等';
COMMENT ON COLUMN operation_log.detail IS '操作详情，JSON 格式存储关键字段变更';