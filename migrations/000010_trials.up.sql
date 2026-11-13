CREATE TABLE trials
(
    user_id     BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    started_at  TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT now(),
    ends_at     TIMESTAMP(0) WITH TIME ZONE NOT NULL,
    reminded_at TIMESTAMP(0) WITH TIME ZONE
);

INSERT INTO trials (user_id, started_at, ends_at, reminded_at)
SELECT id,
       LEAST(now(), COALESCE(subscription_until, now())),
       COALESCE(subscription_until, now()),
       CASE WHEN subscription_until IS NULL OR subscription_until <= now() THEN now() END
FROM users
WHERE subscription = 'pro';
