CREATE TABLE projects
(
    id         BIGSERIAL PRIMARY KEY,
    public_id  TEXT                        NOT NULL UNIQUE,
    user_id    BIGINT                      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    file_name  TEXT                        NOT NULL,
    contract   BYTEA                       NOT NULL,
    access     TEXT                        NOT NULL DEFAULT 'private'
        CHECK (access IN ('private', 'public')),
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE INDEX projects_user_id_created_at_idx ON projects (user_id, created_at DESC);
