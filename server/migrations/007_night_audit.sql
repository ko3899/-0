-- ============================================================
-- 007_night_audit.sql：夜审模块
-- 说明：
--   * night_audit_log 记录每次夜审（按营业日，每日仅一次）
--   * folio_item 增加 biz_date 列，标记房费归属的营业日，
--     用于夜审自动过账的去重（同一天同一账单不重复生成房费）
--   * 夜审核心动作：对在住(check_in.status=0)的账单自动追加当日房费明细
-- ============================================================

-- 1. folio_item 增加营业日列（可空：历史数据与非房费明细为 NULL）
ALTER TABLE folio_item ADD COLUMN IF NOT EXISTS biz_date DATE;
COMMENT ON COLUMN folio_item.biz_date IS '营业日（仅 room_fee 夜审过账时填充，用于去重）';
CREATE INDEX IF NOT EXISTS idx_folio_item_biz_date ON folio_item(folio_id, biz_date);

-- 2. 夜审日志
CREATE TABLE IF NOT EXISTS night_audit_log (
    id            BIGSERIAL PRIMARY KEY,
    biz_date      DATE NOT NULL UNIQUE,
    status        SMALLINT NOT NULL DEFAULT 1,   -- 1已完成 0进行中 2失败
    started_by    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ,
    revenue       NUMERIC(14,2) NOT NULL DEFAULT 0,  -- 当日营收（支付总额）
    checkins      INT NOT NULL DEFAULT 0,            -- 当日入住数
    checkouts     INT NOT NULL DEFAULT 0,            -- 当日退房数
    in_house      INT NOT NULL DEFAULT 0,            -- 夜审时在住数
    posted_count  INT NOT NULL DEFAULT 0,            -- 过账房费笔数
    posted_amount NUMERIC(14,2) NOT NULL DEFAULT 0,  -- 过账房费总额
    exceptions    TEXT,                               -- 异常说明（超期未退房/余额不足等）
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE night_audit_log IS '夜审日志（每营业日一条）';
COMMENT ON COLUMN night_audit_log.biz_date IS '被夜审的营业日（通常为执行日的前一天，即刚结束的营业日）';
COMMENT ON COLUMN night_audit_log.status IS '1已完成 0进行中 2失败';

CREATE INDEX IF NOT EXISTS idx_night_audit_date ON night_audit_log(biz_date DESC);
