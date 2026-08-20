-- 004_floor_management.sql
-- 楼层管理：独立楼层表，支持自由增删楼层和房间

-- 楼层表
CREATE TABLE IF NOT EXISTS floor (
    id         BIGSERIAL PRIMARY KEY,
    store_id   BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    name       VARCHAR(20) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (store_id, name)
);
COMMENT ON TABLE floor IS '楼层管理（门店级）';
COMMENT ON COLUMN floor.name IS '楼层名称，如 1楼、2楼、大堂层';
COMMENT ON COLUMN floor.sort_order IS '排序权重，越小越靠前';

-- 为已有房间按 floor 字段自动生成楼层记录（幂等：ON CONFLICT DO NOTHING）
INSERT INTO floor (store_id, name, sort_order)
SELECT DISTINCT r.store_id, r.floor, CAST(r.floor AS INT)
FROM room r
WHERE r.floor IS NOT NULL AND r.floor != ''
ON CONFLICT (store_id, name) DO NOTHING;