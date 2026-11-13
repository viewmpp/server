CREATE TABLE billing_subscriptions
(
    user_id         BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    subscription_id TEXT                        NOT NULL UNIQUE,
    customer_id     TEXT                        NOT NULL,
    variant_id      TEXT                        NOT NULL,
    status          TEXT                        NOT NULL,
    access_until    TIMESTAMP(0) WITH TIME ZONE NOT NULL,
    period_end      TIMESTAMP(0) WITH TIME ZONE,
    prior_until     TIMESTAMP(0) WITH TIME ZONE,
    event_at        TIMESTAMP(6) WITH TIME ZONE NOT NULL,
    test_mode       BOOLEAN                     NOT NULL,
    created_at      TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT now()
);
