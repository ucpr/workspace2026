CREATE TABLE IF NOT EXISTS orders (
    id         BIGSERIAL PRIMARY KEY,
    item       TEXT        NOT NULL,
    quantity   INTEGER     NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'created',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A little seed data so GET/list works right after boot.
INSERT INTO orders (item, quantity, status)
VALUES ('welcome-widget', 1, 'created')
ON CONFLICT DO NOTHING;
