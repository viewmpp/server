ALTER TABLE tokens
    DROP CONSTRAINT tokens_scope_check;

ALTER TABLE tokens
    ADD CONSTRAINT tokens_scope_check
        CHECK (scope IN ('verification', 'authentication', 'reset'));

CREATE UNIQUE INDEX tokens_user_reset_unique
    ON tokens (user_id) WHERE scope = 'reset';
