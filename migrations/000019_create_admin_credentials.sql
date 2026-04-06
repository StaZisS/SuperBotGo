-- +goose Up
CREATE TABLE admin_credentials (
    id            BIGSERIAL PRIMARY KEY,
    global_user_id BIGINT NOT NULL UNIQUE REFERENCES global_users(id) ON DELETE CASCADE,
    email         VARCHAR(500) NOT NULL UNIQUE,
    password_hash TEXT         NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_admin_credentials_email ON admin_credentials (email);

-- Seed: create the initial admin user and credentials.
-- Default login: admin@superbot.local / changeme123
-- IMPORTANT: change the password after first login!
INSERT INTO global_users (primary_channel, locale, role)
VALUES ('TELEGRAM', 'ru', 'ADMIN')
ON CONFLICT DO NOTHING;

INSERT INTO admin_credentials (global_user_id, email, password_hash)
VALUES (
    (SELECT id FROM global_users WHERE role = 'ADMIN' ORDER BY id LIMIT 1),
    'admin@superbot.local',
    '$2a$12$REGjH8FH0qaPuDpx/DbX5.LTAq5jJzJLZgSDVNAWiOj0DRbLbYhSe'
);

-- +goose Down
DROP TABLE IF EXISTS admin_credentials;
