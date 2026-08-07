CREATE TABLE tokens
(
    id         BIGSERIAL PRIMARY KEY                          NOT NULL,
    user_id    BIGINT REFERENCES users (id) ON DELETE CASCADE NOT NULL,
    token_hash BYTEA                                          NOT NULL,
    scope      TEXT                                           NOT NULL CHECK ( scope IN ('verification', 'authentication') ),
    expires_at TIMESTAMP(0) WITH TIME ZONE                    NOT NULL
);

CREATE UNIQUE INDEX tokens_user_verification_unique
    ON tokens (user_id) WHERE scope = 'verification';