-- +goose Up
CREATE TABLE user_roles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES global_users(id) ON DELETE CASCADE,
    role_type VARCHAR(50) NOT NULL,
    role_name VARCHAR(100) NOT NULL,
    UNIQUE (user_id, role_type, role_name)
);

CREATE INDEX idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX idx_user_roles_user_type ON user_roles(user_id, role_type);

-- Migrate existing role column data to user_roles table
INSERT INTO user_roles (user_id, role_type, role_name)
SELECT id, 'SYSTEM', role FROM global_users WHERE role IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS user_roles;
