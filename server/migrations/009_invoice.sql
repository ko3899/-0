-- ============================================================
-- 009_invoice.sql：发票管理模块
-- 说明：
--   * invoice_title 发票抬头（个人/企业），可关联客户档案，支持多抬头
--   * invoice 发票记录，关联账单(folio)/入住(check_in)/客户/抬头
--   * 发票号系统自动生成（格式：年份+流水号），状态：待开/已开/作废/红冲
--   * 演示环境不对接真实税控，仅做业务流程与台账
-- ============================================================

-- 1. 发票抬头
CREATE TABLE IF NOT EXISTS invoice_title (
    id           BIGSERIAL PRIMARY KEY,
    customer_id  BIGINT REFERENCES customer(id) ON DELETE SET NULL,
    title_type   SMALLINT NOT NULL DEFAULT 0,   -- 0个人 1企业
    title_name   VARCHAR(200) NOT NULL,
    tax_no       VARCHAR(30),                    -- 税号（企业）
    address      VARCHAR(255),
    phone        VARCHAR(30),
    bank_name    VARCHAR(100),
    bank_account VARCHAR(50),
    email        VARCHAR(100),
    is_default   SMALLINT NOT NULL DEFAULT 0,   -- 1默认抬头
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE invoice_title IS '发票抬头（个人/企业）';
COMMENT ON COLUMN invoice_title.title_type IS '0个人 1企业';
COMMENT ON COLUMN invoice_title.is_default IS '1默认抬头';

CREATE INDEX IF NOT EXISTS idx_invoice_title_customer ON invoice_title(customer_id);

-- 2. 发票记录
CREATE TABLE IF NOT EXISTS invoice (
    id            BIGSERIAL PRIMARY KEY,
    store_id      BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    invoice_no    VARCHAR(50) NOT NULL UNIQUE,   -- 发票号码（系统生成）
    folio_id      BIGINT REFERENCES folio(id) ON DELETE SET NULL,
    check_in_id   BIGINT REFERENCES check_in(id) ON DELETE SET NULL,
    customer_id   BIGINT REFERENCES customer(id) ON DELETE SET NULL,
    title_id      BIGINT REFERENCES invoice_title(id) ON DELETE SET NULL,
    invoice_type  SMALLINT NOT NULL DEFAULT 0,   -- 0增值税普通 1增值税专用 2电子普通
    title_name    VARCHAR(200) NOT NULL,
    tax_no        VARCHAR(30),
    amount        NUMERIC(14,2) NOT NULL DEFAULT 0,    -- 含税金额
    tax_amount    NUMERIC(14,2) NOT NULL DEFAULT 0,    -- 税额
    status        SMALLINT NOT NULL DEFAULT 0,   -- 0待开 1已开 2作废 3红冲
    issued_by     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    issued_at     TIMESTAMPTZ,
    voided_at     TIMESTAMPTZ,
    remark        TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE invoice IS '发票记录';
COMMENT ON COLUMN invoice.invoice_type IS '0增值税普通发票 1增值税专用发票 2电子普通发票';
COMMENT ON COLUMN invoice.status IS '0待开 1已开 2作废 3红冲';

CREATE INDEX IF NOT EXISTS idx_invoice_store ON invoice(store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoice_no ON invoice(invoice_no);
CREATE INDEX IF NOT EXISTS idx_invoice_folio ON invoice(folio_id);
CREATE INDEX IF NOT EXISTS idx_invoice_customer ON invoice(customer_id);
CREATE INDEX IF NOT EXISTS idx_invoice_status ON invoice(status, created_at DESC);

-- 发票号序列（格式：FP + 年份 + 8位流水，如 FP202400000001）
CREATE SEQUENCE IF NOT EXISTS invoice_no_seq START 1;
