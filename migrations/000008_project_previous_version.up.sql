ALTER TABLE projects
    ADD COLUMN previous_contract  BYTEA                       DEFAULT NULL,
    ADD COLUMN previous_file_name TEXT                        DEFAULT NULL,
    ADD COLUMN updated_at         TIMESTAMP(0) WITH TIME ZONE DEFAULT NULL;
