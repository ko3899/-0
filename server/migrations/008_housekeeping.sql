-- ============================================================
-- 008_housekeeping.sql：客房清洁管理模块
-- 说明：
--   * housekeeping_task 清洁任务，关联房间与执行人
--   * 退房后房间转空脏(status=1)时，系统自动生成一条待分配清洁任务
--   * 流转：待分配(0)→已分配(1)→清洁中(2)→待查房(3)→已完成(4)/需维修(5)
--   * 查房通过则房间转空净(status=0)；需维修则房间转维修(status=3)
-- ============================================================

CREATE TABLE IF NOT EXISTS housekeeping_task (
    id            BIGSERIAL PRIMARY KEY,
    store_id      BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    room_id       BIGINT NOT NULL REFERENCES room(id) ON DELETE CASCADE,
    check_in_id   BIGINT REFERENCES check_in(id) ON DELETE SET NULL,
    task_type     SMALLINT NOT NULL DEFAULT 0,   -- 0退房清洁 1日常清洁 2深层清洁
    status        SMALLINT NOT NULL DEFAULT 0,   -- 0待分配 1已分配 2清洁中 3待查房 4已完成 5需维修
    priority      SMALLINT NOT NULL DEFAULT 0,   -- 0普通 1紧急
    assigned_to   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    assigned_at   TIMESTAMPTZ,
    started_at    TIMESTAMPTZ,
    submitted_at  TIMESTAMPTZ,   -- 提交查房时间
    completed_at  TIMESTAMPTZ,   -- 查房通过时间
    inspector     BIGINT REFERENCES users(id) ON DELETE SET NULL,  -- 查房人
    remark        TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE housekeeping_task IS '客房清洁任务';
COMMENT ON COLUMN housekeeping_task.task_type IS '0退房清洁 1日常清洁 2深层清洁';
COMMENT ON COLUMN housekeeping_task.status IS '0待分配 1已分配 2清洁中 3待查房 4已完成 5需维修';
COMMENT ON COLUMN housekeeping_task.priority IS '0普通 1紧急';

CREATE INDEX IF NOT EXISTS idx_hk_store_status ON housekeeping_task(store_id, status);
CREATE INDEX IF NOT EXISTS idx_hk_room ON housekeeping_task(room_id);
CREATE INDEX IF NOT EXISTS idx_hk_assigned ON housekeeping_task(assigned_to, status);
CREATE INDEX IF NOT EXISTS idx_hk_created ON housekeeping_task(created_at DESC);
