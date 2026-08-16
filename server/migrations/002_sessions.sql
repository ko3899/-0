-- ============================================================
-- 酒店管理系统 第二期迁移：会话表（登录鉴权）
-- 说明：登录成功后写入会话，鉴权中间件据此校验令牌并解析用户。
-- ============================================================

CREATE TABLE session (
    token      VARCHAR(64) PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE session IS '登录会话（令牌）';
COMMENT ON COLUMN session.expires_at IS '过期时间，鉴权时校验 now() < expires_at';

CREATE INDEX idx_session_user ON session(user_id);
CREATE INDEX idx_session_expires ON session(expires_at);
