DROP INDEX IF EXISTS tokens_user_reset_unique;

DELETE FROM tokens WHERE scope = 'reset';

ALTER TABLE tokens
    DROP CONSTRAINT tokens_scope_check;

ALTER TABLE tokens
    ADD CONSTRAINT tokens_scope_check
        CHECK (scope IN ('verification', 'authentication'));
