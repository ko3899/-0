-- 003_room_change: 换房记录表
-- 前台增值模块：记录每次换房操作，用于审计追溯

CREATE TABLE room_change (
    id            BIGSERIAL PRIMARY KEY,
    check_in_id   BIGINT NOT NULL REFERENCES check_in(id) ON DELETE CASCADE,
    from_room_id  BIGINT NOT NULL REFERENCES room(id),
    to_room_id    BIGINT NOT NULL REFERENCES room(id),
    change_time   TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason        TEXT,
    operator_id   BIGINT REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE room_change IS '换房记录';
COMMENT ON COLUMN room_change.reason IS '换房原因（如：空调故障、客人要求）';