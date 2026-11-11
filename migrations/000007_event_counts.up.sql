CREATE TABLE event_counts
(
    day   DATE   NOT NULL,
    name  TEXT   NOT NULL,
    total BIGINT NOT NULL,
    PRIMARY KEY (day, name)
);
