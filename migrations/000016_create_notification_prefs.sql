-- +goose Up
CREATE TABLE notification_prefs (
    global_user_id   BIGINT       PRIMARY KEY,
    channel_priority VARCHAR(255) NOT NULL DEFAULT 'TELEGRAM',
    mute_mentions    BOOLEAN      NOT NULL DEFAULT FALSE,
    work_hours_start SMALLINT,
    work_hours_end   SMALLINT,
    timezone         VARCHAR(100) NOT NULL DEFAULT 'UTC',

    CONSTRAINT fk_notification_prefs_user
        FOREIGN KEY (global_user_id) REFERENCES global_users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS notification_prefs;
