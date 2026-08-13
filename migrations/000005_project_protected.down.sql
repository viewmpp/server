UPDATE projects SET access = 'private' WHERE access = 'protected';

ALTER TABLE projects
    DROP CONSTRAINT projects_protected_needs_password;

ALTER TABLE projects
    DROP COLUMN password_hash;

ALTER TABLE projects
    DROP CONSTRAINT projects_access_check;

ALTER TABLE projects
    ADD CONSTRAINT projects_access_check
        CHECK (access IN ('private', 'public'));
