-- +goose Up
CREATE TABLE IF NOT EXISTS scheduled_notifications (
    id             BIGSERIAL    PRIMARY KEY,
    global_user_id BIGINT       NOT NULL,
    message        JSONB        NOT NULL,
    priority       SMALLINT     NOT NULL,
    send_at        TIMESTAMPTZ  NOT NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    attempts       INTEGER      NOT NULL DEFAULT 0,
    locked_until   TIMESTAMPTZ,
    last_error     TEXT,
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT fk_scheduled_notifications_user
        FOREIGN KEY (global_user_id) REFERENCES global_users(id) ON DELETE CASCADE
);

ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS id BIGSERIAL;
ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS global_user_id BIGINT;
ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS message JSONB;
ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS priority SMALLINT DEFAULT 0;
ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS send_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;
ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS attempts INTEGER DEFAULT 0;
ALTER TABLE scheduled_notifications ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

UPDATE scheduled_notifications
SET send_at = COALESCE(send_at, now()),
    created_at = COALESCE(created_at, now()),
    attempts = COALESCE(attempts, 0),
    updated_at = COALESCE(updated_at, now());

ALTER TABLE scheduled_notifications ALTER COLUMN send_at SET NOT NULL;
ALTER TABLE scheduled_notifications ALTER COLUMN attempts SET NOT NULL;
ALTER TABLE scheduled_notifications ALTER COLUMN updated_at SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_scheduled_notifications_due
    ON scheduled_notifications (send_at, id)
    WHERE locked_until IS NULL;

CREATE INDEX IF NOT EXISTS idx_scheduled_notifications_locked_until
    ON scheduled_notifications (locked_until)
    WHERE locked_until IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS scheduled_notifications;
