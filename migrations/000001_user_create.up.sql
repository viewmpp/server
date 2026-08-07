CREATE EXTENSION CITEXT;

CREATE TABLE users
(
    id            BIGSERIAL PRIMARY KEY        NOT NULL,
    email         CITEXT UNIQUE                NOT NULL,
    password_hash BYTEA                        NOT NULL,
    verified      BOOLEAN                      NOT NULL DEFAULT FALSE,
    subscription  TEXT                         NOT NULL DEFAULT 'free',
    created_at    TIMESTAMP(0) WITH TIME ZONE  NOT NULL DEFAULT now(),
    version       BIGINT                       NOT NULL DEFAULT 1
);