ALTER TABLE projects
    DROP CONSTRAINT projects_access_check;

ALTER TABLE projects
    ADD CONSTRAINT projects_access_check
        CHECK (access IN ('private', 'public', 'protected'));

ALTER TABLE projects
    ADD COLUMN password_hash BYTEA DEFAULT NULL;

ALTER TABLE projects
    ADD CONSTRAINT projects_protected_needs_password
        CHECK (access <> 'protected' OR password_hash IS NOT NULL);
