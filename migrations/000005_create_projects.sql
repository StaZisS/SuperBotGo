-- +goose Up
CREATE TABLE projects (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT
);

-- +goose Down
DROP TABLE IF EXISTS projects;
