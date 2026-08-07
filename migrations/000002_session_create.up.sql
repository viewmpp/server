CREATE TABLE sessions
(
    token_hash BYTEA PRIMARY KEY,
    user_id    BIGINT                      REFERENCES users (id) ON DELETE CASCADE,
    data       JSONB                       NOT NULL DEFAULT '{}',
    expires_at TIMESTAMP(0) WITH TIME ZONE NOT NULL
);

CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);
